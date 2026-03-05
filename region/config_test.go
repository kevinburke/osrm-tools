package region

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	content := `
name = "Contra Costa County"
geofabrik_url = "https://download.geofabrik.de/north-america/us/california/norcal-latest.osm.pbf"
osm_relation_id = 396462
osrm_port = 9367
`

	dir := t.TempDir()
	path := filepath.Join(dir, "region.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Name != "Contra Costa County" {
		t.Errorf("expected name 'Contra Costa County', got %q", cfg.Name)
	}
	if cfg.OSMRelationID != 396462 {
		t.Errorf("expected relation ID 396462, got %d", cfg.OSMRelationID)
	}
	if cfg.Port() != 9367 {
		t.Errorf("expected port 9367, got %d", cfg.Port())
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid",
			cfg: Config{
				Name:          "Test",
				GeofabrikURL:  "https://example.com/test.osm.pbf",
				OSMRelationID: 123,
			},
			wantErr: false,
		},
		{name: "missing name", cfg: Config{GeofabrikURL: "x", OSMRelationID: 1}, wantErr: true},
		{name: "missing url", cfg: Config{Name: "x", OSMRelationID: 1}, wantErr: true},
		{name: "missing relation", cfg: Config{Name: "x", GeofabrikURL: "x"}, wantErr: true},
		{name: "negative relation", cfg: Config{Name: "x", GeofabrikURL: "x", OSMRelationID: -1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaults(t *testing.T) {
	cfg := Config{}
	if cfg.Port() != 5000 {
		t.Errorf("default port should be 5000, got %d", cfg.Port())
	}
	if cfg.Platform() != "linux/amd64" {
		t.Errorf("default platform should be linux/amd64, got %s", cfg.Platform())
	}
}

func TestRegionSlug(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"Contra Costa County", "contra-costa-county"},
		{"San Francisco", "san-francisco"},
		{"NYC 10001", "nyc-10001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Name: tt.name}
			if got := cfg.RegionSlug(); got != tt.expected {
				t.Errorf("RegionSlug() = %q, want %q", got, tt.expected)
			}
		})
	}
}
