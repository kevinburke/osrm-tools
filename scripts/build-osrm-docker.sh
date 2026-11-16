#!/usr/bin/env bash

# Build a custom OSRM Docker image from source.
# Usage: ./build-osrm-docker.sh [source_dir]
#
# If source_dir is not specified, use the standard GOPATH-style checkout.
# Existing checkouts are built as-is; commit changes before building.

set -euo pipefail

OSRM_SOURCE_DIR="${1:-$HOME/src/github.com/kevinburke/osrm-backend}"
OSRM_REPO_URL="https://github.com/kevinburke/osrm-backend.git"
TARGET_BRANCH="develop"
IMAGE_NAME="osrm-custom"
IMAGE_TAG="latest"

echo "Building custom OSRM Docker image..."

if [ ! -d "$OSRM_SOURCE_DIR" ]; then
    echo "Cloning kevinburke/osrm-backend..."
    git clone --branch "$TARGET_BRANCH" "$OSRM_REPO_URL" "$OSRM_SOURCE_DIR"
fi

cd "$OSRM_SOURCE_DIR"

# Never switch branches, rewrite remotes, or pull into a developer checkout.
# The image tags and embedded revision must identify the source actually built.
if [ -n "$(git status --porcelain)" ]; then
    echo "Error: OSRM source has local changes; commit them before building: $PWD" >&2
    exit 1
fi

if [ ! -f "docker/Dockerfile" ]; then
    echo "Error: Dockerfile not found at docker/Dockerfile"
    exit 1
fi

CURRENT_COMMIT=$(git rev-parse HEAD)
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "Building from commit: $CURRENT_COMMIT on branch: $CURRENT_BRANCH"

"${CONTAINER_ENGINE:-docker}" build \
    --platform linux/amd64 \
    --build-arg DOCKER_TAG="$IMAGE_TAG" \
    --build-arg OSRM_GITSHA="$CURRENT_COMMIT" \
    --build-arg BUILD_CONCURRENCY="$(nproc 2>/dev/null || sysctl -n hw.ncpu)" \
    -t "$IMAGE_NAME:$IMAGE_TAG" \
    -t "$IMAGE_NAME:$CURRENT_COMMIT" \
    -f docker/Dockerfile \
    .

echo ""
echo "Successfully built custom OSRM Docker image: $IMAGE_NAME:$IMAGE_TAG"
echo "Also tagged: $IMAGE_NAME:$CURRENT_COMMIT"
