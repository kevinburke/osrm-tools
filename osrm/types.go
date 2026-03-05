package osrm

import (
	"encoding/json"
	"fmt"
)

// FlexibleInt64 handles JSON numbers that might come as floats or in scientific notation.
// OSRM sometimes returns node IDs like 12812510950.0 which cannot be unmarshaled into int64.
type FlexibleInt64 int64

// UnmarshalJSON implements json.Unmarshaler to handle various number formats.
func (f *FlexibleInt64) UnmarshalJSON(data []byte) error {
	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return err
	}

	// Try to parse as float64 first (handles scientific notation and .0 suffixes)
	floatVal, err := num.Float64()
	if err != nil {
		return fmt.Errorf("failed to parse number %s: %w", string(data), err)
	}

	*f = FlexibleInt64(int64(floatVal))
	return nil
}

// Int64 returns the int64 value.
func (f FlexibleInt64) Int64() int64 {
	return int64(f)
}

// RouteResponse represents the OSRM route API response.
type RouteResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Routes  []struct {
		Duration float64 `json:"duration"` // in seconds
		Distance float64 `json:"distance"` // in meters
		Geometry any     `json:"geometry"` // GeoJSON object when geometries=geojson
		Legs     []struct {
			Steps []struct {
				Duration    float64 `json:"duration"`
				Distance    float64 `json:"distance"`
				Name        string  `json:"name"`
				Instruction string  `json:"instruction"`
				Maneuver    any     `json:"maneuver"`
			} `json:"steps"`
			Duration   float64 `json:"duration"`
			Distance   float64 `json:"distance"`
			Summary    string  `json:"summary"`
			Annotation *struct {
				Datasources []int           `json:"datasources"`
				Durations   []float64       `json:"durations"`
				Distances   []float64       `json:"distances"`
				Weight      []float64       `json:"weight"`
				Nodes       []FlexibleInt64 `json:"nodes"`
				Ways        []FlexibleInt64 `json:"ways"`
			} `json:"annotation,omitempty"`
		} `json:"legs"`
	} `json:"routes"`
}

// NearestResponse represents OSRM's nearest service response.
type NearestResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Waypoints []struct {
		Location []float64       `json:"location"` // [lon, lat]
		Distance float64         `json:"distance"` // distance in meters to nearest road
		Name     string          `json:"name"`
		Hint     string          `json:"hint,omitempty"`
		Nodes    []FlexibleInt64 `json:"nodes,omitempty"`
		Classes  []string        `json:"classes,omitempty"`
	} `json:"waypoints"`
}
