package hull

import (
	"testing"

	"github.com/kevinburke/osrm-tools/geo"
)

func TestConcaveHullTooFewPoints(t *testing.T) {
	points := []geo.Point{
		{Lat: 0, Lon: 0},
		{Lat: 1, Lon: 1},
	}

	result, err := ConcaveHull(points, 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 points returned for < 3 input, got %d", len(result))
	}
}

func TestConcaveHullSquare(t *testing.T) {
	// A simple square - the convex hull (ratio=1.0) should return 4 vertices
	points := []geo.Point{
		{Lat: 0, Lon: 0},
		{Lat: 0, Lon: 1},
		{Lat: 1, Lon: 1},
		{Lat: 1, Lon: 0},
		{Lat: 0.5, Lon: 0.5}, // interior point
	}

	result, err := ConcaveHull(points, 1.0)
	if err != nil {
		t.Fatalf("ConcaveHull failed: %v", err)
	}

	if len(result) < 3 {
		t.Errorf("expected at least 3 hull vertices, got %d", len(result))
	}
}
