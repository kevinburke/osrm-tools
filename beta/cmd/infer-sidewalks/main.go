// Command infer-sidewalks reads an OSM PBF file and geometrically matches
// separately-mapped sidewalk ways to their parent roads, outputting GeoJSON
// for MapRoulette review and a JSON file for OSRM Lua profile enrichment.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/kevinburke/osrm-tools/beta/sidewalk"
	"github.com/kevinburke/osrm-tools/geojson"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
)

const version = "0.1.0"

// Highway types we consider as candidate roads for sidewalk inference.
var roadHighways = map[string]bool{
	"residential":    true,
	"tertiary":       true,
	"secondary":      true,
	"primary":        true,
	"living_street":  true,
	"unclassified":   true,
	"tertiary_link":  true,
	"secondary_link": true,
	"primary_link":   true,
}

func main() {
	pbfPath := flag.String("pbf", "", "path to OSM PBF file (required)")
	boundaryPath := flag.String("boundary", "", "GeoJSON file with a Polygon boundary to filter ways (only ways with a node inside are kept)")
	outputPath := flag.String("output", "", "GeoJSON output path (default: stdout)")
	osrmOutputPath := flag.String("osrm-output", "", "OSRM JSON output path (way IDs + inferred sidewalk tags)")
	htmlOutputPath := flag.String("html", "", "HTML output path (Leaflet map with roads, sidewalks, and OSM links)")
	maxDist := flag.Float64("max-distance", 20, "max road-to-sidewalk distance in meters")
	minCov := flag.Float64("min-coverage", 0.5, "minimum fraction of road length covered to declare a sidewalk")
	review := flag.Bool("review", false, "start a local HTTP server for interactive review")
	reviewPort := flag.Int("review-port", 3612, "port for review server")
	confirmedPath := flag.String("confirmed", "confirmed.osc", "osmChange XML output path for confirmed changes")
	rejectedPath := flag.String("rejected", "rejected.json", "rejected candidates JSON output path")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("infer-sidewalks", version)
		os.Exit(0)
	}

	if *pbfPath == "" {
		fmt.Fprintln(os.Stderr, "error: --pbf flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *review && (*outputPath != "" || *htmlOutputPath != "" || *osrmOutputPath != "") {
		fmt.Fprintln(os.Stderr, "error: --review cannot be combined with --output, --html, or --osrm-output")
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	var boundary [][2]float64
	if *boundaryPath != "" {
		var err error
		boundary, err = loadBoundaryPolygon(*boundaryPath)
		if err != nil {
			logger.Error("loading boundary", "error", err)
			os.Exit(1)
		}
		logger.Info("loaded boundary polygon", "vertices", len(boundary), "path", *boundaryPath)
	}

	roads, sidewalks, buildings, meta, err := scanPBF(logger, *pbfPath, boundary)
	if err != nil {
		logger.Error("PBF scan failed", "error", err)
		os.Exit(1)
	}

	cfg := sidewalk.Config{
		MaxDistance:       *maxDist,
		MinCoverage:       *minCov,
		ParallelThreshold: 20,
		CheckBuildings:    true,
	}

	candidates := sidewalk.MatchRoads(logger, roads, sidewalks, buildings, cfg)

	if *review {
		srv := &reviewServer{
			logger:        logger,
			candidates:    candidates,
			sidewalks:     sidewalks,
			meta:          meta,
			confirmedPath: *confirmedPath,
			rejectedPath:  *rejectedPath,
		}
		if err := srv.listenAndServe(*reviewPort); err != nil {
			logger.Error("review server failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := writeGeoJSON(candidates, *outputPath); err != nil {
		logger.Error("writing GeoJSON", "error", err)
		os.Exit(1)
	}

	if *osrmOutputPath != "" {
		if err := writeOSRMJSON(candidates, *osrmOutputPath); err != nil {
			logger.Error("writing OSRM JSON", "error", err)
			os.Exit(1)
		}
	}

	if *htmlOutputPath != "" {
		if err := writeHTML(candidates, sidewalks, *htmlOutputPath); err != nil {
			logger.Error("writing HTML", "error", err)
			os.Exit(1)
		}
	}

	logger.Info("done",
		"candidates", len(candidates),
		"geojson_output", outputDesc(*outputPath),
		"osrm_output", outputDesc(*osrmOutputPath),
		"html_output", outputDesc(*htmlOutputPath),
	)
}

func outputDesc(path string) string {
	if path == "" {
		return "stdout"
	}
	return path
}

// loadBoundaryPolygon reads a GeoJSON file containing a Feature or
// FeatureCollection with a Polygon geometry and returns the outer ring
// as [][2]float64 (lon, lat pairs).
func loadBoundaryPolygon(path string) ([][2]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading boundary file: %w", err)
	}

	// Parse enough structure to extract coordinates from a Polygon geometry.
	// Supports both a bare Feature and a FeatureCollection (uses the first feature).
	var raw struct {
		Type     string `json:"type"`
		Geometry *struct {
			Type        string        `json:"type"`
			Coordinates [][][]float64 `json:"coordinates"`
		} `json:"geometry"`
		Features []struct {
			Geometry struct {
				Type        string        `json:"type"`
				Coordinates [][][]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing boundary GeoJSON: %w", err)
	}

	var ring [][]float64
	switch {
	case raw.Type == "Feature" && raw.Geometry != nil && raw.Geometry.Type == "Polygon":
		if len(raw.Geometry.Coordinates) == 0 {
			return nil, fmt.Errorf("boundary polygon has no rings")
		}
		ring = raw.Geometry.Coordinates[0]
	case raw.Type == "FeatureCollection" && len(raw.Features) > 0:
		geom := raw.Features[0].Geometry
		if geom.Type != "Polygon" || len(geom.Coordinates) == 0 {
			return nil, fmt.Errorf("first feature in collection is not a Polygon")
		}
		ring = geom.Coordinates[0]
	default:
		return nil, fmt.Errorf("unsupported GeoJSON type %q (expected Feature or FeatureCollection with Polygon)", raw.Type)
	}

	if len(ring) < 3 {
		return nil, fmt.Errorf("boundary polygon has fewer than 3 vertices")
	}

	coords := make([][2]float64, len(ring))
	for i, pt := range ring {
		if len(pt) < 2 {
			return nil, fmt.Errorf("coordinate at index %d has fewer than 2 values", i)
		}
		coords[i] = [2]float64{pt[0], pt[1]}
	}
	return coords, nil
}

// coordInBoundary returns true if the [lon, lat] point is inside the boundary
// polygon using ray casting. If boundary is nil, all points are considered inside.
func coordInBoundary(coord [2]float64, boundary [][2]float64) bool {
	if boundary == nil {
		return true
	}
	n := len(boundary)
	inside := false
	j := n - 1
	for i := range n {
		xi, yi := boundary[i][0], boundary[i][1]
		xj, yj := boundary[j][0], boundary[j][1]
		if ((yi > coord[1]) != (yj > coord[1])) &&
			(coord[0] < (xj-xi)*(coord[1]-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}

// wayInBoundary returns true if at least one of the way's resolved coordinates
// lies inside the boundary polygon.
func wayInBoundary(coords [][2]float64, boundary [][2]float64) bool {
	if boundary == nil {
		return true
	}
	for _, c := range coords {
		if coordInBoundary(c, boundary) {
			return true
		}
	}
	return false
}

// scanPBF performs a two-pass scan of the PBF file:
// Pass 1: collect ways (roads, sidewalks, buildings) and their node ID lists.
// Pass 2: resolve node coordinates.
// If boundary is non-nil, only ways with at least one node inside the polygon are kept.
// wayMeta holds the way version and node ID list needed for osmChange XML
// generation. This information is not part of the sidewalk matching pipeline
// but must be preserved for the review/export workflow.
type wayMeta struct {
	Version int
	NodeIDs []osm.NodeID
}

func scanPBF(logger *slog.Logger, path string, boundary [][2]float64) ([]sidewalk.Road, []sidewalk.Sidewalk, []sidewalk.Building, map[int64]wayMeta, error) {
	// --- Pass 1: collect ways ---
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("opening PBF: %w", err)
	}

	logger.Info("pass 1: scanning ways", "path", path)

	scanner := osmpbf.New(context.Background(), f, runtime.NumCPU())
	scanner.SkipNodes = true
	scanner.SkipRelations = true

	type rawWay struct {
		id      int64
		version int
		tags    map[string]string
		nodeIDs []osm.NodeID
	}

	var rawRoads, rawSidewalks, rawBuildings []rawWay
	neededNodes := make(map[osm.NodeID]struct{})

	for scanner.Scan() {
		obj := scanner.Object()
		w, ok := obj.(*osm.Way)
		if !ok {
			continue
		}

		tags := w.Tags.Map()
		hw := tags["highway"]
		nodeIDs := make([]osm.NodeID, len(w.Nodes))
		for i, n := range w.Nodes {
			nodeIDs[i] = n.ID
		}

		rw := rawWay{id: int64(w.ID), version: w.Version, tags: tags, nodeIDs: nodeIDs}

		switch {
		case roadHighways[hw] && tags["sidewalk"] == "":
			rawRoads = append(rawRoads, rw)
			for _, nid := range nodeIDs {
				neededNodes[nid] = struct{}{}
			}
		case (hw == "footway" && tags["footway"] == "sidewalk") ||
			(hw == "path" && tags["path"] == "sidewalk"):
			rawSidewalks = append(rawSidewalks, rw)
			for _, nid := range nodeIDs {
				neededNodes[nid] = struct{}{}
			}
		case tags["building"] != "":
			rawBuildings = append(rawBuildings, rw)
			for _, nid := range nodeIDs {
				neededNodes[nid] = struct{}{}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		f.Close()
		return nil, nil, nil, nil, fmt.Errorf("pass 1 scan error: %w", err)
	}
	scanner.Close()
	f.Close()

	logger.Info("pass 1 complete",
		"roads", len(rawRoads),
		"sidewalks", len(rawSidewalks),
		"buildings", len(rawBuildings),
		"needed_nodes", len(neededNodes),
	)

	// --- Pass 2: resolve node coordinates ---
	f, err = os.Open(path)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("reopening PBF: %w", err)
	}
	defer f.Close()

	logger.Info("pass 2: scanning nodes")

	scanner = osmpbf.New(context.Background(), f, runtime.NumCPU())
	scanner.SkipWays = true
	scanner.SkipRelations = true

	nodeCoords := make(map[osm.NodeID][2]float64, len(neededNodes))
	for scanner.Scan() {
		obj := scanner.Object()
		n, ok := obj.(*osm.Node)
		if !ok {
			continue
		}
		if _, needed := neededNodes[n.ID]; needed {
			nodeCoords[n.ID] = [2]float64{n.Lon, n.Lat}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("pass 2 scan error: %w", err)
	}
	scanner.Close()

	logger.Info("pass 2 complete", "resolved_nodes", len(nodeCoords))

	// --- Resolve coordinates ---
	resolveCoords := func(nodeIDs []osm.NodeID) [][2]float64 {
		coords := make([][2]float64, 0, len(nodeIDs))
		for _, nid := range nodeIDs {
			if c, ok := nodeCoords[nid]; ok {
				coords = append(coords, c)
			}
		}
		return coords
	}

	roads := make([]sidewalk.Road, 0, len(rawRoads))
	meta := make(map[int64]wayMeta, len(rawRoads))
	for _, rw := range rawRoads {
		coords := resolveCoords(rw.nodeIDs)
		if len(coords) < 2 {
			continue
		}
		if !wayInBoundary(coords, boundary) {
			continue
		}
		roads = append(roads, sidewalk.Road{
			ID:     rw.id,
			Tags:   rw.tags,
			Coords: coords,
		})
		meta[rw.id] = wayMeta{
			Version: rw.version,
			NodeIDs: rw.nodeIDs,
		}
	}

	sidewalks := make([]sidewalk.Sidewalk, 0, len(rawSidewalks))
	for _, rw := range rawSidewalks {
		coords := resolveCoords(rw.nodeIDs)
		if len(coords) < 2 {
			continue
		}
		if !wayInBoundary(coords, boundary) {
			continue
		}
		sidewalks = append(sidewalks, sidewalk.Sidewalk{
			ID:     rw.id,
			Coords: coords,
		})
	}

	buildings := make([]sidewalk.Building, 0, len(rawBuildings))
	for _, rw := range rawBuildings {
		coords := resolveCoords(rw.nodeIDs)
		if len(coords) < 3 {
			continue
		}
		if !wayInBoundary(coords, boundary) {
			continue
		}
		buildings = append(buildings, sidewalk.Building{
			ID:     rw.id,
			Coords: coords,
		})
	}

	if boundary != nil {
		logger.Info("boundary filtering complete",
			"roads", len(roads),
			"sidewalks", len(sidewalks),
			"buildings", len(buildings),
		)
	}

	return roads, sidewalks, buildings, meta, nil
}

// strokeColor returns a simplestyle-spec stroke color based on the inferred tag.
func strokeColor(tag string) string {
	switch tag {
	case "both":
		return "#2ecc71" // green
	case "left":
		return "#3498db" // blue
	case "right":
		return "#e67e22" // orange
	default:
		return "#95a5a6" // gray
	}
}

func writeGeoJSON(candidates []sidewalk.Candidate, path string) error {
	fc := geojson.NewFeatureCollection()

	for _, c := range candidates {
		coords := make([][]float64, len(c.Road.Coords))
		for i, pt := range c.Road.Coords {
			coords[i] = []float64{pt[0], pt[1]}
		}

		props := map[string]any{
			"osm_way_id":        c.Road.ID,
			"highway":           c.Road.Tags["highway"],
			"name":              c.Road.Tags["name"],
			"inferred_sidewalk": c.InferredTag,
			"left_coverage":     fmt.Sprintf("%.2f", c.LeftCoverage),
			"right_coverage":    fmt.Sprintf("%.2f", c.RightCoverage),
			"stroke":            strokeColor(c.InferredTag),
			"stroke-width":      3,
		}

		fc.Add(geojson.NewLineStringFeature(coords, props))
	}

	data, err := fc.Marshal()
	if err != nil {
		return fmt.Errorf("marshaling GeoJSON: %w", err)
	}

	if path == "" {
		_, err = os.Stdout.Write(data)
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

type osrmEntry struct {
	WayID    int64  `json:"way_id"`
	Sidewalk string `json:"sidewalk"`
}

func writeHTML(candidates []sidewalk.Candidate, sidewalks []sidewalk.Sidewalk, path string) error {
	// Build a map of sidewalk ID → coords for looking up matched sidewalks.
	swCoords := make(map[int64][][2]float64, len(sidewalks))
	for _, sw := range sidewalks {
		swCoords[sw.ID] = sw.Coords
	}

	fc := geojson.NewFeatureCollection()

	// Split each whole-road candidate into sub-portions at sidewalk boundaries.
	// 30m minimum run length to avoid noisy micro-segments.
	const minRunLength = 30.0

	for _, c := range candidates {
		splits := sidewalk.SplitCandidate(&c, minRunLength)

		for _, sc := range splits {
			coords := make([][]float64, len(sc.Road.Coords))
			for i, pt := range sc.Road.Coords {
				coords[i] = []float64{pt[0], pt[1]}
			}

			name := sc.Road.Tags["name"]
			if name == "" {
				name = "(unnamed)"
			}

			fc.Add(geojson.NewLineStringFeature(coords, map[string]any{
				"kind":              "road",
				"osm_way_id":        sc.Road.ID,
				"osm_url":           fmt.Sprintf("https://www.openstreetmap.org/way/%d", sc.Road.ID),
				"highway":           sc.Road.Tags["highway"],
				"name":              name,
				"inferred_sidewalk": sc.InferredTag,
				"left_coverage":     fmt.Sprintf("%.0f%%", sc.LeftCoverage*100),
				"right_coverage":    fmt.Sprintf("%.0f%%", sc.RightCoverage*100),
				"stroke":            strokeColor(sc.InferredTag),
				"stroke-width":      4,
				"stroke-opacity":    0.9,
			}))

			// Matched sidewalk features (deduplicated per split).
			seen := make(map[int64]bool)
			for _, m := range sc.Matches {
				if seen[m.SidewalkID] {
					continue
				}
				seen[m.SidewalkID] = true

				swc, ok := swCoords[m.SidewalkID]
				if !ok {
					continue
				}
				sCoords := make([][]float64, len(swc))
				for i, pt := range swc {
					sCoords[i] = []float64{pt[0], pt[1]}
				}
				fc.Add(geojson.NewLineStringFeature(sCoords, map[string]any{
					"kind":           "sidewalk",
					"osm_way_id":     m.SidewalkID,
					"osm_url":        fmt.Sprintf("https://www.openstreetmap.org/way/%d", m.SidewalkID),
					"matched_road":   name,
					"matched_side":   m.Side,
					"stroke":         "#9b59b6", // purple for sidewalks
					"stroke-width":   2,
					"stroke-opacity": 0.7,
				}))
			}
		}
	}

	data, err := fc.Marshal()
	if err != nil {
		return fmt.Errorf("marshaling GeoJSON for HTML: %w", err)
	}

	html, err := geojson.GenerateLeafletHTMLWithHeader(string(data), geojson.LeafletHTMLConfig{
		Title:                "Inferred Sidewalks",
		Version:              version,
		DisableFeaturePopups: true,
		CustomJS: `
      layer.eachLayer(function(l) {
        if (!l.feature) return;
        var p = l.feature.properties || {};
        var lines = [];
        if (p.kind === 'road') {
          lines.push('<b>' + escapeHtml(p.name) + '</b>');
          lines.push('Highway: ' + escapeHtml(p.highway));
          lines.push('Proposed: <b>sidewalk=' + escapeHtml(p.inferred_sidewalk) + '</b>');
          lines.push('Left coverage: ' + p.left_coverage + ' / Right: ' + p.right_coverage);
          lines.push('<a href="' + escapeHtml(p.osm_url) + '" target="_blank">View road on OSM</a>');
        } else {
          lines.push('<b>Sidewalk</b> (matched to ' + escapeHtml(p.matched_road) + ', ' + p.matched_side + ' side)');
          lines.push('<a href="' + escapeHtml(p.osm_url) + '" target="_blank">View sidewalk on OSM</a>');
        }
        l.bindPopup(lines.join('<br>'));
      });

      // Add legend
      var legend = L.control({position: 'bottomright'});
      legend.onAdd = function() {
        var div = L.DomUtil.create('div', 'legend');
        div.style.cssText = 'background:#fff;padding:8px 12px;border-radius:4px;box-shadow:0 1px 4px rgba(0,0,0,.3);font:13px/1.6 system-ui,sans-serif;';
        div.innerHTML = '<b>Proposed sidewalk tag</b><br>' +
          '<span style="color:#2ecc71">\u2501</span> both<br>' +
          '<span style="color:#3498db">\u2501</span> left<br>' +
          '<span style="color:#e67e22">\u2501</span> right<br>' +
          '<span style="color:#9b59b6">\u2501</span> matched sidewalk way';
        return div;
      };
      legend.addTo(map);
`,
	})
	if err != nil {
		return fmt.Errorf("generating HTML: %w", err)
	}

	return os.WriteFile(path, []byte(html), 0o644)
}

func writeOSRMJSON(candidates []sidewalk.Candidate, path string) error {
	entries := make([]osrmEntry, len(candidates))
	for i, c := range candidates {
		entries[i] = osrmEntry{
			WayID:    c.Road.ID,
			Sidewalk: c.InferredTag,
		}
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling OSRM JSON: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}
