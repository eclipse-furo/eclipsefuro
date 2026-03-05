# Contributing to protoc-gen-om-jsonschema

This document provides technical guidance for developers working on this project.

## Debugging the Plugin

Debugging protoc plugins is challenging because they receive input via stdin from protoc and must respond via stdout. The plugin includes an undocumented debug mode to help with this.

### Step 1: Capture the CodeGeneratorRequest

Use `protoc-gen-debugfile` to capture the raw `CodeGeneratorRequest` protobuf to a file during a normal protoc run:

```bash
protoc \
  -I./proto_dependencies \
  -I./proto \
  --debugfile_out=. \
  --debugfile_opt=/tmp/request.bin \
  --om-jsonschema_out=./open-models/schema \
  $(find proto -iname "*.proto")
```

This saves the binary `CodeGeneratorRequest` to `/tmp/request.bin`. Build `protoc-gen-debugfile` first if needed: `cd ../protoc-gen-debugfile && go build -o protoc-gen-debugfile .`

### Step 2: Run the Plugin Standalone

Use `--replay-request` to run the plugin directly without protoc, reading the saved request:

```bash
./protoc-gen-om-jsonschema --replay-request=/tmp/request.bin
```

This allows you to:
- Set breakpoints in your IDE
- Add debug logging
- Step through the code with a debugger
- Test changes without running the full protoc pipeline

### IDE Debugging Example (VS Code)

Create a launch configuration in `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Plugin",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}",
      "args": ["--replay-request=/tmp/request.bin"]
    }
  ]
}
```

### IDE Debugging Example (GoLand/IntelliJ)

1. Create a "Go Build" run configuration
2. Set "Program arguments" to `--replay-request=/tmp/request.bin`
3. Set breakpoints and run in debug mode

### How It Works

1. `protoc-gen-debugfile` runs as a separate protoc plugin and captures the raw `CodeGeneratorRequest` to a file
2. Later, `--replay-request=<path>` reads this file and feeds it to the plugin as if it came from protoc

This approach preserves the exact input that protoc would send, including all file descriptors, options, and parameters.

## Build and Test Commands

```bash
# Build the plugin (binary name must match "protoc-gen-om-jsonschema" for protoc discovery)
go build -o protoc-gen-om-jsonschema .

# Run all tests
go test -v ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

# Generate schemas with default options
./test-gen-open-models.sh

# Generate schemas with strict options
./test-gen-open-models-strict.sh

# Test exclusion options
./test-gen-open-models-exclude.sh
```

### Manual protoc Invocation

```bash
# Basic usage
protoc \
  -I./proto_dependencies \
  -I./proto \
  --om-jsonschema_out=./open-models/schema \
  $(find proto -iname "*.proto")

# With options
protoc \
  -I./proto_dependencies \
  -I./proto \
  --om-jsonschema_out=./open-models/schema \
  --om-jsonschema_opt=strict_any,exclude_packages=google.api;openapi.v3 \
  $(find proto -iname "*.proto")
```

## Plugin Options

| Option | Description |
|--------|-------------|
| `strict_any` | Use standard `typeUrl`/`value` format for `google.protobuf.Any` instead of AI-friendly `@type` format |
| `strict_map` | Use `*Entry` types for maps instead of `additionalProperties` |
| `strict_oneof` | Use description hints for oneof instead of `x-oneof` extension |
| `exclude_packages` | Semicolon-separated packages to exclude (e.g., `google.api;openapi.v3`) |
| `exclude_messages` | Semicolon-separated messages/enums to exclude (e.g., `mypackage.Internal`) |

Options are combined with commas: `--om-jsonschema_opt=strict_any,strict_map,exclude_packages=google.api`

## Project Structure

```
.
├── main.go                 # Main plugin logic (schema generation)
├── proto_parser.go         # Low-level protobuf wire format parsing
├── main_test.go            # Unit and integration tests
├── proto/                  # Sample proto files for testing
├── proto_dependencies/     # Third-party proto files (google.protobuf, etc.)
├── testfiles/              # Expected output for default mode tests + request.bin
├── testfiles-strict/       # Expected output for strict mode tests
├── open-models/            # Generated output directories
│   ├── schema/             # Default mode output
│   ├── schema-strict/      # Strict mode output
│   └── schema-exclude/     # Exclusion test output
└── test-gen-*.sh           # Shell scripts for manual testing
```

