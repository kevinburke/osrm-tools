// Package hull computes concave hulls using the GEOS library (via CGO).
// This package is separated to isolate the CGO/GEOS dependency. Users without
// GEOS installed can still use the rest of the osrm-tools library.
package hull

import (
	"fmt"

	"github.com/kevinburke/osrm-tools/geo"
	"github.com/twpayne/go-geos"
)

// ConcaveHull computes a concave hull around the given points.
//
// ratio controls the concavity:
//   - 1.0 = convex hull (no concavity)
//   - 0.0 = most concave polygon that stays connected (tightest fit)
//
// Returns the hull as a slice of points. Requires at least 3 input points.
func ConcaveHull(points []geo.Point, ratio float64) ([]geo.Point, error) {
	if len(points) < 3 {
		return points, nil
	}

	geoms := make([]*geos.Geom, len(points))
	for i, p := range points {
		geoms[i] = geos.NewPointFromXY(p.Lon, p.Lat)
		if geoms[i] == nil {
			return nil, fmt.Errorf("failed to create point geometry for point %d", i)
		}
	}

	mp := geos.NewCollection(geos.TypeIDMultiPoint, geoms)
	if mp == nil {
		return nil, fmt.Errorf("failed to create multipoint collection")
	}

	hullGeom := mp.ConcaveHull(ratio, 0) // allowHoles=0
	if hullGeom == nil {
		return nil, fmt.Errorf("failed to compute concave hull")
	}

	return geosGeometryToPoints(hullGeom)
}

// pointsToPolygon creates a closed GEOS polygon from a slice of points.
func pointsToPolygon(points []geo.Point) (*geos.Geom, error) {
	n := len(points)
	ring := make([][]float64, n+1)
	for i, p := range points {
		ring[i] = []float64{p.Lon, p.Lat}
	}
	ring[n] = ring[0] // close the ring
	poly := geos.NewPolygon([][][]float64{ring})
	if poly == nil {
		return nil, fmt.Errorf("failed to create polygon from %d points", n)
	}
	return poly, nil
}

// IntersectPolygons computes the intersection of two polygons and returns the
// result as a slice of points (the exterior ring). If the intersection is empty,
// it returns nil with no error.
func IntersectPolygons(a, b []geo.Point) ([]geo.Point, error) {
	polyA, err := pointsToPolygon(a)
	if err != nil {
		return nil, fmt.Errorf("polygon A: %w", err)
	}
	polyB, err := pointsToPolygon(b)
	if err != nil {
		return nil, fmt.Errorf("polygon B: %w", err)
	}
	result := polyA.Intersection(polyB)
	if result == nil || result.IsEmpty() {
		return nil, nil
	}
	return geosGeometryToPoints(result)
}

// geosGeometryToPoints converts a GEOS geometry to a slice of Points.
func geosGeometryToPoints(geom *geos.Geom) ([]geo.Point, error) {
	geomType := geom.TypeID()
	var points []geo.Point

	switch geomType {
	case geos.TypeIDPolygon:
		ring := geom.ExteriorRing()
		if ring == nil {
			return nil, fmt.Errorf("failed to get exterior ring")
		}

		coords := ring.CoordSeq()
		if coords == nil {
			return nil, fmt.Errorf("failed to get coordinates from ring")
		}

		size := coords.Size()
		for i := range size - 1 { // -1 to exclude closing point
			x := coords.X(i)
			y := coords.Y(i)
			points = append(points, geo.Point{Lat: y, Lon: x})
		}

	case geos.TypeIDLineString:
		coords := geom.CoordSeq()
		if coords == nil {
			return nil, fmt.Errorf("failed to get coordinates from linestring")
		}

		size := coords.Size()
		for i := range size {
			x := coords.X(i)
			y := coords.Y(i)
			points = append(points, geo.Point{Lat: y, Lon: x})
		}

	case geos.TypeIDMultiPolygon, geos.TypeIDGeometryCollection:
		n := geom.NumGeometries()
		if n == 0 {
			return nil, nil
		}
		// Use the largest sub-polygon by area.
		var bestGeom *geos.Geom
		var bestArea float64
		for i := range n {
			g := geom.Geometry(i)
			if g == nil || g.TypeID() != geos.TypeIDPolygon {
				continue
			}
			a := g.Area()
			if bestGeom == nil || a > bestArea {
				bestGeom = g
				bestArea = a
			}
		}
		if bestGeom == nil {
			return nil, nil
		}
		return geosGeometryToPoints(bestGeom)

	default:
		return nil, fmt.Errorf("unsupported geometry type: %v", geomType)
	}

	return points, nil
}
