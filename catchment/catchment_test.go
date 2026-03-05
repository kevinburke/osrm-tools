package catchment

import (
	"testing"

	"github.com/kevinburke/osrm-tools/geo"
)

func TestParseRegionAlgorithm(t *testing.T) {
	tests := []struct {
		input    string
		expected RegionAlgorithm
		wantErr  bool
	}{
		{"", RegionAlgorithmConcaveHull, false},
		{"concave_hull", RegionAlgorithmConcaveHull, false},
		{"concave-hull", RegionAlgorithmConcaveHull, false},
		{"concave", RegionAlgorithmConcaveHull, false},
		{"adjacency", RegionAlgorithmAdjacency, false},
		{"components", RegionAlgorithmAdjacency, false},
		{"connected_components", RegionAlgorithmAdjacency, false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRegionAlgorithm(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRegionAlgorithm(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseRegionAlgorithm(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNewCalculator(t *testing.T) {
	dests := []Destination{
		{ID: "A", Name: "School A", Color: "#FF0000"},
		{ID: "B", Name: "School B", Color: "#0000FF"},
	}

	// We pass nil for osrmClient since we're just testing struct construction
	c := NewCalculator(dests, dummyPoint(37.0, -122.0), dummyPoint(38.0, -121.0), 0.001, nil)

	if len(c.Destinations) != 2 {
		t.Errorf("expected 2 destinations, got %d", len(c.Destinations))
	}
	if c.Profile != "cycling" {
		t.Errorf("expected default profile 'cycling', got %q", c.Profile)
	}
	if c.ConcaveHullRatio != 0.05 {
		t.Errorf("expected default hull ratio 0.05, got %f", c.ConcaveHullRatio)
	}
}

func dummyPoint(lat, lon float64) geo.Point {
	return geo.Point{Lat: lat, Lon: lon}
}
