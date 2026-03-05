package catchment

import (
	"context"
	"errors"
	"log/slog"

	"github.com/kevinburke/osrm-tools/geo"
	"github.com/kevinburke/osrm-tools/osrm"
)

// GenerateGrid creates a grid of sample points within the calculator's bounds,
// filtering out points that are far from roads.
func (c *Calculator) GenerateGrid(ctx context.Context) []geo.Point {
	ctx = defaultCtx(ctx)

	var grid []geo.Point
	var roadCheckCount, nearRoadCount int

	latSteps := int((c.BoundsMax.Lat-c.BoundsMin.Lat)/c.GridSpacing + 0.5)
	lonSteps := int((c.BoundsMax.Lon-c.BoundsMin.Lon)/c.GridSpacing + 0.5)

	c.Logger.Info("Generating grid and checking road proximity",
		"max_road_distance_m", c.MaxRoadDistance,
		"lat_steps", latSteps,
		"lon_steps", lonSteps,
	)

	for i := 0; i <= latSteps; i++ {
		lat := c.BoundsMin.Lat + float64(i)*c.GridSpacing
		if lat > c.BoundsMax.Lat {
			lat = c.BoundsMax.Lat
		}

		for j := 0; j <= lonSteps; j++ {
			lon := c.BoundsMin.Lon + float64(j)*c.GridSpacing
			if lon > c.BoundsMax.Lon {
				lon = c.BoundsMax.Lon
			}

			point := geo.Point{Lat: lat, Lon: lon}

			// Check polygon bounds if defined
			if len(c.PolygonBounds) > 0 {
				if !geo.IsPointInConvexPolygon(point, c.PolygonBounds) {
					continue
				}
			}

			// Check road proximity
			if c.MaxRoadDistance > 0 {
				roadCheckCount++

				profile := c.RoadFilterProfile
				if profile == "" {
					profile = "driving"
				}

				isNear, _, err := c.OSRMClient.IsPointNearRoad(ctx, profile, point, c.MaxRoadDistance)
				if err != nil {
					var critErr *osrm.CriticalError
					if errors.As(err, &critErr) {
						c.Logger.Error("Fatal error during road proximity check", "error", err)
						return grid
					}
					c.Logger.Warn("Failed to check road proximity, skipping point",
						"lat", point.Lat, "lon", point.Lon, "error", err)
					continue
				}

				if !isNear {
					continue
				}
				nearRoadCount++

				if roadCheckCount%20 == 0 {
					c.Logger.Info("Road proximity progress",
						"checked", roadCheckCount,
						"near_roads", nearRoadCount,
						slog.Float64("pct", float64(nearRoadCount)*100/float64(roadCheckCount)),
					)
				}
			}

			grid = append(grid, point)
		}
	}

	c.Logger.Info("Grid generation complete",
		"total_checked", roadCheckCount,
		"near_roads", nearRoadCount,
		"max_distance_m", c.MaxRoadDistance,
		"final_grid_size", len(grid),
	)

	return grid
}
