#!/usr/bin/env bash

# Helper for reading values from flat TOML config files.
# Source this file, then call toml_get:
#
#   source scripts/toml-get.sh
#   PORT=$(toml_get region.toml osrm_port 5000)
#
# Only handles simple key = "value" and key = 123 lines.
# Does not support tables, arrays, or multiline values.

toml_get() {
    local file="$1" key="$2" default="$3"

    if [ ! -f "$file" ]; then
        echo "$default"
        return
    fi

    # Match lines like: key = "value" or key = 123
    # Strip comments, leading/trailing whitespace, and quotes.
    local value
    # An absent optional key is successful and should use the default. Unlike
    # grep, awk returns success for no match under the callers' pipefail mode.
    value=$(awk -v key="$key" '$0 ~ "^[[:space:]]*" key "[[:space:]]*=" { print; exit }' "$file" \
        | sed 's/^[^=]*=[[:space:]]*//' \
        | sed 's/[[:space:]]*#.*//' \
        | sed 's/^"//; s/"$//' \
        | sed "s/^'//; s/'$//")

    if [ -z "$value" ]; then
        echo "$default"
    else
        echo "$value"
    fi
}
