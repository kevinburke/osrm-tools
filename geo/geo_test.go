package geo

import (
	"math"
	"testing"
)

func TestHaversineDistance(t *testing.T) {
	// San Francisco to Los Angeles (~559 km)
	sf := Point{Lat: 37.7749, Lon: -122.4194}
	la := Point{Lat: 34.0522, Lon: -118.2437}

	dist := HaversineDistance(sf, la)
	// Should be approximately 559 km
	if dist < 550000 || dist > 570000 {
		t.Errorf("HaversineDistance(SF, LA) = %.0f meters, expected ~559000", dist)
	}

	// Same point should be 0
	if d := HaversineDistance(sf, sf); d != 0 {
		t.Errorf("HaversineDistance(same, same) = %f, expected 0", d)
	}
}

func TestMetersPerDegreeLongitude(t *testing.T) {
	// At the equator, should be close to MetersPerDegreeLatitude
	equator := MetersPerDegreeLongitude(0)
	if math.Abs(equator-MetersPerDegreeLatitude) > 1 {
		t.Errorf("MetersPerDegreeLongitude(0) = %.2f, expected ~%.2f", equator, MetersPerDegreeLatitude)
	}

	// At 37°N (San Francisco area), cos(37°) ≈ 0.7986
	sf := MetersPerDegreeLongitude(37)
	expected := MetersPerDegreeLatitude * math.Cos(37*math.Pi/180)
	if math.Abs(sf-expected) > 0.01 {
		t.Errorf("MetersPerDegreeLongitude(37) = %.2f, expected %.2f", sf, expected)
	}

	// At 90°N (pole), should be approximately 0
	pole := MetersPerDegreeLongitude(90)
	if math.Abs(pole) > 0.01 {
		t.Errorf("MetersPerDegreeLongitude(90) = %.2f, expected ~0", pole)
	}
}

func TestBoundingBox(t *testing.T) {
	points := []Point{
		{Lat: 37.0, Lon: -122.5},
		{Lat: 38.0, Lon: -121.5},
		{Lat: 37.5, Lon: -122.0},
	}

	sw, ne := BoundingBox(points)

	if sw.Lat != 37.0 || sw.Lon != -122.5 {
		t.Errorf("BoundingBox SW = %+v, expected {37.0, -122.5}", sw)
	}
	if ne.Lat != 38.0 || ne.Lon != -121.5 {
		t.Errorf("BoundingBox NE = %+v, expected {38.0, -121.5}", ne)
	}

	// Empty
	sw, ne = BoundingBox(nil)
	if sw != (Point{}) || ne != (Point{}) {
		t.Errorf("BoundingBox(nil) should return zero points")
	}
}

func TestParseGoogleMapCoords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Point
	}{
		{
			name:     "basic coordinates",
			input:    "37.880040, -122.049138",
			expected: Point{Lat: 37.880040, Lon: -122.049138},
		},
		{
			name:     "high precision coordinates get trimmed",
			input:    "37.88516736957163, -122.0501677466162",
			expected: Point{Lat: 37.885167, Lon: -122.050168},
		},
		{
			name:     "coordinates with extra spaces",
			input:    "  37.849508  ,  -122.034477  ",
			expected: Point{Lat: 37.849508, Lon: -122.034477},
		},
		{
			name:     "zero coordinates",
			input:    "0.0, 0.0",
			expected: Point{Lat: 0.0, Lon: 0.0},
		},
		{
			name:     "edge case coordinates",
			input:    "89.999999, 179.999999",
			expected: Point{Lat: 90.0, Lon: 180.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseGoogleMapCoords(tt.input)

			if math.Abs(result.Lat-tt.expected.Lat) > 0.000001 {
				t.Errorf("ParseGoogleMapCoords() lat = %v, expected %v", result.Lat, tt.expected.Lat)
			}
			if math.Abs(result.Lon-tt.expected.Lon) > 0.000001 {
				t.Errorf("ParseGoogleMapCoords() lon = %v, expected %v", result.Lon, tt.expected.Lon)
			}
		})
	}
}

func TestParseGoogleMapCoordsPanics(t *testing.T) {
	panicTests := []struct {
		name  string
		input string
	}{
		{name: "missing comma", input: "37.880040 -122.049138"},
		{name: "too many parts", input: "37.880040, -122.049138, 100.0"},
		{name: "invalid latitude", input: "abc, -122.049138"},
		{name: "invalid longitude", input: "37.880040, xyz"},
		{name: "latitude too high", input: "91.0, -122.049138"},
		{name: "latitude too low", input: "-91.0, -122.049138"},
		{name: "longitude too high", input: "37.880040, 181.0"},
		{name: "longitude too low", input: "37.880040, -181.0"},
		{name: "empty input", input: ""},
	}

	for _, tt := range panicTests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("ParseGoogleMapCoords() should have panicked for input %q", tt.input)
				}
			}()
			ParseGoogleMapCoords(tt.input)
		})
	}
}

func TestParseGoogleMapCoordsPrecisionTrimming(t *testing.T) {
	input := "37.123456789012345, -122.987654321098765"
	result := ParseGoogleMapCoords(input)

	expectedLat := 37.123457
	expectedLon := -122.987654

	if math.Abs(result.Lat-expectedLat) > 0.000001 {
		t.Errorf("lat precision trimming failed: got %v, expected %v", result.Lat, expectedLat)
	}
	if math.Abs(result.Lon-expectedLon) > 0.000001 {
		t.Errorf("lon precision trimming failed: got %v, expected %v", result.Lon, expectedLon)
	}
}

func TestIsPointInConvexPolygon(t *testing.T) {
	// Simple square
	square := []Point{
		{Lat: 0, Lon: 0},
		{Lat: 0, Lon: 1},
		{Lat: 1, Lon: 1},
		{Lat: 1, Lon: 0},
	}

	if !IsPointInConvexPolygon(Point{Lat: 0.5, Lon: 0.5}, square) {
		t.Error("Center of square should be inside")
	}
	if IsPointInConvexPolygon(Point{Lat: 2, Lon: 2}, square) {
		t.Error("Point outside square should not be inside")
	}
	if !IsPointInConvexPolygon(Point{Lat: 0, Lon: 0}, square) {
		t.Error("Vertex should be inside (or on boundary)")
	}
}

func TestIsPointInPolygon(t *testing.T) {
	// L-shaped polygon (concave)
	lShape := []Point{
		{Lat: 0, Lon: 0},
		{Lat: 0, Lon: 2},
		{Lat: 1, Lon: 2},
		{Lat: 1, Lon: 1},
		{Lat: 2, Lon: 1},
		{Lat: 2, Lon: 0},
	}

	if !IsPointInPolygon(Point{Lat: 0.5, Lon: 0.5}, lShape) {
		t.Error("Point in lower-left of L should be inside")
	}
	if IsPointInPolygon(Point{Lat: 1.5, Lon: 1.5}, lShape) {
		t.Error("Point in upper-right cutout of L should be outside")
	}
}
