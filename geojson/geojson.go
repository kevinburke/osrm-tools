// Package geojson provides GeoJSON types and helpers for constructing features.
package geojson

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Feature represents a single GeoJSON feature.
type Feature struct {
	Type       string         `json:"type"`
	Geometry   map[string]any `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

// FeatureCollection is a GeoJSON FeatureCollection.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

// NewFeatureCollection creates an empty FeatureCollection.
func NewFeatureCollection() *FeatureCollection {
	return &FeatureCollection{
		Type:     "FeatureCollection",
		Features: make([]Feature, 0),
	}
}

// Add appends one or more features to the collection.
func (fc *FeatureCollection) Add(features ...Feature) {
	fc.Features = append(fc.Features, features...)
}

// Marshal returns indented JSON for the collection.
func (fc *FeatureCollection) Marshal() ([]byte, error) {
	return json.MarshalIndent(fc, "", "  ")
}

// NewPointFeature creates a GeoJSON Point feature at the given lon/lat with the given properties.
func NewPointFeature(lon, lat float64, properties map[string]any) Feature {
	return Feature{
		Type: "Feature",
		Geometry: map[string]any{
			"type":        "Point",
			"coordinates": []float64{lon, lat},
		},
		Properties: properties,
	}
}

// NewPolygonFeature creates a GeoJSON Polygon feature from the given ring of [lon, lat] coordinate pairs.
// The ring should be closed (first == last) or this function will close it.
func NewPolygonFeature(ring [][]float64, properties map[string]any) Feature {
	// Close the ring if needed
	if len(ring) > 0 {
		first := ring[0]
		last := ring[len(ring)-1]
		if first[0] != last[0] || first[1] != last[1] {
			ring = append(ring, first)
		}
	}

	return Feature{
		Type: "Feature",
		Geometry: map[string]any{
			"type":        "Polygon",
			"coordinates": [][][]float64{ring},
		},
		Properties: properties,
	}
}

// NewLineStringFeature creates a GeoJSON LineString feature from [lon, lat] coordinate pairs.
func NewLineStringFeature(coords [][]float64, properties map[string]any) Feature {
	return Feature{
		Type: "Feature",
		Geometry: map[string]any{
			"type":        "LineString",
			"coordinates": coords,
		},
		Properties: properties,
	}
}

// GenerateGeoJSONioURL creates a geojson.io URL for viewing the provided GeoJSON data.
func GenerateGeoJSONioURL(geojsonData string) string {
	var compactData any
	compactJSON := geojsonData
	if err := json.Unmarshal([]byte(geojsonData), &compactData); err == nil {
		if compact, err := json.Marshal(compactData); err == nil {
			compactJSON = string(compact)
		}
	}

	encodedData := url.QueryEscape(compactJSON)
	return fmt.Sprintf("https://geojson.io/#data=data:application/json,%s", encodedData)
}
