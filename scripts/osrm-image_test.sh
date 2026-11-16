#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=osrm-image.sh
source "$SCRIPT_DIR/osrm-image.sh"
TEST_DIR=$(mktemp -d)
trap 'rm -rf "$TEST_DIR"' EXIT

cat > "$TEST_DIR/engine" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ "$*" == "image inspect osrm-custom:latest" ]]
exit "${IMAGE_INSPECT_STATUS}"
EOF
chmod +x "$TEST_DIR/engine"
export CONTAINER_ENGINE="$TEST_DIR/engine"
export IMAGE_INSPECT_STATUS=1

if osrm_image penalties/region.lua > "$TEST_DIR/image" 2> "$TEST_DIR/error"; then
    echo "Selecting an image with node penalties must fail without the custom image" >&2
    exit 1
fi
error=$(< "$TEST_DIR/error")
[[ "$error" == *'region penalties require the custom OSRM image'* ]]
[[ "$(osrm_image '')" == docker.io/osrm/osrm-backend ]]

export IMAGE_INSPECT_STATUS=0
[[ "$(osrm_image penalties/region.lua)" == osrm-custom:latest ]]
[[ "$(osrm_image '')" == osrm-custom:latest ]]
echo "osrm-image tests passed"
