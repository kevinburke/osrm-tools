// Package geo provides geographic primitives and calculations with zero external dependencies.
package geo

import "math"

// MetersPerDegreeLatitude is the approximate distance in meters per degree of latitude.
// This is effectively constant across the globe (~111.32 km).
const MetersPerDegreeLatitude = 111320.0

// MetersPerDegreeLongitude returns the distance in meters per degree of longitude at
// the given latitude. Longitude degrees shrink toward the poles because meridians converge.
func MetersPerDegreeLongitude(latDegrees float64) float64 {
	return MetersPerDegreeLatitude * math.Cos(latDegrees*math.Pi/180)
}

// Point represents a geographic coordinate (WGS84).
type Point struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// HaversineDistance calculates the great-circle distance between two points
// on the Earth's surface using the Haversine formula. Returns meters.
func HaversineDistance(p1, p2 Point) float64 {
	const R = 6371000 // Earth's radius in meters

	lat1Rad := p1.Lat * math.Pi / 180
	lat2Rad := p2.Lat * math.Pi / 180
	deltaLatRad := (p2.Lat - p1.Lat) * math.Pi / 180
	deltaLonRad := (p2.Lon - p1.Lon) * math.Pi / 180

	a := math.Sin(deltaLatRad/2)*math.Sin(deltaLatRad/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLonRad/2)*math.Sin(deltaLonRad/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// BoundingBox returns the southwest and northeast corners of the axis-aligned
// bounding box that encloses all the given points.
func BoundingBox(points []Point) (sw, ne Point) {
	if len(points) == 0 {
		return Point{}, Point{}
	}

	sw = Point{Lat: points[0].Lat, Lon: points[0].Lon}
	ne = Point{Lat: points[0].Lat, Lon: points[0].Lon}

	for _, p := range points[1:] {
		if p.Lat < sw.Lat {
			sw.Lat = p.Lat
		}
		if p.Lat > ne.Lat {
			ne.Lat = p.Lat
		}
		if p.Lon < sw.Lon {
			sw.Lon = p.Lon
		}
		if p.Lon > ne.Lon {
			ne.Lon = p.Lon
		}
	}

	return sw, ne
}
