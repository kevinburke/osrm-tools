package sidewalk

import (
	"log/slog"

	"github.com/kevinburke/osrm-tools/geo"
)

// gridCellDegrees is the spatial grid cell size in degrees.
// ~0.00045° ≈ 50 m at mid-latitudes.
const gridCellDegrees = 0.00045

// MatchRoads finds sidewalk matches for each road and returns candidates where
// a match was found. Roads, sidewalks, and buildings should already have their
// coordinates resolved.
func MatchRoads(logger *slog.Logger, roads []Road, sidewalks []Sidewalk, buildings []Building, cfg Config) []Candidate {
	if cfg.MaxDistance == 0 {
		cfg.MaxDistance = 20
	}
	if cfg.ParallelThreshold == 0 {
		cfg.ParallelThreshold = 20
	}
	if cfg.MinCoverage == 0 {
		cfg.MinCoverage = 0.5
	}

	// Convert MaxDistance to approximate degrees for the grid query radius.
	// Use a conservative (small-latitude) conversion so we don't miss candidates.
	radiusDeg := cfg.MaxDistance / geo.MetersPerDegreeLatitude

	// Build spatial indexes.
	grid := NewGrid(gridCellDegrees)

	logger.Info("building spatial index",
		"sidewalks", len(sidewalks),
		"buildings", len(buildings),
	)

	for i := range sidewalks {
		sw := &sidewalks[i]
		for j := 0; j+1 < len(sw.Coords); j++ {
			grid.AddSegment(sw.ID, j, sw.Coords[j], sw.Coords[j+1])
		}
	}
	if cfg.CheckBuildings {
		for i := range buildings {
			b := &buildings[i]
			grid.AddPolygon(b.ID, b.Coords)
		}
	}

	logger.Info("matching roads to sidewalks", "roads", len(roads))

	var candidates []Candidate
	for ri := range roads {
		road := &roads[ri]
		if len(road.Coords) < 2 {
			continue
		}

		cand := matchOneRoad(road, grid, cfg, radiusDeg)
		if cand != nil {
			candidates = append(candidates, *cand)
		}
	}

	logger.Info("matching complete", "candidates", len(candidates))
	return candidates
}

// matchOneRoad attempts to match sidewalks to one road. Returns nil if no
// match meets the coverage threshold.
func matchOneRoad(road *Road, grid *Grid, cfg Config, radiusDeg float64) *Candidate {
	nSegs := len(road.Coords) - 1
	segResults := make([]SegmentResult, nSegs)
	totalLength := 0.0
	var matches []Match

	for i := range nSegs {
		rA := road.Coords[i]
		rB := road.Coords[i+1]
		sLen := segmentLength(rA, rB)
		segResults[i].Length = sLen
		totalLength += sLen

		if sLen < 0.1 {
			continue // skip degenerate segments
		}

		roadBearing := Bearing(rA, rB)
		nearby := grid.NearbySegments(rA, rB, radiusDeg)

		for _, ref := range nearby {
			// Distance check.
			dist := SegmentDistance(rA, rB, ref.A, ref.B)
			if dist > cfg.MaxDistance {
				continue
			}

			// Parallelism check.
			swBearing := Bearing(ref.A, ref.B)
			if !BearingsParallel(roadBearing, swBearing, cfg.ParallelThreshold) {
				continue
			}

			// Building obstruction check.
			if cfg.CheckBuildings {
				roadMid := segmentMidpoint(rA, rB)
				swMid := segmentMidpoint(ref.A, ref.B)
				obstructed := false
				for _, poly := range grid.NearbyPolygons(roadMid, swMid) {
					if SegmentIntersectsPolygon(roadMid, swMid, poly.Coords) {
						obstructed = true
						break
					}
				}
				if obstructed {
					continue
				}
			}

			// Determine side using the midpoint of the sidewalk segment.
			swMid := segmentMidpoint(ref.A, ref.B)
			side := SegmentSide(rA, rB, swMid)

			if side == "left" {
				segResults[i].HasLeft = true
			} else {
				segResults[i].HasRight = true
			}

			matches = append(matches, Match{
				RoadID:     road.ID,
				SidewalkID: ref.ID,
				Side:       side,
			})
		}
	}

	if totalLength < 0.1 {
		return nil
	}

	// Aggregate coverage.
	var leftLen, rightLen float64
	for _, r := range segResults {
		if r.HasLeft {
			leftLen += r.Length
		}
		if r.HasRight {
			rightLen += r.Length
		}
	}

	leftCov := leftLen / totalLength
	rightCov := rightLen / totalLength

	hasLeft := leftCov >= cfg.MinCoverage
	hasRight := rightCov >= cfg.MinCoverage

	if !hasLeft && !hasRight {
		return nil
	}

	var tag string
	switch {
	case hasLeft && hasRight:
		tag = "both"
	case hasLeft:
		tag = "left"
	default:
		tag = "right"
	}

	// Set coverage on matches.
	for i := range matches {
		if matches[i].Side == "left" {
			matches[i].Coverage = leftCov
		} else {
			matches[i].Coverage = rightCov
		}
	}

	return &Candidate{
		Road:           *road,
		InferredTag:    tag,
		Matches:        matches,
		LeftCoverage:   leftCov,
		RightCoverage:  rightCov,
		SegmentResults: segResults,
	}
}

