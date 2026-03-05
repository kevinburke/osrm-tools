#!/usr/bin/env bash

# Stop the OSRM routing server.
# Usage: ./stop-server.sh

set -euo pipefail

echo "Stopping OSRM server..."
docker stop osrm-bike 2>/dev/null || true
docker rm osrm-bike 2>/dev/null || true
echo "Server stopped."
