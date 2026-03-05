#!/usr/bin/env bash

# Download the boundary polygon for a region from OpenStreetMap.
# Usage: ./get-boundary-polygon.sh <region.toml>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=toml-get.sh
source "$SCRIPT_DIR/toml-get.sh"

if [ $# -eq 0 ]; then
    echo "Usage: $0 <region.toml>"
    echo "Example: $0 examples/region.toml"
    echo ""
    echo "Downloads a .poly file using the osm_relation_id from the config."
    echo "To find the OSM relation ID:"
    echo "  1. Go to https://www.openstreetmap.org"
    echo "  2. Search for your county/region"
    echo "  3. Click on the administrative boundary"
    echo "  4. Look for 'relation' in the URL or page info"
    exit 1
fi

REGION_CONFIG="$1"

if [ ! -f "$REGION_CONFIG" ]; then
    echo "Error: config file not found: $REGION_CONFIG"
    exit 1
fi

OSM_RELATION_ID=$(toml_get "$REGION_CONFIG" osm_relation_id "")
REGION_NAME=$(toml_get "$REGION_CONFIG" name "unknown")

if [ -z "$OSM_RELATION_ID" ]; then
    echo "Error: osm_relation_id not set in $REGION_CONFIG"
    exit 1
fi

mkdir -p data/polygons
OUTPUT_FILE="data/polygons/region_${OSM_RELATION_ID}.poly"

echo "Downloading polygon for: $REGION_NAME (OSM relation ID: $OSM_RELATION_ID)"
echo "Output file: $OUTPUT_FILE"

curl --fail --silent --show-error --location \
    "https://polygons.openstreetmap.fr/get_poly.py?id=${OSM_RELATION_ID}&params=0" \
    --output "$OUTPUT_FILE"

if [ -f "$OUTPUT_FILE" ] && [ -s "$OUTPUT_FILE" ]; then
    echo "Successfully downloaded polygon to: $OUTPUT_FILE"
    echo "File size: $(du -h "$OUTPUT_FILE" | cut -f1)"
else
    echo "Error: Failed to download polygon or file is empty"
    exit 1
fi
