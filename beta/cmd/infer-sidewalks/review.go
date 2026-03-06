package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sort"

	"github.com/kevinburke/handlers"
	"github.com/kevinburke/osrm-tools/beta/sidewalk"
	"github.com/kevinburke/osrm-tools/geojson"
	"github.com/paulmach/osm"
)

type reviewServer struct {
	logger        *slog.Logger
	candidates    []sidewalk.Candidate
	sidewalks     []sidewalk.Sidewalk
	meta          map[int64]wayMeta
	confirmedPath string
	rejectedPath  string
}

func (s *reviewServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		s.handleIndex(w, r)
	})
	return handlers.Log(mux)
}

func (s *reviewServer) listenAndServe(port int) error {
	addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.logger.Info("review server listening", "url", "http://"+addr)
	return http.Serve(ln, s.handler())
}

// reviewCandidate is the per-road data sent to the browser.
type reviewCandidate struct {
	RoadID        int64            `json:"road_id"`
	Name          string           `json:"name"`
	Highway       string           `json:"highway"`
	ProposedTag   string           `json:"proposed_tag"`
	LeftCoverage  float64          `json:"left_coverage"`
	RightCoverage float64          `json:"right_coverage"`
	OSMURL        string           `json:"osm_url"`
	RoadCoords    [][]float64      `json:"road_coords"`
	Splits        []reviewSplit    `json:"splits"`
	Sidewalks     []reviewSidewalk `json:"sidewalks"`
}

type reviewSplit struct {
	Coords      [][]float64 `json:"coords"`
	InferredTag string      `json:"inferred_tag"`
}

type reviewSidewalk struct {
	ID     int64       `json:"id"`
	Side   string      `json:"side"`
	Coords [][]float64 `json:"coords"`
}

// buildReviewData deduplicates candidates by road ID and prepares UI data.
func (s *reviewServer) buildReviewData() []reviewCandidate {
	// Build sidewalk coord lookup.
	swCoords := make(map[int64][][2]float64, len(s.sidewalks))
	for _, sw := range s.sidewalks {
		swCoords[sw.ID] = sw.Coords
	}

	// Deduplicate candidates by road ID — multiple split candidates share
	// the same road, but confirm/reject applies to the whole way.
	seen := make(map[int64]bool)
	var result []reviewCandidate

	for i := range s.candidates {
		c := &s.candidates[i]
		if seen[c.Road.ID] {
			continue
		}
		seen[c.Road.ID] = true

		name := c.Road.Tags["name"]
		if name == "" {
			name = "(unnamed)"
		}

		rc := reviewCandidate{
			RoadID:        c.Road.ID,
			Name:          name,
			Highway:       c.Road.Tags["highway"],
			ProposedTag:   c.InferredTag,
			LeftCoverage:  c.LeftCoverage,
			RightCoverage: c.RightCoverage,
			OSMURL:        fmt.Sprintf("https://www.openstreetmap.org/way/%d", c.Road.ID),
			RoadCoords:    coordsToSlice(c.Road.Coords),
		}

		// Add split sub-portions for map visualization.
		const minRunLength = 30.0
		splits := sidewalk.SplitCandidate(c, minRunLength)
		for _, sc := range splits {
			rc.Splits = append(rc.Splits, reviewSplit{
				Coords:      coordsToSlice(sc.Road.Coords),
				InferredTag: sc.InferredTag,
			})
		}

		// Add matched sidewalks.
		swSeen := make(map[int64]bool)
		for _, m := range c.Matches {
			if swSeen[m.SidewalkID] {
				continue
			}
			swSeen[m.SidewalkID] = true

			swc, ok := swCoords[m.SidewalkID]
			if !ok {
				continue
			}
			rc.Sidewalks = append(rc.Sidewalks, reviewSidewalk{
				ID:     m.SidewalkID,
				Side:   m.Side,
				Coords: coordsToSlice(swc),
			})
		}

		result = append(result, rc)
	}

	return result
}

func coordsToSlice(coords [][2]float64) [][]float64 {
	out := make([][]float64, len(coords))
	for i, c := range coords {
		out[i] = []float64{c[0], c[1]}
	}
	return out
}

func (s *reviewServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	data := s.buildReviewData()
	jsonData, err := json.Marshal(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshaling data: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, reviewHTML, geojson.LeafletVersion, geojson.LeafletVersion, string(jsonData))
}

