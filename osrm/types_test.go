package osrm

import (
	"encoding/json"
	"testing"
)

func TestOriginalStructWouldFail(t *testing.T) {
	type OriginalNearestResponse struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Waypoints []struct {
			Location []float64 `json:"location"`
			Distance float64   `json:"distance"`
			Name     string    `json:"name"`
			Hint     string    `json:"hint,omitempty"`
			Nodes    []int64   `json:"nodes,omitempty"` // This is the problematic field
			Classes  []string  `json:"classes,omitempty"`
		} `json:"waypoints"`
	}

	jsonData := `{
		"code": "Ok",
		"waypoints": [
			{
				"nodes": [12812510950.0, 12812510950.0],
				"location": [-122.041021, 37.896936],
				"name": "",
				"distance": 47.73252338
			}
		]
	}`

	var response OriginalNearestResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	if err == nil {
		t.Errorf("Expected parsing to fail with original struct, but it succeeded")
	}
}

func TestFlexibleInt64WithLargeNumbers(t *testing.T) {
	jsonData := `{
		"code": "Ok",
		"waypoints": [
			{
				"nodes": [
					12812510950.0,
					12812510950.0
				],
				"hint": "vR9pgMMfaYDUAQAAPgAAAL4QAAAAAAAAGyBQQicN3UBNGu5DAAAAANQBAAA-AAAAvhAAAAAAAABBAAAAQ825-OhCQgIAz7n43kNCAkAADwkAAAAA",
				"location": [
					-122.041021,
					37.896936
				],
				"name": "",
				"distance": 47.73252338
			}
		]
	}`

	var response NearestResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal with FlexibleInt64: %v", err)
	}

	if response.Code != "Ok" {
		t.Errorf("Expected response code 'Ok', got '%s'", response.Code)
	}

	if len(response.Waypoints) == 0 || len(response.Waypoints[0].Nodes) == 0 {
		t.Fatal("No waypoints or nodes found")
	}

	nodeValue := response.Waypoints[0].Nodes[0].Int64()
	expectedValue := int64(12812510950)
	if nodeValue != expectedValue {
		t.Errorf("Expected node value %d, got %d", expectedValue, nodeValue)
	}
}

func TestRouteResponseParsing(t *testing.T) {
	jsonData := `{
		"code": "Ok",
		"routes": [
			{
				"duration": 300.5,
				"distance": 1500.0,
				"geometry": {
					"type": "LineString",
					"coordinates": [[-122.041021, 37.896936], [-122.041112, 37.896946]]
				},
				"legs": [
					{
						"duration": 300.5,
						"distance": 1500.0,
						"summary": "Test route",
						"steps": [],
						"annotation": {
							"nodes": [10705724944, 1070572494e+1],
							"durations": [100.0, 200.5],
							"distances": [500.0, 1000.0],
							"weight": [100.0, 200.5],
							"datasources": [1, 1],
							"ways": [123456789, 987654321]
						}
					}
				]
			}
		]
	}`

	var response RouteResponse
	err := json.Unmarshal([]byte(jsonData), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal route response: %v", err)
	}

	if response.Code != "Ok" {
		t.Errorf("Expected code 'Ok', got '%s'", response.Code)
	}

	if len(response.Routes) == 0 {
		t.Fatal("No routes found")
	}

	route := response.Routes[0]
	if route.Duration != 300.5 {
		t.Errorf("Expected duration 300.5, got %f", route.Duration)
	}
	if route.Distance != 1500.0 {
		t.Errorf("Expected distance 1500.0, got %f", route.Distance)
	}
}

func TestIsAcceptableRoadType(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"Main Street", true},
		{"Oak Road", true},
		{"Bike Lane", true},
		{"Hiking Trail", false},
		{"Dirt Path", false},
		{"Fire Road North", false},
		{"", false},
		{"Broadway", true},         // generic name, default accept
		{"Horse Trail", false},     // rejected
		{"Gravel Path", false},     // rejected
		{"Cypress Avenue", true},   // acceptable
		{"Iron Horse Line", false}, // rejected
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAcceptableRoadType(tt.name); got != tt.expected {
				t.Errorf("IsAcceptableRoadType(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}
