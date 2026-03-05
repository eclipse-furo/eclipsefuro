#!/usr/bin/env bash
set -euo pipefail

# Resolve project root as the directory containing this script
WD="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

# Directory holding all .proto files
SRC_DIR="$WD/proto"

# Output directory for JSON schema (strict mode)
SCHEMA_OUT_DIR="$WD/open-models/schema-strict"

command -v protoc >/dev/null 2>&1 || {
  echo "error: protoc not found in PATH" >&2
  exit 1
}

# Build the plugin
go build -o "$WD/protoc-gen-om-jsonschema" "$WD"

mkdir -p "$SCHEMA_OUT_DIR"
protoc \
        --plugin="protoc-gen-om-jsonschema=$WD/protoc-gen-om-jsonschema" \
        -I"$WD/proto_dependencies" \
         -I"$SRC_DIR" \
         --om-jsonschema_out="$SCHEMA_OUT_DIR" \
         --om-jsonschema_opt=strict_any,strict_map,strict_oneof \
       $(find "${SRC_DIR}" -iname "*.proto")
