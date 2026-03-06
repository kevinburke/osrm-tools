## infer-sidewalks

Geometrically matches separately-mapped OSM sidewalk ways to their parent roads,
inferring `sidewalk=left/right/both` tags.

### Test

```bash
go test -trimpath ./beta/sidewalk/...
```

### Build

```bash
mkdir -p tmp
go build -trimpath -o tmp/ ./beta/cmd/infer-sidewalks
```

### Run (San Miguel CDP subset)

```bash
tmp/infer-sidewalks \
  --pbf ~/src/github.com/kevinburke/walnut-creek-bikes/data/raw/clipped.osm.pbf \
  --boundary ~/src/github.com/kevinburke/walnut-creek-bikes/data/san-miguel-cdp.geojson \
  --html tmp/san-miguel-sidewalks.html \
  --osrm-output tmp/san-miguel-sidewalk-tags.json
```

Then open `tmp/san-miguel-sidewalks.html` in a browser to visually verify matches.
Click any road or sidewalk feature for a popup with the proposed tag and an OSM link.

### Run (full PBF, no boundary filter)

```bash
tmp/infer-sidewalks \
  --pbf ~/src/github.com/kevinburke/walnut-creek-bikes/data/raw/clipped.osm.pbf \
  --output tmp/sidewalks.geojson \
  --osrm-output tmp/sidewalk-tags.json
```

### Review mode (interactive)

```bash
tmp/infer-sidewalks \
  --pbf ~/src/github.com/kevinburke/walnut-creek-bikes/data/raw/clipped.osm.pbf \
  --boundary ~/src/github.com/kevinburke/walnut-creek-bikes/data/san-miguel-cdp.geojson \
  --review
```

Opens http://127.0.0.1:3612 with a Leaflet map and side panel. Step through each
candidate, press **y** to confirm or **n** to reject. Click **Finish & Export** to
write `confirmed.osc` (osmChange XML for JOSM upload) and `rejected.json`.

`--review` is mutually exclusive with `--output`, `--html`, and `--osrm-output`.

### Flags

- `--pbf` (required) — path to OSM PBF file
- `--boundary` — GeoJSON file with a Polygon to filter ways (only keeps ways with a node inside)
- `--output` — GeoJSON output path (default: stdout)
- `--html` — Leaflet HTML output with roads, sidewalks, and OSM links
- `--osrm-output` — JSON array of `{"way_id": N, "sidewalk": "both"}` for OSRM Lua profile
- `--review` — start a local HTTP server for interactive review
- `--review-port` — port for review server (default 3612)
- `--confirmed` — osmChange XML output path (default `confirmed.osc`)
- `--rejected` — rejected candidates JSON output path (default `rejected.json`)
- `--max-distance` — max road-to-sidewalk distance in meters (default 20)
- `--min-coverage` — minimum fraction of road length covered (default 0.5)
- `--version`