// segTag returns the inferred sidewalk tag for a single segment's match status.
func segTag(hasLeft, hasRight bool) string {
	switch {
	case hasLeft && hasRight:
		return "both"
	case hasLeft:
		return "left"
	case hasRight:
		return "right"
	default:
		return ""
	}
}

// SplitCandidate splits a whole-road candidate into sub-candidates at the
// boundaries where sidewalk coverage changes. Each returned candidate covers
// a contiguous run of road segments with the same inferred tag. Runs with no
// sidewalk match are omitted.
//
// minRunLength controls the minimum length (meters) for a run to be emitted
// on its own; shorter runs are merged into the preceding run to avoid noisy
// micro-segments. Use 0 to disable merging.
func SplitCandidate(c *Candidate, minRunLength float64) []Candidate {
	if len(c.SegmentResults) == 0 {
		return nil
	}

	type run struct {
		startSeg int // first segment index (inclusive)
		endSeg   int // last segment index (inclusive)
		tag      string
		length   float64
	}

	// Build initial runs of consecutive segments with the same tag.
	segs := c.SegmentResults
	var runs []run
	cur := run{
		startSeg: 0,
		tag:      segTag(segs[0].HasLeft, segs[0].HasRight),
		length:   segs[0].Length,
	}
	for i := 1; i < len(segs); i++ {
		t := segTag(segs[i].HasLeft, segs[i].HasRight)
		if t == cur.tag {
			cur.endSeg = i
			cur.length += segs[i].Length
		} else {
			runs = append(runs, cur)
			cur = run{startSeg: i, endSeg: i, tag: t, length: segs[i].Length}
		}
	}
	runs = append(runs, cur)

	// If there's only one run, no split needed — return original as-is.
	if len(runs) == 1 {
		return []Candidate{*c}
	}

	// Merge short runs into their predecessor to avoid noisy micro-segments.
	if minRunLength > 0 {
		merged := []run{runs[0]}
		for _, r := range runs[1:] {
			if r.length < minRunLength {
				// Absorb into previous run.
				prev := &merged[len(merged)-1]
				prev.endSeg = r.endSeg
				prev.length += r.length
				// Keep the previous run's tag (it was longer/established first).
			} else {
				merged = append(merged, r)
			}
		}
		runs = merged
	}

	// Build sub-candidates for each run that has a sidewalk tag.
	var result []Candidate
	for _, r := range runs {
		if r.tag == "" {
			continue
		}

		// Coords for segment range [startSeg, endSeg]: nodes startSeg..endSeg+1
		subCoords := c.Road.Coords[r.startSeg : r.endSeg+2]

		// Collect matches whose sidewalk segments overlap this run.
		// (We use a simple approach: include matches from the whole candidate
		// that are on the matching side.)
		var subMatches []Match
		seen := make(map[int64]bool)
		for _, m := range c.Matches {
			if seen[m.SidewalkID] {
				continue
			}
			// Include if the match side is consistent with this run's tag.
			switch r.tag {
			case "both":
				seen[m.SidewalkID] = true
				subMatches = append(subMatches, m)
			case "left":
				if m.Side == "left" {
					seen[m.SidewalkID] = true
					subMatches = append(subMatches, m)
				}
			case "right":
				if m.Side == "right" {
					seen[m.SidewalkID] = true
					subMatches = append(subMatches, m)
				}
			}
		}

		var leftCov, rightCov float64
		switch r.tag {
		case "both":
			leftCov, rightCov = 1, 1
		case "left":
			leftCov = 1
		case "right":
			rightCov = 1
		}

		result = append(result, Candidate{
			Road: Road{
				ID:     c.Road.ID,
				Tags:   c.Road.Tags,
				Coords: subCoords,
			},
			InferredTag:    r.tag,
			Matches:        subMatches,
			LeftCoverage:   leftCov,
			RightCoverage:  rightCov,
			SegmentResults: segs[r.startSeg : r.endSeg+1],
		})
	}

	return result
}