## Architecture

The plugin implements the standard protoc plugin pattern:

1. **Input**: Receives `CodeGeneratorRequest` from protoc via stdin
2. **Processing**: Iterates through all messages and enums in the request
3. **Conversion**: Transforms each protobuf type to JSON Schema
4. **Output**: Writes JSON Schema files to the output directory

### Key Files

- **main.go** (~820 lines): Core generation logic
  - `main()` - Entry point, flag parsing
  - `generateFile()` - Processes a single .proto file
  - `generateMessageSchema()` - Converts a message to JSON Schema
  - `generateEnumSchema()` - Converts an enum to JSON Schema
  - `convertField()` - Converts a field to JSON Schema property
  - `getOneofGroups()` - Extracts oneof group information

- **proto_parser.go** (~230 lines): Wire format parsing
  - `detectMinMaxFromRaw()` - Finds min/max in raw protobuf bytes
  - `decodeVarint()` - Decodes variable-length integers
  - `skipField()` - Skips over protobuf fields

### OpenAPI v3 Annotations

The plugin extracts constraints from `openapi.v3.property` and `openapi.v3.schema` annotations:

```protobuf
message Example {
  option (openapi.v3.schema) = {required: ["name"]};

  string name = 1;
  int32 count = 2 [(openapi.v3.property) = {
    maximum: 100
    minimum: 1
    default: {number: 10}
  }];
}
```

Supported properties:
- `maximum`, `minimum` - Numeric constraints
- `default` - Default values (number, boolean, string)
- `read_only` - Marks fields as read-only
- `required` - Array of required field names (message-level)
- `format` - Custom format hints (`"positive"` sets `minimum: 0`)

## Testing Workflow

1. **Unit tests** in `main_test.go` test individual functions
2. **Integration tests** run protoc as a subprocess and verify output
3. **Comparison tests** compare generated output against `testfiles/` and `testfiles-strict/`
4. **Coverage tests** use `testfiles/request.bin` to test schema generation directly

### Adding New Test Cases

1. Add proto file to `proto/` or use existing `proto_dependencies/`
2. Run `./test-gen-open-models.sh` to generate output
3. Copy expected output to `testfiles/` (or `testfiles-strict/`)
4. Add the file name to the test case list in `main_test.go`

### Regenerating testfiles/request.bin

The file `testfiles/request.bin` is a captured `CodeGeneratorRequest` used by coverage tests. **Regenerate it after changing proto files:**

```bash
# Build protoc-gen-debugfile first (if not already built)
cd ../protoc-gen-debugfile && go build -o protoc-gen-debugfile . && cd ../protoc-gen-om-jsonschema

protoc \
  -I./proto_dependencies -I./proto \
  --debugfile_out=. \
  --debugfile_opt=./testfiles/request.bin \
  $(find proto -iname "*.proto")
```

This file enables tests to call `processRequest()` directly, achieving ~77% code coverage without running protoc as a subprocess.

## Known Limitations

### Proto3 Zero Value Detection

Proto3 doesn't encode fields set to their default value. This means `minimum: 0` cannot be detected directly.

**Workarounds:**
- Unsigned types (`uint32`, etc.) automatically get `minimum: 0`
- Use `format: "positive"` annotation for signed types

### Property Order

Go maps are unordered, so JSON properties may appear in different order than in the proto file. Tests should not depend on property order.

## Output Format

- **File names**: Fully qualified message name (e.g., `google.protobuf.Any`)
- **No extension**: Files have no `.json` extension
- **Flat structure**: All files in one directory
- **Schema version**: JSON Schema Draft 2020-12

Example output:
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "mypackage.MyMessage",
  "type": "object",
  "properties": {
    "fieldName": {
      "type": "string",
      "description": "Field description from proto comments"
    }
  }
}
```
