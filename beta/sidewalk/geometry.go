package sidewalk

import "math"

const (
	earthRadius     = 6371000.0 // meters
	deg2rad         = math.Pi / 180
	rad2deg         = 180 / math.Pi
	metersPerDegLat = 111320.0
)

// metersPerDegLon returns meters per degree of longitude at the given latitude.
func metersPerDegLon(latDeg float64) float64 {
	return metersPerDegLat * math.Cos(latDeg*deg2rad)
}

// Bearing returns the initial bearing from point a to point b in degrees [0, 360).
// Points are [lon, lat].
func Bearing(a, b [2]float64) float64 {
	dLon := (b[0] - a[0]) * deg2rad
	lat1 := a[1] * deg2rad
	lat2 := b[1] * deg2rad

	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)
	brng := math.Atan2(y, x) * rad2deg
	return math.Mod(brng+360, 360)
}

// BearingsParallel returns true if the two bearings (in degrees) are within
// threshold degrees of being parallel (accounting for ±180° flip).
func BearingsParallel(b1, b2, threshold float64) bool {
	diff := math.Abs(b1 - b2)
	diff = math.Mod(diff, 360)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff <= threshold || (180-diff) <= threshold
}

// SegmentSide determines which side of the directed road segment A→B the point
// lies on. Returns "left" or "right" per OSM convention (standing at A, looking
// toward B, left is to your left).
//
// Uses the 2D cross product: (B-A) × (S-A). Positive = left, negative = right.
// The latitude-scaling distortion affects magnitude but not sign.
func SegmentSide(roadA, roadB, point [2]float64) string {
	cross := (roadB[0]-roadA[0])*(point[1]-roadA[1]) -
		(roadB[1]-roadA[1])*(point[0]-roadA[0])
	if cross >= 0 {
		return "left"
	}
	return "right"
}

// PointToSegmentDistance returns the minimum distance in meters from point p
// to the line segment a–b. All points are [lon, lat].
func PointToSegmentDistance(p, a, b [2]float64) float64 {
	// Work in a local meter-scale coordinate system centered on a.
	midLat := (a[1] + b[1]) / 2
	mLon := metersPerDegLon(midLat)
	mLat := metersPerDegLat

	ax, ay := 0.0, 0.0
	bx, by := (b[0]-a[0])*mLon, (b[1]-a[1])*mLat
	px, py := (p[0]-a[0])*mLon, (p[1]-a[1])*mLat

	dx, dy := bx-ax, by-ay
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		// a and b are the same point.
		return math.Sqrt(px*px + py*py)
	}

	// Parameter t of the projection of p onto the line through a–b.
	t := ((px-ax)*dx + (py-ay)*dy) / lenSq
	t = math.Max(0, math.Min(1, t))

	closestX := ax + t*dx
	closestY := ay + t*dy
	ex := px - closestX
	ey := py - closestY
	return math.Sqrt(ex*ex + ey*ey)
}

// SegmentDistance returns the minimum distance in meters between two line
// segments a1–a2 and b1–b2. All points are [lon, lat].
func SegmentDistance(a1, a2, b1, b2 [2]float64) float64 {
	// Check each endpoint against the other segment.
	d := PointToSegmentDistance(a1, b1, b2)
	if d2 := PointToSegmentDistance(a2, b1, b2); d2 < d {
		d = d2
	}
	if d2 := PointToSegmentDistance(b1, a1, a2); d2 < d {
		d = d2
	}
	if d2 := PointToSegmentDistance(b2, a1, a2); d2 < d {
		d = d2
	}

	// Also check for actual segment intersection (distance = 0).
	if segmentsIntersect(a1, a2, b1, b2) {
		return 0
	}
	return d
}

// segmentsIntersect returns true if segments p1–p2 and p3–p4 intersect.
func segmentsIntersect(p1, p2, p3, p4 [2]float64) bool {
	d1 := crossSign(p3, p4, p1)
	d2 := crossSign(p3, p4, p2)
	d3 := crossSign(p1, p2, p3)
	d4 := crossSign(p1, p2, p4)

	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}

	if d1 == 0 && onSegment(p3, p4, p1) {
		return true
	}
	if d2 == 0 && onSegment(p3, p4, p2) {
		return true
	}
	if d3 == 0 && onSegment(p1, p2, p3) {
		return true
	}
	if d4 == 0 && onSegment(p1, p2, p4) {
		return true
	}
	return false
}

// crossSign returns the sign of the cross product (b-a) × (c-a).
func crossSign(a, b, c [2]float64) float64 {
	return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])
}

// onSegment checks if point p is on segment a–b, assuming collinearity.
func onSegment(a, b, p [2]float64) bool {
	return math.Min(a[0], b[0]) <= p[0] && p[0] <= math.Max(a[0], b[0]) &&
		math.Min(a[1], b[1]) <= p[1] && p[1] <= math.Max(a[1], b[1])
}

// SegmentIntersectsPolygon returns true if the line segment a–b intersects the
// polygon (given as a closed ring where first == last, or unclosed).
func SegmentIntersectsPolygon(a, b [2]float64, polygon [][2]float64) bool {
	n := len(polygon)
	if n < 3 {
		return false
	}

	// Check if either endpoint is inside the polygon.
	if pointInPolygon(a, polygon) || pointInPolygon(b, polygon) {
		return true
	}

	// Check if the segment intersects any polygon edge.
	for i := range n {
		j := (i + 1) % n
		if segmentsIntersect(a, b, polygon[i], polygon[j]) {
			return true
		}
	}
	return false
}

// pointInPolygon uses ray casting to test if p is inside the polygon.
func pointInPolygon(p [2]float64, polygon [][2]float64) bool {
	n := len(polygon)
	inside := false
	j := n - 1
	for i := range n {
		xi, yi := polygon[i][0], polygon[i][1]
		xj, yj := polygon[j][0], polygon[j][1]
		if ((yi > p[1]) != (yj > p[1])) &&
			(p[0] < (xj-xi)*(p[1]-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}

// segmentLength returns the length of a segment in meters.
func segmentLength(a, b [2]float64) float64 {
	midLat := (a[1] + b[1]) / 2
	dx := (b[0] - a[0]) * metersPerDegLon(midLat)
	dy := (b[1] - a[1]) * metersPerDegLat
	return math.Sqrt(dx*dx + dy*dy)
}

// segmentMidpoint returns the midpoint of a segment.
func segmentMidpoint(a, b [2]float64) [2]float64 {
	return [2]float64{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2}
}
