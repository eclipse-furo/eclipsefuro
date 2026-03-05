#!/usr/bin/env bash
set -euo pipefail

# Resolve project root as the directory containing this script
WD="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

# Directory holding all .proto files
SRC_DIR="$WD/proto"

# Output directory for JSON schema with exclusions
SCHEMA_OUT_DIR="$WD/open-models/schema-exclude"

command -v protoc >/dev/null 2>&1 || {
  echo "error: protoc not found in PATH" >&2
  exit 1
}

# Build the plugin
go build -o "$WD/protoc-gen-om-jsonschema" "$WD"

# Clean previous output
rm -rf "$SCHEMA_OUT_DIR"
mkdir -p "$SCHEMA_OUT_DIR"

# Run protoc with exclude_packages and exclude_messages options
# This excludes:
# - All types from google.api and openapi.v3 packages
# - Specific message google.protobuf.FieldMask
protoc \
        --plugin="protoc-gen-om-jsonschema=$WD/protoc-gen-om-jsonschema" \
        -I"$WD/proto_dependencies" \
         -I"$SRC_DIR" \
         --om-jsonschema_out="$SCHEMA_OUT_DIR" \
         --om-jsonschema_opt="exclude_packages=google.api;openapi.v3,exclude_messages=google.protobuf.FieldMask" \
       $(find "${SRC_DIR}" -iname "*.proto")

echo "Generated schemas to $SCHEMA_OUT_DIR"
echo ""
echo "Excluded packages: google.api, openapi.v3"
echo "Excluded messages: google.protobuf.FieldMask"
echo ""
echo "Checking exclusions..."

# Verify exclusions worked
if ls "$SCHEMA_OUT_DIR"/google.api.* 2>/dev/null; then
  echo "ERROR: google.api.* files should not exist!"
  exit 1
else
  echo "✓ google.api.* correctly excluded"
fi

if ls "$SCHEMA_OUT_DIR"/openapi.v3.* 2>/dev/null; then
  echo "ERROR: openapi.v3.* files should not exist!"
  exit 1
else
  echo "✓ openapi.v3.* correctly excluded"
fi

if [ -f "$SCHEMA_OUT_DIR/google.protobuf.FieldMask" ]; then
  echo "ERROR: google.protobuf.FieldMask should not exist!"
  exit 1
else
  echo "✓ google.protobuf.FieldMask correctly excluded"
fi

# Verify some files were still generated
if [ -f "$SCHEMA_OUT_DIR/google.protobuf.Any" ]; then
  echo "✓ google.protobuf.Any still generated (not excluded)"
else
  echo "ERROR: google.protobuf.Any should exist!"
  exit 1
fi

echo ""
echo "All exclusion tests passed!"
