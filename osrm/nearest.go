package osrm

import (
	"context"
	"fmt"
	"strings"

	"github.com/kevinburke/osrm-tools/geo"
)

// IsAcceptableRoadType checks if a road name indicates a paved, bike-suitable surface.
func IsAcceptableRoadType(name string) bool {
	if name == "" {
		return false // Accept unnamed roads (often regular streets)
	}

	lower := strings.ToLower(name)

	// Reject these trail types (often unpaved) - check first (more specific)
	rejectTypes := []string{
		"trail", "track", "fire road", "hiking", "nature", "wilderness",
		"dirt", "gravel", "unpaved", "bridle", "horse", "line", "path",
	}
	for _, reject := range rejectTypes {
		if strings.Contains(lower, reject) {
			return false
		}
	}

	// Accept these road types (paved, bike-suitable)
	acceptableTypes := []string{
		"street", "road", "avenue", "boulevard", "drive", "lane", "way", "court",
		"circle", "place", "terrace", "highway", "route", "cycleway",
		"bike", "cycle", "footway", "sidewalk", "pedestrian",
	}
	for _, accept := range acceptableTypes {
		if strings.Contains(lower, accept) {
			return true
		}
	}

	// Default: accept if no clear indicators (many roads have generic names)
	return true
}

// IsPointNearRoad checks if a point is within maxDistance meters of a suitable road using
// OSRM's nearest service. Returns (isNear, distance, error) where distance is in meters.
func (c *Client) IsPointNearRoad(ctx context.Context, profile string, point geo.Point, maxDistance float64) (bool, float64, error) {
	if maxDistance <= 0 {
		return true, 0, nil // Skip road proximity check if disabled
	}

	nearestResp, err := c.GetNearest(ctx, profile, point, 5)
	if err != nil {
		return false, -1, err
	}

	if nearestResp.Code != "Ok" || len(nearestResp.Waypoints) == 0 {
		return false, -1, fmt.Errorf("no nearest road found (code=%s message=%s)", nearestResp.Code, nearestResp.Message)
	}

	// Filter waypoints to find acceptable road types within distance
	var closestDistance float64 = -1
	found := false

	for _, waypoint := range nearestResp.Waypoints {
		if waypoint.Distance <= maxDistance && IsAcceptableRoadType(waypoint.Name) {
			if !found || waypoint.Distance < closestDistance {
				closestDistance = waypoint.Distance
				found = true
			}
		}
	}

	if !found {
		return false, nearestResp.Waypoints[0].Distance, nil
	}

	return true, closestDistance, nil
}
