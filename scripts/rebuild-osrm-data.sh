#!/usr/bin/env bash

# Rebuild OSRM data with custom bicycle profile.
# Usage: ./rebuild-osrm-data.sh <region.toml>
#
# This runs: extract -> partition -> customize -> start server

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

PORT=$(toml_get "$REGION_CONFIG" osrm_port 5000)
PLATFORM=$(toml_get "$REGION_CONFIG" docker_platform "linux/amd64")
PENALTY_FILE=$(toml_get "$REGION_CONFIG" penalty_file "")

if [ ! -f "profiles/bicycle.lua" ]; then
    echo "Error: Custom bicycle profile not found at profiles/bicycle.lua"
    exit 1
fi

if [ ! -f "penalties/street_preferences.lua" ]; then
    echo "Error: Street preferences module not found at penalties/street_preferences.lua"
    exit 1
fi

# Ensure data directory exists
mkdir -p data/raw data/processed

# Find OSM data file (prefer clipped, then any .osm.pbf)
OSM_FILE=$(find data/raw -name "clipped.osm.pbf" | head -1)
if [ -z "$OSM_FILE" ]; then
    OSM_FILE=$(find data/raw -name "*.osm.pbf" | head -1)
fi
if [ -z "$OSM_FILE" ]; then
    echo "Error: No OSM data file found in data/raw/"
    echo "Run ./scripts/download-osm-data.sh (and optionally ./scripts/clip-osm-data.sh) first."
    exit 1
fi

OSM_BASENAME=$(basename "$OSM_FILE" .osm.pbf)

echo "Found OSM data: $OSM_FILE"
echo "Building OSRM data with custom bicycle profile..."

# Stop existing server if running
docker stop osrm-bike 2>/dev/null || true
docker rm osrm-bike 2>/dev/null || true

# Determine which OSRM image to use
OSRM_IMAGE="osrm/osrm-backend"
if docker image inspect osrm-custom:latest >/dev/null 2>&1; then
    OSRM_IMAGE="osrm-custom:latest"
    echo "Using custom OSRM image: $OSRM_IMAGE"
else
    echo "Using official OSRM image: $OSRM_IMAGE"
fi

# Build volume mount args for penalties
PENALTY_ARGS=()
if [ -n "$PENALTY_FILE" ] && [ -f "$PENALTY_FILE" ]; then
    PENALTY_ARGS=(-v "$PWD/$PENALTY_FILE:/opt/region_penalties.lua")
    echo "Mounting region penalties: $PENALTY_FILE"
fi

# Step 1: Extract
echo "Step 1/3: Extract..."
docker run --platform "$PLATFORM" \
    -v "$PWD/data:/data" \
    -v "$PWD/profiles:/profiles" \
    -v "$PWD/penalties/street_preferences.lua:/opt/street_preferences.lua" \
    "${PENALTY_ARGS[@]}" \
    "$OSRM_IMAGE" \
    osrm-extract -p /profiles/bicycle.lua "/data/${OSM_FILE#data/}"

# Step 2: Partition
echo "Step 2/3: Partition..."
docker run --platform "$PLATFORM" \
    -v "$PWD/data/raw:/data" \
    "$OSRM_IMAGE" \
    osrm-partition /data/"$OSM_BASENAME.osrm"

# Step 3: Customize
echo "Step 3/3: Customize..."
docker run --platform "$PLATFORM" \
    -v "$PWD/data/raw:/data" \
    "$OSRM_IMAGE" \
    osrm-customize /data/"$OSM_BASENAME.osrm"

# Move OSRM files to processed directory
echo "Moving files to processed directory..."
mv data/raw/"$OSM_BASENAME".osrm* data/processed/

echo ""
echo "OSRM data rebuilt with custom bicycle profile."
echo "Generated files:"
ls -la data/processed/"$OSM_BASENAME".osrm*
echo ""

# Auto-start the server
echo "Starting OSRM server..."
OSRM_FILE=$(find data/processed -name "*.osrm" | head -1)
if [ -z "$OSRM_FILE" ]; then
    echo "Warning: No OSRM data file found after processing."
    exit 1
fi

docker run -d --platform "$PLATFORM" --name osrm-bike \
    -p "${PORT}:${PORT}" \
    -v "$PWD/data:/data" \
    "$OSRM_IMAGE" \
    osrm-routed --algorithm mld -p "${PORT}" "/data/${OSRM_FILE#data/}"

echo ""
echo "OSRM server started on port ${PORT}"
echo "Test with: curl 'http://localhost:${PORT}/route/v1/cycling/-122.4194,37.7749;-122.4094,37.7849'"
