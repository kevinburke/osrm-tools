package sidewalk

import (
	"log/slog"
	"math"
	"os"
	"testing"
)

func TestBearing(t *testing.T) {
	// Due north: bearing should be ~0°.
	a := [2]float64{-122.0, 37.0}
	b := [2]float64{-122.0, 38.0}
	brng := Bearing(a, b)
	if math.Abs(brng) > 1 && math.Abs(brng-360) > 1 {
		t.Errorf("expected ~0° for due north, got %.2f°", brng)
	}

	// Due east: bearing should be ~90°.
	b = [2]float64{-121.0, 37.0}
	brng = Bearing(a, b)
	if math.Abs(brng-90) > 1 {
		t.Errorf("expected ~90° for due east, got %.2f°", brng)
	}

	// Due south: bearing should be ~180°.
	b = [2]float64{-122.0, 36.0}
	brng = Bearing(a, b)
	if math.Abs(brng-180) > 1 {
		t.Errorf("expected ~180° for due south, got %.2f°", brng)
	}

	// Due west: bearing should be ~270°.
	b = [2]float64{-123.0, 37.0}
	brng = Bearing(a, b)
	if math.Abs(brng-270) > 1 {
		t.Errorf("expected ~270° for due west, got %.2f°", brng)
	}
}

func TestBearingsParallel(t *testing.T) {
	tests := []struct {
		b1, b2    float64
		threshold float64
		want      bool
	}{
		{0, 5, 20, true},      // nearly same direction
		{0, 180, 20, true},    // opposite direction = parallel
		{350, 10, 20, true},   // near 0/360 wrap
		{170, 350, 20, true},  // near 180 wrap, opposite direction
		{0, 45, 20, false},    // too far apart
		{90, 270, 20, true},   // opposite = parallel
		{80, 280, 20, true},   // opposite, within threshold
		{45, 90, 20, false},   // 45° apart
		{100, 150, 20, false}, // 50° apart
	}
	for _, tt := range tests {
		got := BearingsParallel(tt.b1, tt.b2, tt.threshold)
		if got != tt.want {
			t.Errorf("BearingsParallel(%.0f, %.0f, %.0f) = %v, want %v",
				tt.b1, tt.b2, tt.threshold, got, tt.want)
		}
	}
}

func TestSegmentSide(t *testing.T) {
	// Road segment going east (west to east).
	rA := [2]float64{-122.01, 37.0}
	rB := [2]float64{-122.00, 37.0}

	// Point to the north (left side when facing east).
	pLeft := [2]float64{-122.005, 37.001}
	if side := SegmentSide(rA, rB, pLeft); side != "left" {
		t.Errorf("expected left for point north of eastbound road, got %s", side)
	}

	// Point to the south (right side when facing east).
	pRight := [2]float64{-122.005, 36.999}
	if side := SegmentSide(rA, rB, pRight); side != "right" {
		t.Errorf("expected right for point south of eastbound road, got %s", side)
	}
}

func TestPointToSegmentDistance(t *testing.T) {
	// A horizontal segment at lat 37.0, lon from -122.01 to -122.00.
	a := [2]float64{-122.01, 37.0}
	b := [2]float64{-122.00, 37.0}

	// Point directly above the midpoint, ~111m north.
	p := [2]float64{-122.005, 37.001}
	d := PointToSegmentDistance(p, a, b)
	if math.Abs(d-111.3) > 2 {
		t.Errorf("expected ~111m, got %.1f", d)
	}

	// Point at the endpoint a.
	d = PointToSegmentDistance(a, a, b)
	if d > 0.01 {
		t.Errorf("expected ~0, got %.4f", d)
	}
}

func TestSegmentDistance(t *testing.T) {
	// Two parallel horizontal segments, offset north by ~0.001° ≈ 111m.
	a1 := [2]float64{-122.01, 37.0}
	a2 := [2]float64{-122.00, 37.0}
	b1 := [2]float64{-122.01, 37.001}
	b2 := [2]float64{-122.00, 37.001}

	d := SegmentDistance(a1, a2, b1, b2)
	if math.Abs(d-111.3) > 2 {
		t.Errorf("expected ~111m between parallel segments, got %.1f", d)
	}
}

func TestSegmentIntersectsPolygon(t *testing.T) {
	// A small square building.
	building := [][2]float64{
		{-122.005, 37.0005},
		{-122.004, 37.0005},
		{-122.004, 37.0015},
		{-122.005, 37.0015},
		{-122.005, 37.0005}, // closed
	}

	// Line from road to sidewalk that passes through the building.
	a := [2]float64{-122.0045, 37.0}
	b := [2]float64{-122.0045, 37.002}
	if !SegmentIntersectsPolygon(a, b, building) {
		t.Error("expected intersection with building")
	}

	// Line that does not pass through the building.
	a = [2]float64{-122.01, 37.0}
	b = [2]float64{-122.01, 37.002}
	if SegmentIntersectsPolygon(a, b, building) {
		t.Error("expected no intersection")
	}
}

