package geo

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseGoogleMapCoords parses coordinates from Google Maps format and trims to 6 decimal places.
// Input format: "37.88516736957163, -122.0501677466162"
// Panics on invalid input.
func ParseGoogleMapCoords(input string) Point {
	point, err := ParseGoogleMapCoordsWithError(input)
	if err != nil {
		panic(err.Error())
	}
	return point
}

// ParseGoogleMapCoordsWithError is like ParseGoogleMapCoords but returns an error instead of panicking.
func ParseGoogleMapCoordsWithError(input string) (Point, error) {
	parts := strings.Split(input, ",")
	if len(parts) != 2 {
		return Point{}, fmt.Errorf("invalid coordinate format '%s': must be 'lat,lon'", input)
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return Point{}, fmt.Errorf("invalid latitude in '%s': %w", input, err)
	}

	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return Point{}, fmt.Errorf("invalid longitude in '%s': %w", input, err)
	}

	if lat < -90 || lat > 90 {
		return Point{}, fmt.Errorf("latitude %.6f out of range [-90, 90]", lat)
	}

	if lon < -180 || lon > 180 {
		return Point{}, fmt.Errorf("longitude %.6f out of range [-180, 180]", lon)
	}

	// Trim to 6 decimal places (sufficient precision for most uses)
	lat = math.Round(lat*1000000) / 1000000
	lon = math.Round(lon*1000000) / 1000000

	return Point{Lat: lat, Lon: lon}, nil
}
