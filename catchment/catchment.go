// Package catchment provides an N-destination catchment area calculator.
// For each point on a grid, it routes to every destination and assigns the point
// to whichever destination has the shortest travel time.
package catchment

import (
	"context"
	"log/slog"

	"github.com/kevinburke/osrm-tools/geo"
	"github.com/kevinburke/osrm-tools/osrm"
)

// Destination is one of the N target locations in a catchment analysis.
type Destination struct {
	ID    string    // Short identifier (e.g. "A", "northside")
	Name  string    // Display name (e.g. "Northside Elementary")
	Point geo.Point // Location
	Color string    // Hex color for GeoJSON output (e.g. "#FF0000")
}

// GridPoint represents a sampled point with routing results to each destination.
type GridPoint struct {
	geo.Point
	AssignedTo      string             // Destination ID of the closest destination
	TravelTimes     map[string]float64 // Destination ID -> travel time in seconds
	MinTime         float64            // Travel time to assigned destination (seconds)
	RouteGeometries map[string]any     // Destination ID -> GeoJSON geometry (debug mode only)
}

// RegionAlgorithm determines how polygons are generated from classified grid points.
type RegionAlgorithm string

const (
	RegionAlgorithmConcaveHull RegionAlgorithm = "concave_hull"
	RegionAlgorithmAdjacency   RegionAlgorithm = "adjacency"
)

// Calculator handles the catchment area calculation.
type Calculator struct {
	Destinations    []Destination
	BoundsMin       geo.Point   // Southwest corner of area to analyze
	BoundsMax       geo.Point   // Northeast corner of area to analyze
	PolygonBounds   []geo.Point // Optional: vertices of convex polygon for precise bounds
	GridSpacing     float64     // Grid spacing in degrees (e.g., 0.001 ~ 111 meters at equator)
	OSRMClient      *osrm.Client
	Profile         string // OSRM routing profile (e.g. "cycling")
	DebugMode       bool
	RegionAlgorithm RegionAlgorithm
	Logger          *slog.Logger

	// Road proximity filtering
	MaxRoadDistance   float64 // Maximum distance in meters from a road for a point to be valid
	RoadFilterProfile string  // OSRM profile for road filtering (e.g. "driving")

	// Polygon generation
	ConcaveHullRatio float64 // 0.0 = most concave, 1.0 = convex hull
}

// NewCalculator creates a Calculator for the given destinations and bounds.
func NewCalculator(destinations []Destination, boundsMin, boundsMax geo.Point, gridSpacing float64, osrmClient *osrm.Client) *Calculator {
	return &Calculator{
		Destinations:      destinations,
		BoundsMin:         boundsMin,
		BoundsMax:         boundsMax,
		GridSpacing:       gridSpacing,
		OSRMClient:        osrmClient,
		Profile:           "cycling",
		DebugMode:         false,
		RegionAlgorithm:   RegionAlgorithmConcaveHull,
		MaxRoadDistance:   68.58, // 75 yards in meters
		RoadFilterProfile: "driving",
		ConcaveHullRatio:  0.05,
		Logger:            slog.Default(),
	}
}

// NewCalculatorWithPolygon creates a Calculator with convex polygon bounds.
// The bounding box is computed automatically from the polygon vertices.
func NewCalculatorWithPolygon(destinations []Destination, polygonBounds []geo.Point, gridSpacing float64, osrmClient *osrm.Client) *Calculator {
	sw, ne := geo.BoundingBox(polygonBounds)

	c := NewCalculator(destinations, sw, ne, gridSpacing, osrmClient)
	c.PolygonBounds = polygonBounds
	return c
}

// ParseRegionAlgorithm normalizes user input into a supported region aggregation algorithm.
func ParseRegionAlgorithm(value string) (RegionAlgorithm, error) {
	switch value {
	case "", "concave_hull", "concave-hull", "concave":
		return RegionAlgorithmConcaveHull, nil
	case "adjacency", "components", "connected_components":
		return RegionAlgorithmAdjacency, nil
	}

	return "", &UnsupportedAlgorithmError{Value: value}
}

// UnsupportedAlgorithmError indicates an unknown region algorithm value.
type UnsupportedAlgorithmError struct {
	Value string
}

func (e *UnsupportedAlgorithmError) Error() string {
	return "unknown region-algorithm: " + e.Value
}

// destinationByID returns the Destination with the given ID, or nil.
func (c *Calculator) destinationByID(id string) *Destination {
	for i := range c.Destinations {
		if c.Destinations[i].ID == id {
			return &c.Destinations[i]
		}
	}
	return nil
}

// defaultCtx returns the provided context, or context.Background() if nil.
func defaultCtx(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