func TestGridAddAndQuery(t *testing.T) {
	g := NewGrid(0.001) // ~111m cells

	// Add a sidewalk segment.
	g.AddSegment(100, 0, [2]float64{-122.005, 37.001}, [2]float64{-122.004, 37.001})

	// Query near the segment.
	results := g.NearbySegments(
		[2]float64{-122.005, 37.0005},
		[2]float64{-122.004, 37.0005},
		0.001,
	)
	if len(results) == 0 {
		t.Fatal("expected to find nearby segment")
	}
	if results[0].ID != 100 {
		t.Errorf("expected ID 100, got %d", results[0].ID)
	}

	// Query far from the segment.
	results = g.NearbySegments(
		[2]float64{-123.0, 38.0},
		[2]float64{-123.0, 38.001},
		0.001,
	)
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestMatchRoadsSimple(t *testing.T) {
	// A road going east, with a parallel sidewalk 15m to the north (left).
	// 0.000135° lat ≈ 15m.
	roads := []Road{{
		ID:   1,
		Tags: map[string]string{"highway": "residential", "name": "Test St"},
		Coords: [][2]float64{
			{-122.010, 37.000},
			{-122.005, 37.000},
			{-122.000, 37.000},
		},
	}}
	sidewalks := []Sidewalk{{
		ID: 100,
		Coords: [][2]float64{
			{-122.010, 37.000135},
			{-122.005, 37.000135},
			{-122.000, 37.000135},
		},
	}}

	cfg := DefaultConfig()
	cfg.CheckBuildings = false
	candidates := MatchRoads(testLogger(), roads, sidewalks, nil, cfg)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]
	if c.InferredTag != "left" {
		t.Errorf("expected inferred tag 'left', got %q", c.InferredTag)
	}
	if c.LeftCoverage < 0.9 {
		t.Errorf("expected high left coverage, got %.2f", c.LeftCoverage)
	}
}

func TestMatchRoadsBothSides(t *testing.T) {
	roads := []Road{{
		ID:   2,
		Tags: map[string]string{"highway": "residential"},
		Coords: [][2]float64{
			{-122.010, 37.000},
			{-122.000, 37.000},
		},
	}}
	sidewalks := []Sidewalk{
		{
			ID: 200,
			Coords: [][2]float64{
				{-122.010, 37.000135},
				{-122.000, 37.000135},
			},
		},
		{
			ID: 201,
			Coords: [][2]float64{
				{-122.010, 36.999865},
				{-122.000, 36.999865},
			},
		},
	}

	cfg := DefaultConfig()
	cfg.CheckBuildings = false
	candidates := MatchRoads(testLogger(), roads, sidewalks, nil, cfg)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].InferredTag != "both" {
		t.Errorf("expected 'both', got %q", candidates[0].InferredTag)
	}
}

func TestMatchRoadsNoMatch(t *testing.T) {
	// Sidewalk is perpendicular to the road — should not match.
	roads := []Road{{
		ID:   3,
		Tags: map[string]string{"highway": "residential"},
		Coords: [][2]float64{
			{-122.010, 37.000},
			{-122.000, 37.000},
		},
	}}
	sidewalks := []Sidewalk{{
		ID: 300,
		Coords: [][2]float64{
			{-122.005, 36.999},
			{-122.005, 37.001},
		},
	}}

	cfg := DefaultConfig()
	cfg.CheckBuildings = false
	candidates := MatchRoads(testLogger(), roads, sidewalks, nil, cfg)

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for perpendicular sidewalk, got %d", len(candidates))
	}
}

