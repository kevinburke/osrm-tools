#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR=$(mktemp -d)
trap 'rm -rf "$TEST_DIR"' EXIT

cat > "$TEST_DIR/region.toml" <<'EOF'
# A minimal consumer config intentionally omits optional settings.
name = "Contra Costa County" # comment
osrm_port = 9367
empty = ""
  penalty_file = 'penalties/walnut-creek.lua'
EOF

assert_value() {
    local expected="$1" key="$2" default="$3" actual
    # Run with the same strict options as the lifecycle scripts. A fresh shell
    # prevents an assertion's conditional context from disabling errexit.
    actual=$(bash -c 'set -euo pipefail; source "$1"; toml_get "$2" "$3" "$4"' \
        -- "$SCRIPT_DIR/toml-get.sh" "$TEST_DIR/region.toml" "$key" "$default")
    if [ "$actual" != "$expected" ]; then
        echo "toml_get $key: expected '$expected', got '$actual'" >&2
        exit 1
    fi
}

assert_value "Contra Costa County" name unknown
assert_value 9367 osrm_port 5000
assert_value penalties/walnut-creek.lua penalty_file ""
assert_value fallback empty fallback
assert_value linux/amd64 docker_platform linux/amd64
assert_value "" absent ""

echo "toml-get tests passed"