type exportRequest struct {
	Confirmed []int64 `json:"confirmed"`
	Rejected  []int64 `json:"rejected"`
}

type exportResponse struct {
	ConfirmedCount int    `json:"confirmed_count"`
	RejectedCount  int    `json:"rejected_count"`
	ConfirmedPath  string `json:"confirmed_path"`
	RejectedPath   string `json:"rejected_path"`
}

func (s *reviewServer) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Build lookups.
	candByRoad := make(map[int64]*sidewalk.Candidate, len(s.candidates))
	for i := range s.candidates {
		c := &s.candidates[i]
		if _, exists := candByRoad[c.Road.ID]; !exists {
			candByRoad[c.Road.ID] = c
		}
	}

	// Generate osmChange XML for confirmed roads.
	if err := s.writeOSMChange(req.Confirmed, candByRoad); err != nil {
		http.Error(w, fmt.Sprintf("writing osmChange: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate rejected JSON.
	if err := s.writeRejectedJSON(req.Rejected, candByRoad); err != nil {
		http.Error(w, fmt.Sprintf("writing rejected JSON: %v", err), http.StatusInternalServerError)
		return
	}

	s.logger.Info("export complete",
		"confirmed", len(req.Confirmed),
		"rejected", len(req.Rejected),
		"confirmed_path", s.confirmedPath,
		"rejected_path", s.rejectedPath,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(exportResponse{
		ConfirmedCount: len(req.Confirmed),
		RejectedCount:  len(req.Rejected),
		ConfirmedPath:  s.confirmedPath,
		RejectedPath:   s.rejectedPath,
	})
}

func (s *reviewServer) writeOSMChange(confirmedIDs []int64, candByRoad map[int64]*sidewalk.Candidate) error {
	change := &osm.Change{
		Version:   "0.6",
		Generator: "infer-sidewalks " + version,
	}

	for _, roadID := range confirmedIDs {
		c, ok := candByRoad[roadID]
		if !ok {
			continue
		}
		m, ok := s.meta[roadID]
		if !ok {
			continue
		}

		// Build node refs (only ID needed for osmChange modify).
		nodes := make(osm.WayNodes, len(m.NodeIDs))
		for i, nid := range m.NodeIDs {
			nodes[i] = osm.WayNode{ID: nid}
		}

		// Build tags: original tags + sidewalk:*=separate tags.
		// Per https://wiki.openstreetmap.org/wiki/Key:sidewalk — since
		// sidewalks are mapped as separate ways, we tag the road with
		// sidewalk:<side>=separate rather than sidewalk=left/right/both.
		tags := make(osm.Tags, 0, len(c.Road.Tags)+2)
		for k, v := range c.Road.Tags {
			tags = append(tags, osm.Tag{Key: k, Value: v})
		}
		switch c.InferredTag {
		case "both":
			tags = append(tags, osm.Tag{Key: "sidewalk:both", Value: "separate"})
		case "left":
			tags = append(tags, osm.Tag{Key: "sidewalk:left", Value: "separate"})
		case "right":
			tags = append(tags, osm.Tag{Key: "sidewalk:right", Value: "separate"})
		}
		tags.SortByKeyValue()

		way := &osm.Way{
			ID:      osm.WayID(roadID),
			Version: m.Version,
			Visible: true,
			Nodes:   nodes,
			Tags:    tags,
		}
		change.AppendModify(way)
	}

	data, err := xml.MarshalIndent(change, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling osmChange XML: %w", err)
	}

	// Prepend XML declaration.
	out := []byte(xml.Header)
	out = append(out, data...)
	out = append(out, '\n')

	return os.WriteFile(s.confirmedPath, out, 0o644)
}

type rejectedEntry struct {
	WayID         int64   `json:"way_id"`
	Name          string  `json:"name"`
	Highway       string  `json:"highway"`
	ProposedTag   string  `json:"proposed_tag"`
	LeftCoverage  float64 `json:"left_coverage"`
	RightCoverage float64 `json:"right_coverage"`
	OSMURL        string  `json:"osm_url"`
}

func (s *reviewServer) writeRejectedJSON(rejectedIDs []int64, candByRoad map[int64]*sidewalk.Candidate) error {
	entries := make([]rejectedEntry, 0, len(rejectedIDs))
	for _, roadID := range rejectedIDs {
		c, ok := candByRoad[roadID]
		if !ok {
			continue
		}
		name := c.Road.Tags["name"]
		if name == "" {
			name = "(unnamed)"
		}
		entries = append(entries, rejectedEntry{
			WayID:         roadID,
			Name:          name,
			Highway:       c.Road.Tags["highway"],
			ProposedTag:   c.InferredTag,
			LeftCoverage:  c.LeftCoverage,
			RightCoverage: c.RightCoverage,
			OSMURL:        fmt.Sprintf("https://www.openstreetmap.org/way/%d", roadID),
		})
	}

	// Sort by way ID for deterministic output.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].WayID < entries[j].WayID
	})

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling rejected JSON: %w", err)
	}
	data = append(data, '\n')

	return os.WriteFile(s.rejectedPath, data, 0o644)
}

const reviewHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Review Inferred Sidewalks</title>
<link rel="stylesheet" href="https://unpkg.com/leaflet@%s/dist/leaflet.css" />
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: system-ui, -apple-system, sans-serif; display: flex; height: 100vh; overflow: hidden; }
  #map { flex: 0 0 65%%; height: 100%%; }
  #panel { flex: 0 0 35%%; height: 100%%; display: flex; flex-direction: column; border-left: 2px solid #ddd; }
  #header { padding: 12px 16px; background: #f8f9fa; border-bottom: 1px solid #ddd; }
  #header h1 { font-size: 16px; margin-bottom: 4px; }
  #progress { font-size: 13px; color: #666; }
  #progress .confirmed { color: #27ae60; font-weight: 600; }
  #progress .rejected { color: #c0392b; font-weight: 600; }
  #progress .remaining { color: #7f8c8d; }
  #candidate-info { flex: 1; overflow-y: auto; padding: 16px; }
  .info-row { margin-bottom: 8px; font-size: 14px; }
  .info-row label { font-weight: 600; color: #555; display: inline-block; min-width: 90px; }
  .proposed-tag { font-weight: 700; padding: 2px 8px; border-radius: 3px; color: #fff; }
  .proposed-tag.both { background: #27ae60; }
  .proposed-tag.left { background: #2980b9; }
  .proposed-tag.right { background: #d35400; }
  .coverage-bar { display: inline-block; width: 80px; height: 10px; background: #ecf0f1; border-radius: 5px; vertical-align: middle; margin-left: 4px; }
  .coverage-fill { height: 100%%; border-radius: 5px; }
  .coverage-fill.left { background: #2980b9; }
  .coverage-fill.right { background: #d35400; }
  .status-badge { display: inline-block; padding: 2px 8px; border-radius: 3px; font-size: 12px; font-weight: 600; margin-left: 8px; }
  .status-badge.confirmed { background: #d5f5e3; color: #1e8449; }
  .status-badge.rejected { background: #fadbd8; color: #922b21; }
  #buttons { padding: 12px 16px; border-top: 1px solid #ddd; display: flex; gap: 8px; }
  #buttons button { flex: 1; padding: 10px; border: 2px solid; border-radius: 6px; font-size: 14px; font-weight: 600; cursor: pointer; }
  #btn-confirm { background: #d5f5e3; border-color: #27ae60; color: #1e8449; }
  #btn-confirm:hover { background: #a9dfbf; }
  #btn-reject { background: #fadbd8; border-color: #c0392b; color: #922b21; }
  #btn-reject:hover { background: #f5b7b1; }
  #btn-skip { background: #fff; border-color: #bdc3c7; color: #7f8c8d; }
  #btn-skip:hover { background: #ecf0f1; }
  #nav { padding: 8px 16px; border-top: 1px solid #ddd; display: flex; justify-content: space-between; align-items: center; }
  #nav button { padding: 6px 16px; border: 1px solid #bdc3c7; border-radius: 4px; background: #fff; cursor: pointer; font-size: 13px; }
  #nav button:hover { background: #ecf0f1; }
  #nav button:disabled { opacity: 0.4; cursor: default; }
  #nav span { font-size: 13px; color: #666; }
  #export-bar { padding: 12px 16px; border-top: 2px solid #ddd; background: #f8f9fa; }
  #btn-export { width: 100%%; padding: 12px; border: none; border-radius: 6px; background: #2c3e50; color: #fff; font-size: 15px; font-weight: 600; cursor: pointer; }
  #btn-export:hover { background: #34495e; }
  #btn-export:disabled { opacity: 0.5; cursor: default; }
  #export-result { margin-top: 8px; font-size: 13px; color: #27ae60; display: none; }
  .keyboard-hint { font-size: 11px; color: #999; text-align: center; padding: 4px; }
  #empty-state { flex: 1; display: flex; align-items: center; justify-content: center; color: #999; font-size: 16px; }
  .side-label { font-weight: 700; font-size: 16px; text-shadow: 0 0 4px #fff, 0 0 4px #fff, 0 0 4px #fff; line-height: 1; }
  .side-label.left { color: #2980b9; }
  .side-label.right { color: #d35400; }
  .arrow-icon { font-size: 20px; color: #333; text-shadow: 0 0 3px #fff, 0 0 3px #fff; line-height: 1; }
  .tag-label { font-size: 12px; font-weight: 700; background: rgba(255,255,255,0.85); padding: 2px 6px; border-radius: 3px; border: 1px solid #999; white-space: nowrap; line-height: 1.2; }
</style>
</head>
<body>
<div id="map"></div>
<div id="panel">
  <div id="header">
    <h1>Sidewalk Review</h1>
    <div id="progress"></div>
  </div>
  <div id="candidate-info"></div>
  <div class="keyboard-hint">Keys: <b>y</b> confirm, <b>n</b> reject, <b>s</b> skip, <b>&larr;&rarr;</b> navigate</div>
  <div id="buttons">
    <button id="btn-confirm" onclick="decide('confirmed')">Confirm (y)</button>
    <button id="btn-reject" onclick="decide('rejected')">Reject (n)</button>
    <button id="btn-skip" onclick="decide('skip')">Skip (s)</button>
  </div>
  <div id="nav">
    <button id="btn-prev" onclick="navigate(-1)">&larr; Prev</button>
    <span id="nav-pos"></span>
    <button id="btn-next" onclick="navigate(1)">Next &rarr;</button>
  </div>
  <div id="export-bar">
    <button id="btn-export" onclick="doExport()">Finish &amp; Export</button>
    <div id="export-result"></div>
  </div>
</div>

<script src="https://unpkg.com/leaflet@%s/dist/leaflet.js"></script>
<script>
var candidates = %s;
var decisions = {}; // road_id -> "confirmed" | "rejected" | "skip"
var currentIdx = 0;
var mapLayers = [];

var map = L.map('map').setView([37.9, -122.06], 15);
L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
  attribution: '&copy; OpenStreetMap contributors',
  maxZoom: 19
}).addTo(map);

function strokeColor(tag) {
  switch(tag) {
    case 'both': return '#2ecc71';
    case 'left': return '#3498db';
    case 'right': return '#e67e22';
    default: return '#95a5a6';
  }
}

function clearMapLayers() {
  mapLayers.forEach(function(l) { map.removeLayer(l); });
  mapLayers = [];
}

// Compute bearing in degrees (clockwise from north) between two [lat,lng] points.
function bearing(lat1, lon1, lat2, lon2) {
  var dLon = (lon2 - lon1) * Math.PI / 180;
  var y = Math.sin(dLon) * Math.cos(lat2 * Math.PI / 180);
  var x = Math.cos(lat1 * Math.PI / 180) * Math.sin(lat2 * Math.PI / 180) -
          Math.sin(lat1 * Math.PI / 180) * Math.cos(lat2 * Math.PI / 180) * Math.cos(dLon);
  return ((Math.atan2(y, x) * 180 / Math.PI) + 360) %% 360;
}

// Offset a lat/lng point perpendicular to a bearing by approx meters.
function offsetPoint(lat, lon, bearingDeg, meters) {
  // Approximate: 1 degree lat ~ 111320m, 1 degree lon ~ 111320*cos(lat) m
  var rad = bearingDeg * Math.PI / 180;
  var dLat = meters * Math.cos(rad) / 111320;
  var dLon = meters * Math.sin(rad) / (111320 * Math.cos(lat * Math.PI / 180));
  return [lat + dLat, lon + dLon];
}

function showCandidate(idx) {
  if (candidates.length === 0) {
    document.getElementById('candidate-info').innerHTML = '<div id="empty-state">No candidates to review.</div>';
    document.getElementById('btn-confirm').disabled = true;
    document.getElementById('btn-reject').disabled = true;
    document.getElementById('btn-skip').disabled = true;
    updateProgress();
    return;
  }
  currentIdx = Math.max(0, Math.min(idx, candidates.length - 1));
  var c = candidates[currentIdx];

  clearMapLayers();

  var bounds = L.latLngBounds([]);
  var roadLatLngs = c.road_coords.map(function(p) { return [p[1], p[0]]; });

  // 1. Draw whole road as a wide light outline for context.
  var outline = L.polyline(roadLatLngs, {
    color: '#ccc',
    weight: 8,
    opacity: 0.6
  }).addTo(map);
  mapLayers.push(outline);
  bounds.extend(outline.getBounds());

  // 2. Draw colored split sub-portions on top.
  if (c.splits) {
    c.splits.forEach(function(sp) {
      var latlngs = sp.coords.map(function(p) { return [p[1], p[0]]; });
      var line = L.polyline(latlngs, {
        color: strokeColor(sp.inferred_tag),
        weight: 5,
        opacity: 0.9
      }).addTo(map);
      mapLayers.push(line);
    });
  } else {
    var line = L.polyline(roadLatLngs, {
      color: strokeColor(c.proposed_tag),
      weight: 5,
      opacity: 0.9
    }).addTo(map);
    mapLayers.push(line);
  }

  // 3. Draw matched sidewalks.
  if (c.sidewalks) {
    c.sidewalks.forEach(function(sw) {
      var latlngs = sw.coords.map(function(p) { return [p[1], p[0]]; });
      var line = L.polyline(latlngs, {
        color: '#9b59b6',
        weight: 3,
        opacity: 0.7,
        dashArray: '6, 4'
      }).addTo(map);
      mapLayers.push(line);
      bounds.extend(line.getBounds());
    });
  }

  // 4. Compute midpoint and bearing for annotations.
  var midIdx = Math.floor(roadLatLngs.length / 2);
  var prevIdx = Math.max(0, midIdx - 1);
  var midLat = roadLatLngs[midIdx][0];
  var midLon = roadLatLngs[midIdx][1];
  var prevLat = roadLatLngs[prevIdx][0];
  var prevLon = roadLatLngs[prevIdx][1];
  var roadBearing = bearing(prevLat, prevLon, midLat, midLon);

  // Arrow marker showing way direction at midpoint.
  var arrowMarker = L.marker([midLat, midLon], {
    icon: L.divIcon({
      className: 'arrow-icon',
      html: '<div style="transform:rotate(' + (roadBearing - 90) + 'deg)">&#9654;</div>',
      iconSize: [20, 20],
      iconAnchor: [10, 10]
    }),
    interactive: false
  }).addTo(map);
  mapLayers.push(arrowMarker);

  // "L" label offset to the left of the way direction.
  // Left in OSM = left when traveling in way direction = bearing - 90°.
  var leftBearing = (roadBearing - 90 + 360) %% 360;
  var leftPos = offsetPoint(midLat, midLon, leftBearing, 25);
  var leftLabel = L.marker(leftPos, {
    icon: L.divIcon({
      className: 'side-label left',
      html: 'L',
      iconSize: [16, 16],
      iconAnchor: [8, 8]
    }),
    interactive: false
  }).addTo(map);
  mapLayers.push(leftLabel);

  // "R" label offset to the right.
  var rightBearing = (roadBearing + 90) %% 360;
  var rightPos = offsetPoint(midLat, midLon, rightBearing, 25);
  var rightLabel = L.marker(rightPos, {
    icon: L.divIcon({
      className: 'side-label right',
      html: 'R',
      iconSize: [16, 16],
      iconAnchor: [8, 8]
    }),
    interactive: false
  }).addTo(map);
  mapLayers.push(rightLabel);

  // Tag label near the start of the road.
  var startLat = roadLatLngs[0][0];
  var startLon = roadLatLngs[0][1];
  var tagLabel = L.marker([startLat, startLon], {
    icon: L.divIcon({
      className: 'tag-label',
      html: 'sidewalk:' + escapeHtml(c.proposed_tag) + '=separate',
      iconSize: null,
      iconAnchor: [-5, -5]
    }),
    interactive: false
  }).addTo(map);
  mapLayers.push(tagLabel);

  map.fitBounds(bounds, { padding: [40, 40] });

  // Update info panel.
  var decision = decisions[c.road_id];
  var statusHTML = '';
  if (decision === 'confirmed') statusHTML = '<span class="status-badge confirmed">Confirmed</span>';
  else if (decision === 'rejected') statusHTML = '<span class="status-badge rejected">Rejected</span>';

  var leftPct = Math.round(c.left_coverage * 100);
  var rightPct = Math.round(c.right_coverage * 100);

  document.getElementById('candidate-info').innerHTML =
    '<div class="info-row"><label>Road:</label> <b>' + escapeHtml(c.name) + '</b>' + statusHTML + '</div>' +
    '<div class="info-row"><label>Highway:</label> ' + escapeHtml(c.highway) + '</div>' +
    '<div class="info-row"><label>Proposed:</label> <span class="proposed-tag ' + c.proposed_tag + '">sidewalk:' + c.proposed_tag + '=separate</span></div>' +
    '<div class="info-row"><label>Left:</label> ' + leftPct + '%%' +
      '<span class="coverage-bar"><span class="coverage-fill left" style="width:' + leftPct + '%%"></span></span></div>' +
    '<div class="info-row"><label>Right:</label> ' + rightPct + '%%' +
      '<span class="coverage-bar"><span class="coverage-fill right" style="width:' + rightPct + '%%"></span></span></div>' +
    '<div class="info-row"><label>OSM:</label> <a href="' + escapeHtml(c.osm_url) + '" target="_blank">way/' + c.road_id + '</a></div>';

  // Update nav.
  document.getElementById('nav-pos').textContent = (currentIdx + 1) + ' of ' + candidates.length;
  document.getElementById('btn-prev').disabled = (currentIdx === 0);
  document.getElementById('btn-next').disabled = (currentIdx === candidates.length - 1);

  updateProgress();
}

function updateProgress() {
  var confirmed = 0, rejected = 0, skipped = 0;
  for (var k in decisions) {
    if (decisions[k] === 'confirmed') confirmed++;
    else if (decisions[k] === 'rejected') rejected++;
    else if (decisions[k] === 'skip') skipped++;
  }
  var remaining = candidates.length - confirmed - rejected - skipped;
  document.getElementById('progress').innerHTML =
    '<span class="confirmed">' + confirmed + ' confirmed</span> &middot; ' +
    '<span class="rejected">' + rejected + ' rejected</span> &middot; ' +
    '<span class="remaining">' + remaining + ' remaining</span>';
}

function decide(d) {
  if (candidates.length === 0) return;
  var c = candidates[currentIdx];
  decisions[c.road_id] = d;
  // Auto-advance to next unreviewed.
  if (currentIdx < candidates.length - 1) {
    showCandidate(currentIdx + 1);
  } else {
    showCandidate(currentIdx); // refresh current to show badge
  }
}

function navigate(delta) {
  showCandidate(currentIdx + delta);
}

function doExport() {
  var confirmed = [], rejected = [];
  for (var k in decisions) {
    var id = parseInt(k, 10);
    if (decisions[k] === 'confirmed') confirmed.push(id);
    else if (decisions[k] === 'rejected') rejected.push(id);
  }

  if (confirmed.length === 0 && rejected.length === 0) {
    alert('No candidates have been confirmed or rejected yet.');
    return;
  }

  document.getElementById('btn-export').disabled = true;
  document.getElementById('btn-export').textContent = 'Exporting...';

  fetch('/api/export', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({confirmed: confirmed, rejected: rejected})
  })
  .then(function(resp) {
    if (!resp.ok) return resp.text().then(function(t) { throw new Error(t); });
    return resp.json();
  })
  .then(function(data) {
    var el = document.getElementById('export-result');
    el.style.display = 'block';
    el.innerHTML = 'Exported ' + data.confirmed_count + ' confirmed to <b>' + escapeHtml(data.confirmed_path) +
      '</b> and ' + data.rejected_count + ' rejected to <b>' + escapeHtml(data.rejected_path) + '</b>.';
    document.getElementById('btn-export').textContent = 'Finish & Export';
    document.getElementById('btn-export').disabled = false;
  })
  .catch(function(err) {
    alert('Export failed: ' + err.message);
    document.getElementById('btn-export').textContent = 'Finish & Export';
    document.getElementById('btn-export').disabled = false;
  });
}

function escapeHtml(s) {
  if (!s) return '';
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

document.addEventListener('keydown', function(e) {
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
  switch(e.key) {
    case 'y': decide('confirmed'); break;
    case 'n': decide('rejected'); break;
    case 's': decide('skip'); break;
    case 'ArrowLeft': navigate(-1); break;
    case 'ArrowRight': navigate(1); break;
  }
});

// Init.
showCandidate(0);
</script>
</body>
</html>
`
