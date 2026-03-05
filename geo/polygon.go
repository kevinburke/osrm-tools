package geo

// IsPointInConvexPolygon checks if a point is inside a convex polygon using cross products.
// The polygon vertices should be ordered (either CW or CCW). Returns false if the polygon
// has fewer than 3 vertices.
func IsPointInConvexPolygon(point Point, polygon []Point) bool {
	if len(polygon) < 3 {
		return false
	}

	n := len(polygon)
	positive := false
	negative := false

	for i := range n {
		p1 := polygon[i]
		p2 := polygon[(i+1)%n]

		// Calculate cross product of vectors (p2-p1) x (point-p1)
		crossProduct := (p2.Lon-p1.Lon)*(point.Lat-p1.Lat) - (p2.Lat-p1.Lat)*(point.Lon-p1.Lon)

		if crossProduct > 0 {
			positive = true
		} else if crossProduct < 0 {
			negative = true
		}

		// If we have both positive and negative, point is outside convex polygon
		if positive && negative {
			return false
		}
	}

	return true // All cross products have same sign (or zero)
}

// IsPointInPolygon uses the ray casting algorithm to determine if a point is inside
// an arbitrary polygon (convex or concave).
func IsPointInPolygon(point Point, polygon []Point) bool {
	n := len(polygon)
	inside := false

	j := n - 1
	for i := range n {
		xi, yi := polygon[i].Lon, polygon[i].Lat
		xj, yj := polygon[j].Lon, polygon[j].Lat

		intersect := ((yi > point.Lat) != (yj > point.Lat)) &&
			(point.Lon < (xj-xi)*(point.Lat-yi)/(yj-yi)+xi)
		if intersect {
			inside = !inside
		}
		j = i
	}

	return inside
}
