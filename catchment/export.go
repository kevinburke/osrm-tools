package catchment

import (
	"log"
	"math"

	"github.com/kevinburke/osrm-tools/geo"
	"github.com/kevinburke/osrm-tools/geojson"
	"github.com/kevinburke/osrm-tools/hull"
)

// ExportToGeoJSON exports results as polygons (one per assignment), honoring the configured
// region algorithm. Returns a GeoJSON FeatureCollection as an indented JSON string.
func (c *Calculator) ExportToGeoJSON(results []GridPoint) string {
	fc := geojson.NewFeatureCollection()

	// Group by assignment
	byAssignment := make(map[string][]GridPoint)
	for _, gp := range results {
		byAssignment[gp.AssignedTo] = append(byAssignment[gp.AssignedTo], gp)
	}

	for assignment, gridPoints := range byAssignment {
		components := [][]GridPoint{gridPoints}
		if c.RegionAlgorithm == RegionAlgorithmAdjacency {
			components = c.connectedComponents(gridPoints)
		}

		for idx, component := range components {
			features := c.buildRegionFeatures(assignment, component, idx)
			fc.Add(features...)
		}
	}

	data, _ := fc.Marshal()
	return string(data)
}

// buildRegionFeatures converts a slice of grid points into GeoJSON features.
func (c *Calculator) buildRegionFeatures(assignment string, gridPoints []GridPoint, componentIdx int) []geojson.Feature {
	if len(gridPoints) == 0 {
		return nil
	}

	dest := c.destinationByID(assignment)
	color := "#808080"
	name := assignment
	if dest != nil {
		color = dest.Color
		name = dest.Name
	}

	if len(gridPoints) >= 3 {
		points := make([]geo.Point, len(gridPoints))
		for i, gp := range gridPoints {
			points[i] = gp.Point
		}

		hullPoints, err := hull.ConcaveHull(points, c.ConcaveHullRatio)
		if err == nil && len(hullPoints) >= 3 {
			ring := make([][]float64, 0, len(hullPoints)+1)
			for _, p := range hullPoints {
				ring = append(ring, []float64{p.Lon, p.Lat})
			}
			ring = append(ring, []float64{hullPoints[0].Lon, hullPoints[0].Lat})

			properties := map[string]any{
				"assignment":     assignment,
				"name":           name,
				"point_count":    len(gridPoints),
				"fill":           color,
				"fill-opacity":   0.4,
				"stroke":         color,
				"stroke-width":   2,
				"stroke-opacity": 0.8,
			}
			if c.RegionAlgorithm == RegionAlgorithmAdjacency {
				properties["component_id"] = componentIdx + 1
			}

			// Compute average travel times for each destination
			for _, d := range c.Destinations {
				var sum float64
				var count int
				for _, gp := range gridPoints {
					if t, ok := gp.TravelTimes[d.ID]; ok {
						sum += t
						count++
					}
				}
				if count > 0 {
					properties["avg_time_to_"+d.ID+"_min"] = math.Round(sum/float64(count)/60*10) / 10
				}
			}

			return []geojson.Feature{geojson.NewPolygonFeature(ring, properties)}
		}

		if err != nil {
			log.Printf("Warning: Failed to compute concave hull for assignment %s: %v", assignment, err)
		}
	}

	// Fallback: individual points
	features := make([]geojson.Feature, 0, len(gridPoints))
	for _, gp := range gridPoints {
		properties := map[string]any{
			"assignment":   gp.AssignedTo,
			"name":         name,
			"marker-color": color,
			"marker-size":  "medium",
		}
		for _, d := range c.Destinations {
			if t, ok := gp.TravelTimes[d.ID]; ok {
				properties["time_to_"+d.ID+"_min"] = math.Round(t/60*10) / 10
			}
		}
		if c.RegionAlgorithm == RegionAlgorithmAdjacency {
			properties["component_id"] = componentIdx + 1
		}

		f := geojson.NewPointFeature(gp.Lon, gp.Lat, properties)
		features = append(features, f)
	}

	return features
}

// gridIndex represents a grid cell coordinate.
type gridIndex struct {
	row int
	col int
}

// pointToGridIndex converts a point to its grid index.
func (c *Calculator) pointToGridIndex(p geo.Point) gridIndex {
	if c.GridSpacing == 0 {
		return gridIndex{}
	}

	row := int(math.Round((p.Lat - c.BoundsMin.Lat) / c.GridSpacing))
	col := int(math.Round((p.Lon - c.BoundsMin.Lon) / c.GridSpacing))
	return gridIndex{row: row, col: col}
}

// connectedComponents finds connected groups of grid points using 8-way adjacency.
func (c *Calculator) connectedComponents(points []GridPoint) [][]GridPoint {
	if len(points) == 0 {
		return nil
	}

	indices := make([]gridIndex, len(points))
	indexMap := make(map[gridIndex][]int)
	for i, point := range points {
		idx := c.pointToGridIndex(point.Point)
		indices[i] = idx
		indexMap[idx] = append(indexMap[idx], i)
	}

	visited := make([]bool, len(points))
	var components [][]GridPoint
	neighbors := []gridIndex{
		{row: 1, col: 0}, {row: -1, col: 0}, {row: 0, col: 1}, {row: 0, col: -1},
		{row: 1, col: 1}, {row: 1, col: -1}, {row: -1, col: 1}, {row: -1, col: -1},
	}

	for i := range points {
		if visited[i] {
			continue
		}

		queue := []int{i}
		visited[i] = true
		var component []GridPoint

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			component = append(component, points[current])
			currentIndex := indices[current]

			for _, offset := range neighbors {
				neighborIndex := gridIndex{row: currentIndex.row + offset.row, col: currentIndex.col + offset.col}
				if neighborPoints, ok := indexMap[neighborIndex]; ok {
					for _, neighbor := range neighborPoints {
						if !visited[neighbor] {
							visited[neighbor] = true
							queue = append(queue, neighbor)
						}
					}
				}
			}
		}

		components = append(components, component)
	}

	return components
}
