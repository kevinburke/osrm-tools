package main

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinburke/osrm-tools/beta/sidewalk"
	"github.com/paulmach/osm"
)

func testServer(t *testing.T) (*reviewServer, *httptest.Server) {
	t.Helper()
	candidates := []sidewalk.Candidate{
		{
			Road: sidewalk.Road{
				ID:     100,
				Tags:   map[string]string{"highway": "residential", "name": "Main St"},
				Coords: [][2]float64{{-122.06, 37.9}, {-122.05, 37.9}},
			},
			InferredTag:   "both",
			LeftCoverage:  0.85,
			RightCoverage: 0.90,
			Matches: []sidewalk.Match{
				{RoadID: 100, SidewalkID: 200, Side: "left", Coverage: 0.85},
				{RoadID: 100, SidewalkID: 201, Side: "right", Coverage: 0.90},
			},
			SegmentResults: []sidewalk.SegmentResult{
				{HasLeft: true, HasRight: true, Length: 100},
			},
		},
		{
			Road: sidewalk.Road{
				ID:     101,
				Tags:   map[string]string{"highway": "tertiary", "name": "Oak Ave"},
				Coords: [][2]float64{{-122.07, 37.91}, {-122.06, 37.91}},
			},
			InferredTag:   "right",
			LeftCoverage:  0.1,
			RightCoverage: 0.75,
			Matches: []sidewalk.Match{
				{RoadID: 101, SidewalkID: 202, Side: "right", Coverage: 0.75},
			},
			SegmentResults: []sidewalk.SegmentResult{
				{HasLeft: false, HasRight: true, Length: 80},
			},
		},
	}

	sidewalks := []sidewalk.Sidewalk{
		{ID: 200, Coords: [][2]float64{{-122.06, 37.9001}, {-122.05, 37.9001}}},
		{ID: 201, Coords: [][2]float64{{-122.06, 37.8999}, {-122.05, 37.8999}}},
		{ID: 202, Coords: [][2]float64{{-122.07, 37.9099}, {-122.06, 37.9099}}},
	}

	meta := map[int64]wayMeta{
		100: {Version: 5, NodeIDs: []osm.NodeID{1001, 1002}},
		101: {Version: 3, NodeIDs: []osm.NodeID{1003, 1004}},
	}

	srv := &reviewServer{
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		candidates:    candidates,
		sidewalks:     sidewalks,
		meta:          meta,
		confirmedPath: filepath.Join(t.TempDir(), "confirmed.osc"),
		rejectedPath:  filepath.Join(t.TempDir(), "rejected.json"),
	}

	ts := httptest.NewServer(srv.handler())
	t.Cleanup(ts.Close)
	return srv, ts
}

func TestReviewIndex(t *testing.T) {
	_, ts := testServer(t)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("GET / status = %d, want 200", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Should contain candidate data.
	if !strings.Contains(html, "Main St") {
		t.Error("HTML does not contain candidate road name 'Main St'")
	}
	if !strings.Contains(html, "Oak Ave") {
		t.Error("HTML does not contain candidate road name 'Oak Ave'")
	}
	if !strings.Contains(html, "leaflet") {
		t.Error("HTML does not contain leaflet reference")
	}
}

func TestReviewExport(t *testing.T) {
	srv, ts := testServer(t)

	reqBody := `{"confirmed": [100], "rejected": [101]}`
	resp, err := http.Post(ts.URL+"/api/export", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST /api/export: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /api/export status = %d, body = %s", resp.StatusCode, body)
	}

	var result exportResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if result.ConfirmedCount != 1 {
		t.Errorf("confirmed_count = %d, want 1", result.ConfirmedCount)
	}
	if result.RejectedCount != 1 {
		t.Errorf("rejected_count = %d, want 1", result.RejectedCount)
	}

	// Verify osmChange XML.
	oscData, err := os.ReadFile(srv.confirmedPath)
	if err != nil {
		t.Fatalf("reading confirmed.osc: %v", err)
	}

	var change osm.Change
	// Strip XML declaration before unmarshaling.
	xmlContent := string(oscData)
	if idx := strings.Index(xmlContent, "<osmChange"); idx >= 0 {
		xmlContent = xmlContent[idx:]
	}
	if err := xml.Unmarshal([]byte(xmlContent), &change); err != nil {
		t.Fatalf("parsing osmChange XML: %v\n%s", err, oscData)
	}

	if change.Modify == nil || len(change.Modify.Ways) != 1 {
		t.Fatalf("expected 1 modified way, got %v", change.Modify)
	}

	way := change.Modify.Ways[0]
	if way.ID != 100 {
		t.Errorf("way ID = %d, want 100", way.ID)
	}
	if way.Version != 5 {
		t.Errorf("way version = %d, want 5", way.Version)
	}
	if !way.Visible {
		t.Error("way visible = false, want true")
	}
	if len(way.Nodes) != 2 {
		t.Errorf("way nodes = %d, want 2", len(way.Nodes))
	}

	// Should have sidewalk:both=separate tag.
	swTag := way.Tags.Find("sidewalk:both")
	if swTag != "separate" {
		t.Errorf("sidewalk:both tag = %q, want %q", swTag, "separate")
	}

	// Verify rejected JSON.
	rejData, err := os.ReadFile(srv.rejectedPath)
	if err != nil {
		t.Fatalf("reading rejected.json: %v", err)
	}

	var rejected []rejectedEntry
	if err := json.Unmarshal(rejData, &rejected); err != nil {
		t.Fatalf("parsing rejected JSON: %v", err)
	}
	if len(rejected) != 1 {
		t.Fatalf("expected 1 rejected entry, got %d", len(rejected))
	}
	if rejected[0].WayID != 101 {
		t.Errorf("rejected way_id = %d, want 101", rejected[0].WayID)
	}
	if rejected[0].ProposedTag != "right" {
		t.Errorf("rejected proposed_tag = %q, want %q", rejected[0].ProposedTag, "right")
	}
}

func TestReviewExportEmpty(t *testing.T) {
	_, ts := testServer(t)

	resp, err := http.Post(ts.URL+"/api/export", "application/json",
		strings.NewReader(`{"confirmed": [], "rejected": []}`))
	if err != nil {
		t.Fatalf("POST /api/export: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result exportResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if result.ConfirmedCount != 0 || result.RejectedCount != 0 {
		t.Errorf("counts = %d/%d, want 0/0", result.ConfirmedCount, result.RejectedCount)
	}
}
