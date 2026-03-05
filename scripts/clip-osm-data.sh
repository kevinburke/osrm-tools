#!/usr/bin/env bash

# Clip OSM data to the region boundary polygon.
# Usage: ./clip-osm-data.sh <region.toml>
#
# Requires: osmium-tool (brew install osmium-tool)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=toml-get.sh
source "$SCRIPT_DIR/toml-get.sh"

if [ $# -eq 0 ]; then
    echo "Usage: $0 <region.toml>"
    echo "Example: $0 examples/region.toml"
    exit 1
fi

REGION_CONFIG="$1"

if [ ! -f "$REGION_CONFIG" ]; then
    echo "Error: config file not found: $REGION_CONFIG"
    exit 1
fi

OSM_RELATION_ID=$(toml_get "$REGION_CONFIG" osm_relation_id "")
REGION_NAME=$(toml_get "$REGION_CONFIG" name "unknown")

POLY_FILE="data/polygons/region_${OSM_RELATION_ID}.poly"

if [ ! -f "$POLY_FILE" ]; then
    echo "Error: polygon file not found: $POLY_FILE"
    echo "Run ./scripts/get-boundary-polygon.sh first."
    exit 1
fi

# Find the downloaded OSM data file
OSM_FILE=$(find data/raw -name "*.osm.pbf" | head -1)
if [ -z "$OSM_FILE" ]; then
    echo "Error: No .osm.pbf file found in data/raw/"
    echo "Run ./scripts/download-osm-data.sh first."
    exit 1
fi

OUTPUT_FILE="data/raw/clipped.osm.pbf"

echo "Clipping OSM data for: $REGION_NAME"
echo "  Input:   $OSM_FILE"
echo "  Polygon: $POLY_FILE"
echo "  Output:  $OUTPUT_FILE"

osmium extract -p "$POLY_FILE" "$OSM_FILE" -o "$OUTPUT_FILE" --overwrite

echo "Clipping complete: $OUTPUT_FILE"
ls -lh "$OUTPUT_FILE"
