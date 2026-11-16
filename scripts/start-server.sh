#!/usr/bin/env bash

# Start the OSRM routing server.
# Usage: ./start-server.sh <region.toml>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=toml-get.sh
source "$SCRIPT_DIR/toml-get.sh"
# shellcheck source=osrm-image.sh
source "$SCRIPT_DIR/osrm-image.sh"

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

PORT=$(toml_get "$REGION_CONFIG" osrm_port 5000)
PLATFORM=$(toml_get "$REGION_CONFIG" docker_platform "linux/amd64")
PENALTY_FILE=$(toml_get "$REGION_CONFIG" penalty_file "")
OSRM_IMAGE=$(osrm_image "$PENALTY_FILE")

# Find the OSRM data file by looking for .osrm.properties (osrm-routed takes the
# base path without the .properties extension).
OSRM_PROPS=$(find data -type f -name "*.osrm.properties" -print -quit)
if [ -z "$OSRM_PROPS" ]; then
    echo "Error: No OSRM data file found in data/"
    echo "Run ./scripts/rebuild-osrm-data.sh first."
    exit 1
fi
OSRM_FILE="${OSRM_PROPS%.properties}"

echo "Using OSRM data: $OSRM_FILE"

echo "Using OSRM image: $OSRM_IMAGE"

# Stop existing server if running
"${CONTAINER_ENGINE:-docker}" stop osrm-bike 2>/dev/null || true
"${CONTAINER_ENGINE:-docker}" rm osrm-bike 2>/dev/null || true

# Start server
"${CONTAINER_ENGINE:-docker}" run -d --platform "$PLATFORM" --name osrm-bike \
    -p "${PORT}:${PORT}" \
    -v "$PWD/data:/data" \
    "$OSRM_IMAGE" \
    osrm-routed --algorithm mld -p "${PORT}" "/data/${OSRM_FILE#data/}"

echo "OSRM server started on port ${PORT}"
echo "Test with: curl 'http://localhost:${PORT}/route/v1/cycling/-122.4194,37.7749;-122.4094,37.7849'"
