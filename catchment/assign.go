package catchment

import (
	"context"
	"errors"

	"github.com/kevinburke/osrm-tools/geo"
	"github.com/kevinburke/osrm-tools/osrm"
)

// CalculateCatchment processes all grid points and assigns each to the closest destination.
func (c *Calculator) CalculateCatchment(ctx context.Context, grid []geo.Point) ([]GridPoint, error) {
	ctx = defaultCtx(ctx)
	results := make([]GridPoint, 0, len(grid))

	c.Logger.Info("Processing grid points", "count", len(grid), "destinations", len(c.Destinations))

	for i, point := range grid {
		gp := GridPoint{
			Point:       point,
			TravelTimes: make(map[string]float64, len(c.Destinations)),
		}
		if c.DebugMode {
			gp.RouteGeometries = make(map[string]any, len(c.Destinations))
		}

		allFailed := true
		for _, dest := range c.Destinations {
			route, err := c.OSRMClient.GetRouteWithGeometry(ctx, c.Profile, point, dest.Point)
			if err != nil {
				var critErr *osrm.CriticalError
				if errors.As(err, &critErr) {
					return results, critErr
				}
				c.Logger.Warn("Could not route to destination, skipping",
					"lat", point.Lat, "lon", point.Lon,
					"dest", dest.ID, "error", err)
				continue
			}

			duration := route.Routes[0].Duration
			gp.TravelTimes[dest.ID] = duration
			allFailed = false

			if c.DebugMode {
				gp.RouteGeometries[dest.ID] = route.Routes[0].Geometry
			}
		}

		if allFailed {
			continue
		}

		// Find the closest destination
		first := true
		for destID, duration := range gp.TravelTimes {
			if first || duration < gp.MinTime {
				gp.AssignedTo = destID
				gp.MinTime = duration
				first = false
			}
		}

		results = append(results, gp)

		if (i+1)%10 == 0 {
			c.Logger.Info("Processing progress", "done", i+1, "total", len(grid))
		}
	}

	return results, nil
}
