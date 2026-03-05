#!/usr/bin/env bash

# Download OSM data for a region.
# Usage: ./download-osm-data.sh <region.toml>

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

GEOFABRIK_URL=$(toml_get "$REGION_CONFIG" geofabrik_url "")

if [ -z "$GEOFABRIK_URL" ]; then
    echo "Error: geofabrik_url not set in $REGION_CONFIG"
    exit 1
fi

mkdir -p data/raw
echo "Downloading OSM data from: $GEOFABRIK_URL"
curl --location --remote-name --output-dir data/raw "$GEOFABRIK_URL"

echo "Download complete. Files in data/raw/:"
ls -lh data/raw/*.osm.pbf
