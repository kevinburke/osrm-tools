// Package region provides configuration for OSRM region setup.
package region

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds the parameters for downloading, clipping, and building OSRM data
// for a geographic region.
type Config struct {
	// Name is a human-readable name for the region (e.g. "Contra Costa County").
	Name string `toml:"name"`

	// GeofabrikURL is the download URL for the base OSM data file.
	// Example: "https://download.geofabrik.de/north-america/us/california/norcal-latest.osm.pbf"
	GeofabrikURL string `toml:"geofabrik_url"`

	// OSMRelationID is the OpenStreetMap relation ID for the region boundary.
	// Used to download a .poly file for clipping. Example: 396462 for Contra Costa County.
	OSMRelationID int64 `toml:"osm_relation_id"`

	// OSRMPort is the port to run the OSRM server on (default: 5000).
	OSRMPort int `toml:"osrm_port,omitempty"`

	// DockerPlatform is the Docker platform flag (default: "linux/amd64").
	DockerPlatform string `toml:"docker_platform,omitempty"`

	// PenaltyFile is the path to a region-specific Lua penalty file (optional).
	// If set, it is mounted into the Docker container at /opt/region_penalties.lua.
	PenaltyFile string `toml:"penalty_file,omitempty"`
}

// LoadConfig reads a region config from a TOML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	return &cfg, nil
}

// Validate checks that all required fields are present and values are sensible.
func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("config: name is required")
	}
	if c.GeofabrikURL == "" {
		return fmt.Errorf("config: geofabrik_url is required")
	}
	if c.OSMRelationID <= 0 {
		return fmt.Errorf("config: osm_relation_id must be a positive integer")
	}
	return nil
}

// Port returns the configured OSRM port, defaulting to 5000.
func (c *Config) Port() int {
	if c.OSRMPort > 0 {
		return c.OSRMPort
	}
	return 5000
}

// Platform returns the configured Docker platform, defaulting to "linux/amd64".
func (c *Config) Platform() string {
	if c.DockerPlatform != "" {
		return c.DockerPlatform
	}
	return "linux/amd64"
}

// DataDir returns the data directory path for this region, based on the config file location.
func (c *Config) DataDir(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "data")
}

// RegionSlug returns a filesystem-safe slug derived from the region name.
func (c *Config) RegionSlug() string {
	slug := make([]byte, 0, len(c.Name))
	for _, r := range c.Name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			slug = append(slug, byte(r))
		case r >= 'A' && r <= 'Z':
			slug = append(slug, byte(r-'A'+'a'))
		case r == ' ':
			slug = append(slug, '-')
		}
	}
	return string(slug)
}
