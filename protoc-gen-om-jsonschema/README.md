# protoc-gen-om-jsonschema

This is a protoc plugin to generate JSON schema files out of proto files.
It creates a JSON schema file for each message type.

The schema files can be used to work with JSON data and AI's.
They are also useful for validating JSON data against the schema.

## Usage

```bash
# Build the plugin
go build -o protoc-gen-om-jsonschema .

# Run protoc with the plugin
protoc \
  -I./proto_dependencies \
  -I./proto \
  --om-jsonschema_out=./output \
  your_proto_files.proto
```

## Plugin Options

### strict_any

By default, `google.protobuf.Any` is generated with an AI-friendly schema using `@type` and `additionalProperties: true`. This matches the JSON serialization format and is easier for AI systems to work with.

**Proto:**
```protobuf
import "google/protobuf/any.proto";

message Example {
  google.protobuf.Any payload = 1;
}
```

**Default output (AI-friendly):**
```json
{
  "$id": "google.protobuf.Any",
  "type": "object",
  "description": "A container for any message type. The @type field identifies which message type is contained, and the remaining fields are the actual message data. Set @type to the $id of the target schema and include all fields from that schema.",
  "properties": {
    "@type": {
      "type": "string",
      "description": "The $id of the schema that describes the additional fields in this object."
    }
  },
  "required": ["@type"],
  "additionalProperties": true
}
```

**With `strict_any` option:**
```json
{
  "$id": "google.protobuf.Any",
  "type": "object",
  "properties": {
    "typeUrl": {
      "type": "string",
      "description": "A URL/resource name that uniquely identifies the type..."
    },
    "value": {
      "type": "string",
      "description": "Must be a valid serialized protocol buffer of the above specified type."
    }
  }
}
```

To use the strict format:

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=strict_any \
  your_proto_files.proto
```

### strict_map

By default, `map<K,V>` fields are generated with an AI-friendly schema using `additionalProperties`. This matches the actual JSON wire format where maps serialize as objects.

**Proto:**
```protobuf
message Example {
  map<string, string> attributes = 1;
  map<string, MyMessage> items = 2;
}
```

**Default output (AI-friendly):**
```json
{
  "$id": "Example",
  "type": "object",
  "properties": {
    "attributes": {
      "type": "object",
      "additionalProperties": {
        "type": "string"
      }
    },
    "items": {
      "type": "object",
      "additionalProperties": {
        "$ref": "MyMessage"
      }
    }
  }
}
```

**With `strict_map` option:**

Main schema:
```json
{
  "$id": "Example",
  "type": "object",
  "properties": {
    "attributes": {
      "$ref": "Example.AttributesEntry"
    },
    "items": {
      "$ref": "Example.ItemsEntry"
    }
  }
}
```

Separate Entry schema file:
```json
{
  "$id": "Example.AttributesEntry",
  "type": "object",
  "properties": {
    "key": { "type": "string" },
    "value": { "type": "string" }
  }
}
```

To use the strict format:

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=strict_map \
  your_proto_files.proto
```

### strict_oneof

By default, `oneof` groups are indicated using both:
1. The `x-oneof` extension field (machine-readable)
2. An IMPORTANT hint in the description explaining how to use `x-oneof`

With `strict_oneof`, only a concise description hint is used (no `x-oneof` extension).

**Proto:**
```protobuf
message Content {
  oneof payload {
    string text = 1;
    bytes binary = 2;
    MyMessage structured = 3;
  }
}
```

**Default output (x-oneof with explanatory hint):**
```json
{
  "$id": "Content",
  "type": "object",
  "description": "Original description of the type... | IMPORTANT: This message contains mutually exclusive field groups defined in x-oneof. Each x-oneof entry has a 'name' (the group name) and 'fields' (the field names). You must set exactly ONE field from each group, not multiple. Setting multiple fields from the same group will cause data loss.",
  "properties": {
    "text": { "type": "string" },
    "binary": { "type": "string" },
    "structured": { "$ref": "MyMessage" }
  },
  "x-oneof": [
    {
      "name": "payload",
      "fields": ["text", "binary", "structured"]
    }
  ]
}
```

**With `strict_oneof` option (concise description only):**
```json
{
  "$id": "Content",
  "type": "object",
  "description": "Original description of the type... | ONEOF[payload]: Set exactly ONE of [text, binary, structured]",
  "properties": {
    "text": { "type": "string" },
    "binary": { "type": "string" },
    "structured": { "$ref": "MyMessage" }
  }
}
```

To use the concise description format:

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=strict_oneof \
  your_proto_files.proto
```

### Combining options

Multiple options can be combined with commas:

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=strict_any,strict_map,strict_oneof \
  your_proto_files.proto
```

### exclude_packages