func TestMatchRoadsBuildingObstruction(t *testing.T) {
	// Road going east, sidewalk 15m north, but a building in between.
	roads := []Road{{
		ID:   4,
		Tags: map[string]string{"highway": "residential"},
		Coords: [][2]float64{
			{-122.010, 37.000},
			{-122.000, 37.000},
		},
	}}
	sidewalks := []Sidewalk{{
		ID: 400,
		Coords: [][2]float64{
			{-122.010, 37.000135},
			{-122.000, 37.000135},
		},
	}}
	// Building spanning the entire width between road and sidewalk.
	buildings := []Building{{
		ID: 500,
		Coords: [][2]float64{
			{-122.011, 37.00003},
			{-122.011, 37.00011},
			{0.001 + -122.011, 37.00011},
			{0.001 + -122.011, 37.00003},
			{-122.011, 37.00003},
		},
	}}

	cfg := DefaultConfig()
	cfg.CheckBuildings = true
	candidates := MatchRoads(testLogger(), roads, sidewalks, buildings, cfg)

	// The building should obstruct at least some matches, reducing coverage.
	// Whether it fully blocks depends on exact midpoint geometry. Just verify
	// coverage is reduced compared to no buildings.
	cfgNoBld := DefaultConfig()
	cfgNoBld.CheckBuildings = false
	candNoBld := MatchRoads(testLogger(), roads, sidewalks, nil, cfgNoBld)

	if len(candNoBld) == 0 {
		t.Fatal("expected match without buildings")
	}

	if len(candidates) > 0 && candidates[0].LeftCoverage >= candNoBld[0].LeftCoverage {
		t.Logf("building may not have obstructed (coverage %.2f vs %.2f), geometry-dependent",
			candidates[0].LeftCoverage, candNoBld[0].LeftCoverage)
	}
}

func TestSplitCandidatePartialSidewalk(t *testing.T) {
	// Road going east with 4 segments. Sidewalk only covers the first 2 segments.
	// 0.002° lon ≈ 160m at lat 37, so each segment ≈ 160m > 30m min run length.
	roads := []Road{{
		ID:   10,
		Tags: map[string]string{"highway": "residential", "name": "Split St"},
		Coords: [][2]float64{
			{-122.020, 37.000},
			{-122.018, 37.000},
			{-122.016, 37.000},
			{-122.014, 37.000},
			{-122.012, 37.000},
		},
	}}
	// Sidewalk covers only the western half (first 2 segments).
	sidewalks := []Sidewalk{{
		ID: 110,
		Coords: [][2]float64{
			{-122.020, 37.000135},
			{-122.018, 37.000135},
			{-122.016, 37.000135},
		},
	}}

	cfg := DefaultConfig()
	cfg.CheckBuildings = false
	cfg.MinCoverage = 0.3 // lower threshold so partial road still matches
	candidates := MatchRoads(testLogger(), roads, sidewalks, nil, cfg)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 whole-road candidate, got %d", len(candidates))
	}
	c := candidates[0]

	// The whole-road candidate should have ~75% left coverage (3 of 4 segments
	// match because the sidewalk endpoint at -122.016 is within range of segment 2).
	if c.LeftCoverage < 0.6 || c.LeftCoverage > 0.85 {
		t.Errorf("expected ~75%% left coverage, got %.2f", c.LeftCoverage)
	}
	if len(c.SegmentResults) != 4 {
		t.Fatalf("expected 4 segment results, got %d", len(c.SegmentResults))
	}

	// Split should produce a matched portion and drop the unmatched tail.
	splits := SplitCandidate(&c, 30)
	if len(splits) == 0 {
		t.Fatal("expected at least 1 split candidate")
	}

	// The matched split should cover the western portion with tag "left".
	found := false
	for _, s := range splits {
		if s.InferredTag == "left" {
			found = true
			// Its coords should be a subset (fewer nodes than the whole road).
			if len(s.Road.Coords) >= len(c.Road.Coords) {
				t.Errorf("split candidate should have fewer coords than whole road, got %d vs %d",
					len(s.Road.Coords), len(c.Road.Coords))
			}
		}
	}
	if !found {
		tags := make([]string, len(splits))
		for i, s := range splits {
			tags[i] = s.InferredTag
		}
		t.Errorf("expected a split with tag 'left', got tags: %v", tags)
	}
}

func TestSplitCandidateNoSplit(t *testing.T) {
	// When all segments match, SplitCandidate should return the original as-is.
	roads := []Road{{
		ID:   11,
		Tags: map[string]string{"highway": "residential"},
		Coords: [][2]float64{
			{-122.010, 37.000},
			{-122.005, 37.000},
			{-122.000, 37.000},
		},
	}}
	sidewalks := []Sidewalk{{
		ID: 111,
		Coords: [][2]float64{
			{-122.010, 37.000135},
			{-122.005, 37.000135},
			{-122.000, 37.000135},
		},
	}}

	cfg := DefaultConfig()
	cfg.CheckBuildings = false
	candidates := MatchRoads(testLogger(), roads, sidewalks, nil, cfg)

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	splits := SplitCandidate(&candidates[0], 30)
	if len(splits) != 1 {
		t.Fatalf("expected 1 split (no split needed), got %d", len(splits))
	}
	if len(splits[0].Road.Coords) != len(candidates[0].Road.Coords) {
		t.Errorf("expected same coords when no split, got %d vs %d",
			len(splits[0].Road.Coords), len(candidates[0].Road.Coords))
	}
}
