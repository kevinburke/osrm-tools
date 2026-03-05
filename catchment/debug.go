package catchment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"

	"github.com/kevinburke/osrm-tools/geo"
	"github.com/kevinburke/osrm-tools/geojson"
	"github.com/kevinburke/osrm-tools/osrm"
)

// SampleRoute stores a sample route for debugging.
type SampleRoute struct {
	From             geo.Point
	To               string // Destination ID
	ToPoint          geo.Point
	Duration         float64
	Distance         float64
	AlternativeIndex int
	IsPrimary        bool
	Geometry         any
}

// RunSampleDebug runs a small sample with full route visualization.
func (c *Calculator) RunSampleDebug(ctx context.Context, grid []geo.Point, numSamples int, overridePoints []geo.Point) error {
	ctx = defaultCtx(ctx)

	samples := make([]geo.Point, 0, numSamples)
	if len(overridePoints) > 0 {
		samples = append(samples, overridePoints...)
	}

	if numSamples > len(samples) {
		additional := getRandomSamplePoints(grid, numSamples-len(samples))
		samples = append(samples, additional...)
	} else if numSamples > 0 && numSamples < len(samples) {
		samples = samples[:numSamples]
	}

	if len(samples) == 0 {
		return fmt.Errorf("no debug samples available")
	}

	c.Logger.Info("Running debug sample", "count", len(samples))

	var sampleRoutes []SampleRoute
	var gridPoints []GridPoint

	for i, point := range samples {
		c.Logger.Info("Processing debug sample", "index", i+1, "total", len(samples),
			"lat", point.Lat, "lon", point.Lon)

		gp := GridPoint{
			Point:       point,
			TravelTimes: make(map[string]float64, len(c.Destinations)),
		}
		allFailed := true

		for _, dest := range c.Destinations {
			route, err := c.OSRMClient.GetRouteWithGeometry(ctx, c.Profile, point, dest.Point)
			if err != nil {
				var critErr *osrm.CriticalError
				if errors.As(err, &critErr) {
					return critErr
				}
				c.Logger.Warn("Could not route to destination",
					"dest", dest.ID, "error", err)
				continue
			}

			duration := route.Routes[0].Duration
			gp.TravelTimes[dest.ID] = duration
			allFailed = false

			// Store all route alternatives for visualization
			for altIndex, routeOption := range route.Routes {
				sampleRoutes = append(sampleRoutes, SampleRoute{
					From:             point,
					To:               dest.ID,
					ToPoint:          dest.Point,
					Duration:         routeOption.Duration,
					Distance:         routeOption.Distance,
					AlternativeIndex: altIndex,
					IsPrimary:        altIndex == 0,
					Geometry:         routeOption.Geometry,
				})
			}

			c.Logger.Info("Route result",
				"dest", dest.ID,
				"duration_min", fmt.Sprintf("%.1f", duration/60),
				"distance_km", fmt.Sprintf("%.1f", route.Routes[0].Distance/1000),
			)
		}

		if allFailed {
			continue
		}

		// Find closest
		first := true
		for destID, duration := range gp.TravelTimes {
			if first || duration < gp.MinTime {
				gp.AssignedTo = destID
				gp.MinTime = duration
				first = false
			}
		}

		gridPoints = append(gridPoints, gp)
	}

	return c.ExportDebugVisualization(sampleRoutes, gridPoints)
}

// ExportDebugVisualization exports debug routes and points as separate GeoJSON files.
func (c *Calculator) ExportDebugVisualization(routes []SampleRoute, points []GridPoint) error {
	// Routes GeoJSON
	fc := geojson.NewFeatureCollection()

	for _, route := range routes {
		dest := c.destinationByID(route.To)
		color := "#808080"
		altColor := "#C0C0C0"
		if dest != nil {
			color = dest.Color
			altColor = lightenColor(dest.Color)
		}

		chosenColor := color
		strokeOpacity := 0.7
		if !route.IsPrimary {
			chosenColor = altColor
			strokeOpacity = 0.5
		}

		properties := map[string]any{
			"from_lat":          route.From.Lat,
			"from_lon":          route.From.Lon,
			"to":                route.To,
			"duration_min":      route.Duration / 60,
			"distance_km":       route.Distance / 1000,
			"alternative_index": route.AlternativeIndex,
			"is_primary":        route.IsPrimary,
			"stroke":            chosenColor,
			"stroke-width":      2,
			"stroke-opacity":    strokeOpacity,
		}

		// Use raw geometry from OSRM
		feature := geojson.Feature{
			Type:       "Feature",
			Geometry:   nil,
			Properties: properties,
		}
		if geomMap, ok := route.Geometry.(map[string]any); ok {
			feature.Geometry = geomMap
		}
		fc.Add(feature)
	}

	// Add destination markers
	for _, dest := range c.Destinations {
		fc.Add(geojson.NewPointFeature(dest.Point.Lon, dest.Point.Lat, map[string]any{
			"name":          dest.Name,
			"marker-color":  dest.Color,
			"marker-size":   "large",
			"marker-symbol": dest.ID,
		}))
	}

	routesJSON, _ := fc.Marshal()
	if err := os.WriteFile("debug_routes.geojson", routesJSON, 0644); err != nil {
		return err
	}

	// Points GeoJSON
	pointsJSON := c.ExportToGeoJSON(points)
	if err := os.WriteFile("debug_points.geojson", []byte(pointsJSON), 0644); err != nil {
		return err
	}

	// Summary
	summary := map[string]any{
		"total_samples": len(points),
		"destinations":  c.Destinations,
	}
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	return os.WriteFile("debug_summary.json", summaryJSON, 0644)
}

func getRandomSamplePoints(grid []geo.Point, n int) []geo.Point {
	if len(grid) == 0 {
		return nil
	}

	shuffled := make([]geo.Point, len(grid))
	copy(shuffled, grid)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	if n >= len(shuffled) {
		return shuffled
	}
	return shuffled[:n]
}

// lightenColor is a simple helper that returns a lighter variant of a hex color.
func lightenColor(hex string) string {
	// Simple mapping for common colors
	switch hex {
	case "#FF0000":
		return "#FFA07A"
	case "#0000FF":
		return "#87CEFA"
	case "#00FF00":
		return "#90EE90"
	case "#FF00FF":
		return "#DDA0DD"
	default:
		return "#C0C0C0"
	}
}