Exclude entire packages from schema generation. Useful for skipping dependency packages like `google.api` or `openapi.v3`.

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=exclude_packages=google.api;openapi.v3;google.rpc \
  your_proto_files.proto
```

Package names are separated by semicolons. All messages and enums in excluded packages will be skipped.

### exclude_messages

Exclude specific messages or enums from schema generation. Useful for skipping internal types while keeping the rest of a package.

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=exclude_messages=mypackage.InternalMessage;mypackage.DebugInfo \
  your_proto_files.proto
```

Message names must be fully qualified (e.g., `package.MessageName`). This option also works for enums.

### Combining exclusions with other options

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=strict_any,exclude_packages=google.api;openapi.v3,exclude_messages=mypackage.Internal \
  your_proto_files.proto
```

## Complex Type Handling

This section documents how protobuf's complex types are represented in the generated JSON Schema. The plugin generates schemas optimized for AI consumption.

### repeated (Arrays)

Protobuf `repeated` fields are output as JSON Schema arrays with an `items` schema describing the element type.

**Proto:**
```protobuf
message Example {
  repeated string tags = 1;
  repeated MyMessage items = 2;
}
```

**Generated JSON Schema:**
```json
{
  "properties": {
    "tags": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "items": {
      "type": "array",
      "items": {
        "$ref": "MyMessage"
      }
    }
  }
}
```

### map<K,V> (Maps)

By default, protobuf `map` fields are represented using `additionalProperties`, matching the actual JSON wire format.

**Proto:**
```protobuf
message Example {
  map<string, string> attributes = 1;
  map<string, MyMessage> items = 2;
}
```

**Generated JSON Schema (default):**
```json
{
  "properties": {
    "attributes": {
      "type": "object",
      "additionalProperties": {
        "type": "string"
      }
    },
    "items": {
      "type": "object",
      "additionalProperties": {
        "$ref": "MyMessage"
      }
    }
  }
}
```

**Why:** This matches how maps are actually serialized to JSON: `{"key1": "value1", "key2": "value2"}`. AI systems can directly generate this format without understanding protobuf internals.

**With `strict_map` option:** Uses separate `*Entry` schema files with `key` and `value` properties, reflecting protobuf's internal representation.

### oneof (Union Types)

Protobuf `oneof` fields are output as separate properties with an `x-oneof` extension and an explanatory description hint to help AI understand the mutual exclusivity constraint.

**Proto:**
```protobuf
message Example {
  oneof content {
    string text = 1;
    bytes binary = 2;
    MyMessage structured = 3;
  }
}
```

**Generated JSON Schema (default):**
```json
{
  "description": "Original description of the type... | IMPORTANT: This message contains mutually exclusive field groups defined in x-oneof. Each x-oneof entry has a 'name' (the group name) and 'fields' (the field names). You must set exactly ONE field from each group, not multiple. Setting multiple fields from the same group will cause data loss.",
  "properties": {
    "text": { "type": "string" },
    "binary": { "type": "string" },
    "structured": { "$ref": "MyMessage" }
  },
  "x-oneof": [
    {
      "name": "content",
      "fields": ["text", "binary", "structured"]
    }
  ]
}
```

Each `x-oneof` entry has a `name` (the oneof group name) and `fields` (the field names that belong to that group). Only one field from each group should be set.

**With `strict_oneof` option:** Uses only a concise description hint without the `x-oneof` extension.

### Design Philosophy

This plugin prioritizes **AI readability** over strict JSON Schema validation:

1. **Simplicity**: Schemas show the essential type information without complex validation constructs
2. **References**: Message and enum types use `$ref` pointing to separate schema files by their `$id`
3. **Flat structure**: Each message/enum gets its own schema file, enabling modular understanding
4. **Implicit semantics**: Array cardinality, map structure, and oneof exclusivity are understood from protobuf conventions rather than explicit JSON Schema constraints

For strict JSON Schema validation, consider using a different tool or extending this plugin.

## OpenAPI v3 Annotations

The plugin supports `openapi.v3.property` annotations for additional schema constraints:

```protobuf
message Example {
  int32 count = 1 [(openapi.v3.property) = {
    maximum: 100
    minimum: 10
    default: {number: 50}
  }];
}
```

## Known Issue: Proto3 Zero Values

Due to proto3 wire format limitations, `minimum: 0` cannot be detected directly (proto3 doesn't encode fields set to their default value).

**Automatic handling**: Unsigned integer types (`uint32`, `uint64`, `fixed32`, `fixed64`) automatically get `minimum: 0`.

**Workaround for signed/float types**: Use `format: "positive"` to indicate a minimum of 0:

```protobuf
float percentage = 1 [(openapi.v3.property) = {
  maximum: 100
  format: "positive"  // Sets minimum: 0 in JSON Schema output
}];
```
