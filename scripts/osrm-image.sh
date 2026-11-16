#!/usr/bin/env bash

# Print the image to use without silently dropping region node penalties.
osrm_image() {
    local penalty_file="$1" engine="${CONTAINER_ENGINE:-docker}"
    if ! command -v "$engine" >/dev/null 2>&1; then
        echo "Error: container engine not found: $engine" >&2
        return 1
    fi
    if "$engine" image inspect osrm-custom:latest >/dev/null 2>&1; then
        echo osrm-custom:latest
    elif [ -n "$penalty_file" ]; then
        echo "Error: region penalties require the custom OSRM image; the stock image cannot apply node penalties." >&2
        echo "Build it with CONTAINER_ENGINE=$engine bash scripts/build-osrm-docker.sh <osrm-backend source directory>." >&2
        return 1
    else
        echo docker.io/osrm/osrm-backend
    fi
}
