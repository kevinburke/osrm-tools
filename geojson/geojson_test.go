package geojson

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewFeatureCollection(t *testing.T) {
	fc := NewFeatureCollection()
	if fc.Type != "FeatureCollection" {
		t.Errorf("expected type FeatureCollection, got %s", fc.Type)
	}
	if len(fc.Features) != 0 {
		t.Errorf("expected 0 features, got %d", len(fc.Features))
	}
}

func TestNewPointFeature(t *testing.T) {
	f := NewPointFeature(-122.4, 37.8, map[string]any{"name": "test"})
	if f.Type != "Feature" {
		t.Errorf("expected type Feature, got %s", f.Type)
	}

	geomType, ok := f.Geometry["type"].(string)
	if !ok || geomType != "Point" {
		t.Errorf("expected geometry type Point, got %v", f.Geometry["type"])
	}

	coords, ok := f.Geometry["coordinates"].([]float64)
	if !ok || len(coords) != 2 {
		t.Fatalf("expected 2 coordinates, got %v", f.Geometry["coordinates"])
	}
	if coords[0] != -122.4 || coords[1] != 37.8 {
		t.Errorf("expected [-122.4, 37.8], got %v", coords)
	}
}

func TestNewPolygonFeatureClosesRing(t *testing.T) {
	ring := [][]float64{
		{0, 0}, {1, 0}, {1, 1}, {0, 1},
	}
	f := NewPolygonFeature(ring, nil)

	coordsRaw := f.Geometry["coordinates"]
	coords, ok := coordsRaw.([][][]float64)
	if !ok {
		t.Fatalf("unexpected coordinate type %T", coordsRaw)
	}

	outer := coords[0]
	if len(outer) != 5 {
		t.Errorf("expected ring to be closed (5 points), got %d", len(outer))
	}

	if outer[0][0] != outer[len(outer)-1][0] || outer[0][1] != outer[len(outer)-1][1] {
		t.Error("ring is not closed")
	}
}

func TestMarshal(t *testing.T) {
	fc := NewFeatureCollection()
	fc.Add(NewPointFeature(0, 0, map[string]any{"hello": "world"}))

	data, err := fc.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Should be valid JSON
	var check map[string]any
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("Marshal output is not valid JSON: %v", err)
	}

	if check["type"] != "FeatureCollection" {
		t.Errorf("unexpected type in output: %v", check["type"])
	}
}

func TestLeafletVersionInTemplate(t *testing.T) {
	html := GenerateLeafletHTML(`{"type":"FeatureCollection","features":[]}`)
	wantCSS := "https://unpkg.com/leaflet@" + LeafletVersion + "/dist/leaflet.css"
	if !strings.Contains(html, wantCSS) {
		t.Errorf("template missing CSS link %q", wantCSS)
	}
	wantJS := "https://unpkg.com/leaflet@" + LeafletVersion + "/dist/leaflet.js"
	if !strings.Contains(html, wantJS) {
		t.Errorf("template missing JS link %q", wantJS)
	}
}

func TestGenerateLeafletHTML(t *testing.T) {
	fc := NewFeatureCollection()
	fc.Add(NewPointFeature(-122.4, 37.8, map[string]any{"name": "SF"}))
	data, err := fc.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	html := GenerateLeafletHTML(string(data))

	// Should not contain the placeholder.
	if strings.Contains(html, "GEOJSON_PLACEHOLDER") {
		t.Error("placeholder was not replaced")
	}

	// Should contain the GeoJSON data.
	if !strings.Contains(html, `"name": "SF"`) {
		t.Error("GeoJSON data not found in output")
	}

	// Basic HTML structure checks.
	for _, want := range []string{
		"<!doctype html>",
		"<html lang=",
		"</html>",
		`<div id="map">`,
		"L.map(",
		"L.geoJSON(",
		"L.tileLayer(",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing expected string %q", want)
		}
	}
}

func TestGenerateLeafletHTMLEmptyCollection(t *testing.T) {
	html := GenerateLeafletHTML(`{"type":"FeatureCollection","features":[]}`)

	if strings.Contains(html, "GEOJSON_PLACEHOLDER") {
		t.Error("placeholder was not replaced")
	}
	if !strings.Contains(html, `"FeatureCollection"`) {
		t.Error("empty collection not present in output")
	}
}

func TestGenerateGeoJSONioURL(t *testing.T) {
	input := `{"type":"FeatureCollection","features":[]}`
	result := GenerateGeoJSONioURL(input)

	if !strings.HasPrefix(result, "https://geojson.io/#data=data:application/json,") {
		t.Errorf("unexpected URL prefix: %s", result)
	}

	if !strings.Contains(result, "FeatureCollection") {
		t.Errorf("URL should contain FeatureCollection: %s", result)
	}
}
