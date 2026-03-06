package sidewalk

import "math"

// gridKey is a cell coordinate in the spatial grid.
type gridKey struct {
	x, y int
}

// SegmentRef identifies a specific segment within a sidewalk way.
type SegmentRef struct {
	ID     int64 // sidewalk way ID
	SegIdx int   // index of the first node in the segment (segment is Coords[SegIdx]–Coords[SegIdx+1])
	A, B   [2]float64
}

// PolygonRef identifies a building polygon.
type PolygonRef struct {
	ID     int64
	Coords [][2]float64
}

// Grid is a grid-based spatial index for fast nearby-element queries.
// Cell size is specified in degrees; ~0.00045° ≈ 50 m at mid-latitudes.
type Grid struct {
	cellSize float64
	segments map[gridKey][]SegmentRef
	polygons map[gridKey][]PolygonRef
}

// NewGrid creates a spatial grid with the given cell size in degrees.
// For ~50 m cells at mid-latitudes, use approximately 0.00045.
func NewGrid(cellSizeDegrees float64) *Grid {
	return &Grid{
		cellSize: cellSizeDegrees,
		segments: make(map[gridKey][]SegmentRef),
		polygons: make(map[gridKey][]PolygonRef),
	}
}

// cellsForSegment returns all grid cells that the bounding box of a–b touches.
func (g *Grid) cellsForSegment(a, b [2]float64) []gridKey {
	minLon := math.Min(a[0], b[0])
	maxLon := math.Max(a[0], b[0])
	minLat := math.Min(a[1], b[1])
	maxLat := math.Max(a[1], b[1])

	x0 := int(math.Floor(minLon / g.cellSize))
	x1 := int(math.Floor(maxLon / g.cellSize))
	y0 := int(math.Floor(minLat / g.cellSize))
	y1 := int(math.Floor(maxLat / g.cellSize))

	keys := make([]gridKey, 0, (x1-x0+1)*(y1-y0+1))
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			keys = append(keys, gridKey{x, y})
		}
	}
	return keys
}

// AddSegment inserts a sidewalk segment into the grid.
func (g *Grid) AddSegment(id int64, segIdx int, a, b [2]float64) {
	ref := SegmentRef{ID: id, SegIdx: segIdx, A: a, B: b}
	for _, k := range g.cellsForSegment(a, b) {
		g.segments[k] = append(g.segments[k], ref)
	}
}

// AddPolygon inserts a building polygon into all cells its bounding box covers.
func (g *Grid) AddPolygon(id int64, coords [][2]float64) {
	if len(coords) < 3 {
		return
	}
	minLon, maxLon := coords[0][0], coords[0][0]
	minLat, maxLat := coords[0][1], coords[0][1]
	for _, c := range coords[1:] {
		minLon = math.Min(minLon, c[0])
		maxLon = math.Max(maxLon, c[0])
		minLat = math.Min(minLat, c[1])
		maxLat = math.Max(maxLat, c[1])
	}

	ref := PolygonRef{ID: id, Coords: coords}
	x0 := int(math.Floor(minLon / g.cellSize))
	x1 := int(math.Floor(maxLon / g.cellSize))
	y0 := int(math.Floor(minLat / g.cellSize))
	y1 := int(math.Floor(maxLat / g.cellSize))
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			g.polygons[gridKey{x, y}] = append(g.polygons[gridKey{x, y}], ref)
		}
	}
}

// NearbySegments returns all sidewalk segment refs in cells that the bounding
// box of a–b (expanded by radiusDegrees) touches. The caller must still check
// actual distances.
func (g *Grid) NearbySegments(a, b [2]float64, radiusDegrees float64) []SegmentRef {
	expanded_a := [2]float64{a[0] - radiusDegrees, a[1] - radiusDegrees}
	expanded_b := [2]float64{b[0] + radiusDegrees, b[1] + radiusDegrees}
	// Also include the original segment extent.
	expanded_a[0] = math.Min(expanded_a[0], math.Min(a[0], b[0])-radiusDegrees)
	expanded_a[1] = math.Min(expanded_a[1], math.Min(a[1], b[1])-radiusDegrees)
	expanded_b[0] = math.Max(expanded_b[0], math.Max(a[0], b[0])+radiusDegrees)
	expanded_b[1] = math.Max(expanded_b[1], math.Max(a[1], b[1])+radiusDegrees)

	seen := make(map[SegmentRef]struct{})
	var result []SegmentRef
	for _, k := range g.cellsForSegment(expanded_a, expanded_b) {
		for _, ref := range g.segments[k] {
			if _, ok := seen[ref]; !ok {
				seen[ref] = struct{}{}
				result = append(result, ref)
			}
		}
	}
	return result
}

// NearbyPolygons returns all building polygon refs in the cells that the
// bounding box of a–b touches.
func (g *Grid) NearbyPolygons(a, b [2]float64) []PolygonRef {
	seen := make(map[int64]struct{})
	var result []PolygonRef
	for _, k := range g.cellsForSegment(a, b) {
		for _, ref := range g.polygons[k] {
			if _, ok := seen[ref.ID]; !ok {
				seen[ref.ID] = struct{}{}
				result = append(result, ref)
			}
		}
	}
	return result
}
