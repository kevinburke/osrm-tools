# osrm-tools

Generic tooling for running [OSRM](https://project-osrm.org/) routing analysis
on any geographic region. Includes Go library packages for route querying,
catchment area calculation, and GeoJSON export, plus parameterized scripts for
downloading OSM data, building OSRM Docker containers, and managing the routing
server.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (for running OSRM)
- [Go](https://go.dev/) 1.22+ (for the library packages)
- [osmium-tool](https://osmcode.org/osmium-tool/) (for clipping OSM data to a
  region boundary; `brew install osmium-tool`)
- [GEOS](https://libgeos.org/) (only needed if using the `hull` package for
  concave hull computation; `brew install geos`)

## Quick start

### 1. Create a region config

Create a `region.toml` describing your area (see `examples/region.toml`):

```toml
# Human-readable region name
name = "Contra Costa County"

# Geofabrik download URL for base OSM data
geofabrik_url = "https://download.geofabrik.de/north-america/us/california/norcal-latest.osm.pbf"

# OSM relation ID for the region boundary
osm_relation_id = 396462

# Port for the OSRM server (default: 5000)
osrm_port = 9367
```

| Field | Description |
|-------|-------------|
| `name` | Human-readable region name |
| `geofabrik_url` | Download URL for base OSM data ([Geofabrik downloads](https://download.geofabrik.de/)) |
| `osm_relation_id` | OSM relation ID for the region boundary ([how to find this](#finding-an-osm-relation-id)) |
| `osrm_port` | Port for the OSRM server (default: 5000) |
| `docker_platform` | Docker platform flag (default: `linux/amd64`) |
| `penalty_file` | Path to a region-specific Lua penalty file (optional) |

### 2. Download and prepare data

```sh
# Download OSM data
make download REGION_CONFIG=region.toml

# Download the region boundary polygon
make get-boundary REGION_CONFIG=region.toml

# Clip OSM data to the region (requires osmium-tool)
make clip REGION_CONFIG=region.toml
```

### 3. Build OSRM data and start the server

```sh
# Extract, partition, customize, and start the server
make rebuild REGION_CONFIG=region.toml
```

This runs the OSRM pipeline inside Docker with the custom bicycle profile and
starts a routing server on the configured port.

### 4. Test the server

```sh
curl 'http://localhost:9367/route/v1/cycling/-122.4194,37.7749;-122.4094,37.7849'
```

### Server management

```sh
make start REGION_CONFIG=region.toml   # start server
make stop                               # stop server
make restart REGION_CONFIG=region.toml  # restart
make status                             # check if running
make logs                               # tail server logs
```

## Go packages

```
go get github.com/kevinburke/osrm-tools
```

### `geo` — Geographic primitives

Zero external dependencies. Provides `Point`, distance calculations, coordinate
parsing, and polygon containment tests.

```go
import "github.com/kevinburke/osrm-tools/geo"

p := geo.ParseGoogleMapCoords("37.880040, -122.049138")
dist := geo.HaversineDistance(p1, p2)                      // meters
mPerDeg := geo.MetersPerDegreeLongitude(37.88)              // latitude-aware
inside := geo.IsPointInPolygon(point, polygon)
sw, ne := geo.BoundingBox(points)
```

### `osrm` — OSRM HTTP client

Typed client with `context.Context` support for the OSRM route and nearest
services.

```go
import "github.com/kevinburke/osrm-tools/osrm"

client := osrm.NewClient("http://localhost:9367")

// Get a route
resp, err := client.GetRouteWithGeometry(ctx, "cycling", from, to)

// Check road proximity
isNear, dist, err := client.IsPointNearRoad(ctx, "driving", point, 75.0)
```

Parse errors that indicate an incompatible OSRM server are returned as
`*osrm.CriticalError`, so callers can distinguish transient failures from
fundamental mismatches:

```go
var critErr *osrm.CriticalError
if errors.As(err, &critErr) {
    log.Fatal("OSRM server incompatible:", err)
}
```

### `geojson` — GeoJSON helpers

Construct GeoJSON features and feature collections without pulling in a full
GeoJSON library.

```go
import "github.com/kevinburke/osrm-tools/geojson"

fc := geojson.NewFeatureCollection()
fc.Add(geojson.NewPointFeature(lon, lat, map[string]any{"name": "Home"}))
fc.Add(geojson.NewPolygonFeature(ring, map[string]any{"fill": "#FF0000"}))
data, _ := fc.Marshal()

url := geojson.GenerateGeoJSONioURL(string(data))
```

### `hull` — Concave hull (CGO/GEOS)

Isolated in its own package so the GEOS dependency doesn't affect users who
don't need it. The rest of the library works without GEOS installed.

```go
import "github.com/kevinburke/osrm-tools/hull"

// ratio: 0.0 = tightest fit, 1.0 = convex hull
hullPoints, err := hull.ConcaveHull(points, 0.3)
```

### `catchment` — N-destination catchment calculator

For each point on a grid, routes to every destination and assigns the point to
whichever is closest by travel time. Generalizes to any number of destinations.

```go
import "github.com/kevinburke/osrm-tools/catchment"

dests := []catchment.Destination{
    {ID: "north", Name: "North School", Point: northPt, Color: "#FF0000"},
    {ID: "south", Name: "South School", Point: southPt, Color: "#0000FF"},
}

calc := catchment.NewCalculator(dests, sw, ne, 0.001, osrmClient)
grid := calc.GenerateGrid(ctx)
results, err := calc.CalculateCatchment(ctx, grid)
geojsonStr := calc.ExportToGeoJSON(results)
```

### `region` — Region configuration

Load and validate the region TOML config used by the shell scripts.

```go
import "github.com/kevinburke/osrm-tools/region"

cfg, err := region.LoadConfig("region.toml")
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}
fmt.Println(cfg.Port())        // 9367
fmt.Println(cfg.RegionSlug())  // "contra-costa-county"
```

## Lua penalty system

The routing profile (`profiles/bicycle.lua`) loads a generic penalty framework
from `penalties/street_preferences.lua`. Region-specific penalty data lives in a
separate file that gets mounted into Docker at `/opt/region_penalties.lua`.

### Writing a region penalty file

Copy `penalties/example-region.lua` and add your penalties:

```lua
local region = {}

region.node_penalties = {
  [417767169] = {
    name = "Dangerous Overpass",
    penalty = "high",                -- 80% speed reduction
    extra_duration_seconds = 240,
    description = "No bike infrastructure"
  },
  [57823162] = {
    name = "Protected Bike Lane",
    penalty = "bonus_high",          -- 100% speed increase
    description = "Route along protected lane"
  },
}

region.street_rules = {
  {pattern = "highway 101", penalty = "high", description = "Major highway"},
}

return region
```

### Available penalty levels

| Key | Multiplier | Effect |
|-----|-----------|--------|
| `high` | 0.2 | 80% speed reduction |
| `medium` | 0.5 | 50% speed reduction |
| `low` | 0.7 | 30% speed reduction |
| `bonus_low` | 1.3 | 30% speed increase |
| `bonus_medium` | 1.5 | 50% speed increase |
| `bonus_high` | 2.0 | 100% speed increase |

### Finding OSM node IDs

1. Go to [openstreetmap.org](https://www.openstreetmap.org/)
2. Navigate to the intersection or point of interest
3. Click "Edit" to open the editor
4. Select a node — the ID appears in the sidebar
5. Use the numeric ID in your penalty file

### Activating region penalties

Set `penalty_file` in your `region.toml`:

```toml
# Path to region-specific Lua penalty file
penalty_file = "penalties/my-region.lua"
```

The rebuild script mounts this file into the Docker container automatically.

## Building a custom OSRM Docker image

If you need features from a custom OSRM fork:

```sh
bash scripts/build-osrm-docker.sh [source_dir]
```

This clones (or updates) `kevinburke/osrm-backend` and builds an
`osrm-custom:latest` Docker image. The other scripts automatically prefer this
image when it exists.

## Finding an OSM relation ID

Every administrative boundary (country, state, county, city) in OpenStreetMap is
a "relation" with a numeric ID. You need this ID to download the boundary
polygon for clipping OSM data to your region.

### Option A: Search on openstreetmap.org

1. Go to [openstreetmap.org](https://www.openstreetmap.org/)
2. Search for your region (e.g. "Contra Costa County")
3. In the search results, look for the entry with a type like "Administrative"
   or "Boundary" — click on it
4. The URL will look like `https://www.openstreetmap.org/relation/396462` — the
   number at the end is the relation ID

### Option B: Use Nominatim

Search the [Nominatim API](https://nominatim.openstreetmap.org/) directly:

```
https://nominatim.openstreetmap.org/search?q=Contra+Costa+County&format=json
```

Look for the result with `"osm_type": "relation"` and use the `"osm_id"` value.

### Option C: Use the wiki

The OpenStreetMap wiki often lists relation IDs for well-known boundaries.
Search for your region at
[wiki.openstreetmap.org](https://wiki.openstreetmap.org/).

### Verifying a relation ID

You can preview the boundary before using it:

```
https://www.openstreetmap.org/relation/<ID>
```

For example: https://www.openstreetmap.org/relation/396462 shows Contra Costa
County, CA.
