// Package sidewalk provides geometric matching of separately-mapped OSM
// sidewalk ways to their parent roads, inferring sidewalk=left/right/both tags.
package sidewalk

// Road is an OSM way with a highway tag that lacks a sidewalk tag.
type Road struct {
	ID     int64
	Tags   map[string]string
	Coords [][2]float64 // [lon, lat] pairs in way-node order
}

// Sidewalk is an OSM way tagged highway=footway + footway=sidewalk (or path variant).
type Sidewalk struct {
	ID     int64
	Coords [][2]float64
}

// Building is an OSM way with a building=* tag, stored as a closed polygon.
type Building struct {
	ID     int64
	Coords [][2]float64
}

// Match records one sidewalk-to-road segment association.
type Match struct {
	RoadID     int64
	SidewalkID int64
	Side       string  // "left" or "right"
	Coverage   float64 // fraction of road length covered by this match
}

// SegmentResult records the sidewalk match status for a single road segment
// (the segment between Coords[i] and Coords[i+1]).
type SegmentResult struct {
	HasLeft  bool
	HasRight bool
	Length   float64 // segment length in meters
}

// Candidate is a road (or sub-portion of a road) with its inferred sidewalk annotation.
type Candidate struct {
	Road          Road
	InferredTag   string // "left", "right", "both", or "" if no match
	Matches       []Match
	LeftCoverage  float64
	RightCoverage float64
	// SegmentResults has one entry per road segment (len = len(Road.Coords)-1).
	// Populated by MatchRoads; used by SplitCandidate.
	SegmentResults []SegmentResult
}

// Config controls the matching algorithm parameters.
type Config struct {
	// MaxDistance is the maximum road-to-sidewalk distance in meters (default 20).
	MaxDistance float64
	// ParallelThreshold is the maximum bearing difference in degrees to consider
	// two segments parallel (default 20).
	ParallelThreshold float64
	// MinCoverage is the minimum fraction of road length that must be covered by
	// sidewalk matches on a given side to declare that side has a sidewalk (default 0.5).
	MinCoverage float64
	// CheckBuildings enables building obstruction checks between road and sidewalk.
	CheckBuildings bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxDistance:       20,
		ParallelThreshold: 20,
		MinCoverage:       0.5,
		CheckBuildings:    true,
	}
}
