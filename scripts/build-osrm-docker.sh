#!/usr/bin/env bash

# Build a custom OSRM Docker image from source.
# Usage: ./build-osrm-docker.sh [source_dir]
#
# If source_dir is not specified, clones kevinburke/osrm-backend to ../osrm-backend.

set -euo pipefail

OSRM_SOURCE_DIR="${1:-../osrm-backend}"
OSRM_REPO_URL="https://github.com/kevinburke/osrm-backend.git"
TARGET_BRANCH="develop"
IMAGE_NAME="osrm-custom"
IMAGE_TAG="latest"

echo "Building custom OSRM Docker image..."

if [ ! -d "$OSRM_SOURCE_DIR" ]; then
    echo "Cloning kevinburke/osrm-backend..."
    git clone "$OSRM_REPO_URL" "$OSRM_SOURCE_DIR"
fi

cd "$OSRM_SOURCE_DIR"

CURRENT_ORIGIN=$(git remote get-url origin)
if [[ "$CURRENT_ORIGIN" != *"kevinburke/osrm-backend"* ]]; then
    echo "Warning: Repository origin is $CURRENT_ORIGIN"
    echo "Expected kevinburke/osrm-backend. Updating remote..."
    git remote set-url origin "$OSRM_REPO_URL"
fi

echo "Updating repository and checking out $TARGET_BRANCH..."
git fetch origin
git checkout "$TARGET_BRANCH"
git pull origin "$TARGET_BRANCH"

if [ ! -f "docker/Dockerfile" ]; then
    echo "Error: Dockerfile not found at docker/Dockerfile"
    exit 1
fi

CURRENT_COMMIT=$(git rev-parse --short HEAD)
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "Building from commit: $CURRENT_COMMIT on branch: $CURRENT_BRANCH"

docker build \
    --platform linux/amd64 \
    --build-arg DOCKER_TAG="$IMAGE_TAG" \
    --build-arg BUILD_CONCURRENCY="$(nproc 2>/dev/null || sysctl -n hw.ncpu)" \
    -t "$IMAGE_NAME:$IMAGE_TAG" \
    -t "$IMAGE_NAME:$CURRENT_COMMIT" \
    -f docker/Dockerfile \
    .

echo ""
echo "Successfully built custom OSRM Docker image: $IMAGE_NAME:$IMAGE_TAG"
echo "Also tagged: $IMAGE_NAME:$CURRENT_COMMIT"
