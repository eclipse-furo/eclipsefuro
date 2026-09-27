# protoc-gen-om-jsonschema

This is a protoc plugin to generate JSON schema files out of proto files.
It creates a JSON schema file for each message type.

The schema files can be used to work with JSON data and AI's.
They are also useful for validating JSON data against the schema.

## Usage

```bash
# Build the plugin
go build -o protoc-gen-om-jsonschema .

# Check the version
./protoc-gen-om-jsonschema --version

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

### file_extension

By default, generated schema files have no file extension (e.g., `furo.cube.Colour`). Use `file_extension` to append an extension to all generated filenames.

```bash
# Append .json extension
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=file_extension=.json \
  your_proto_files.proto

# Append .schema.json extension
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=file_extension=.schema.json \
  your_proto_files.proto
```

**Default:** No extension (backward compatible). Output filenames are just the fully qualified message name (e.g., `google.protobuf.Any`).

**With `file_extension=.json`:** Output filenames become `google.protobuf.Any.json`, `mypackage.MyMessage.json`, etc.

### ref_prefix

By default, `$id` and `$ref` values use the fully qualified protobuf name (e.g., `google.protobuf.Any`). Use `ref_prefix` to prepend a URI prefix to all `$id` and `$ref` values while keeping output filenames local.

```bash
# Generate schemas with full URI identifiers
protoc \
  --om-jsonschema_out=./schemas \
  --om-jsonschema_opt=ref_prefix=https://example.com/schemas/,file_extension=.json \
  -I./proto your_proto_files.proto
```

This produces files like `schemas/google.protobuf.Any.json` containing:
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.com/schemas/google.protobuf.Any.json",
  ...
}
```

And `$ref` values like:
```json
{
  "$ref": "https://example.com/schemas/furo.cube.CubeDefinition.json"
}
```

**Default:** No prefix (backward compatible). `$id` and `$ref` values are just the fully qualified name plus any file extension.

### inline_refs

By default, message and enum fields reference the schema file of their type with `$ref`. Consumers that cannot resolve references to other files, like the parameters of an AI tool (function calling), need every schema self-contained. With `inline_refs`, message and enum fields are inlined into the schema of the message that uses them.

The field comment replaces the type comment on the inlined schema, since it says what this particular use of the type means. Field constraints (`maximum`, `default`, `readOnly`, ...) are kept on it.

A recursive message cannot be inlined forever. The root message of a file is referenced as `#`, any other recursive message is emitted once under `$defs` and referenced as `#/$defs/<full name>`.

**Proto:**
```protobuf
message Tree {
  // the root node
  Node root = 1;
}
message Node {
  string label = 1;
  repeated Node children = 2;
}
```

**Output for `Tree`:**
```json
{
  "$id": "Tree",
  "type": "object",
  "properties": {
    "root": { "$ref": "#/$defs/Node", "description": "the root node" }
  },
  "$defs": {
    "Node": {
      "type": "object",
      "properties": {
        "label": { "type": "string" },
        "children": { "type": "array", "items": { "$ref": "#/$defs/Node" } }
      }
    }
  }
}
```

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=inline_refs \
  your_proto_files.proto
```

### defs_min_size

With `inline_refs`, a message used in many places is copied at every use. In a densely linked package the copies multiply per level, and a single schema can reach megabytes. `defs_min_size` bounds that: a message used a second time whose inlined schema is at least this many bytes of compact JSON (the size an AI system is sent) moves to `$defs`, and every use, the first included, references it. The field comment stays next to the `$ref`. Smaller messages stay inline everywhere, which is easier for AI systems to follow.

Default: `3072`. `defs_min_size=0` never moves a message to `$defs` (recursive messages still go there).

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=inline_refs,defs_min_size=4096 \
  your_proto_files.proto
```

### skip_read_only

Leaves out fields a client cannot send, which are noise in an input schema:

- `(openapi.v3.property) = {read_only: true}`
- `(google.api.field_behavior) = OUTPUT_ONLY`
- `(google.api.field_behavior) = IDENTIFIER`

Skipped fields are also removed from `required` and from oneof groups. Without the option, `OUTPUT_ONLY` fields are marked `readOnly: true`, like openapi `read_only`.

### skip_unspecified_enum

Leaves out the zero value of an enum when its name ends in `_UNSPECIFIED`. proto3 requires that value, but it is never a meaningful choice for someone filling in the field.

### Schemas for AI tools

The three options together produce schemas that can be used as the parameters of an AI tool as they are:

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=file_extension=.json,inline_refs,skip_read_only,skip_unspecified_enum \
  your_proto_files.proto
```

Size is bounded by `defs_min_size` (on by default).

### Combining options

Multiple options can be combined with commas:

```bash
protoc \
  --om-jsonschema_out=./output \
  --om-jsonschema_opt=strict_any,strict_map,strict_oneof,file_extension=.json \
  your_proto_files.proto
```

### exclude_packages

Exclude entire packages from schema generation. Useful for skipping dependency packages like `google.api` or `openapi.v3`.

```bash
protoc \
  --om-jsonschema_out=./output \
  "--om-jsonschema_opt=exclude_packages=google.api;openapi.v3;google.rpc" \
  your_proto_files.proto
```

Package names are separated by semicolons. All messages and enums in excluded packages will be skipped.

### exclude_messages

Exclude specific messages or enums from schema generation. Useful for skipping internal types while keeping the rest of a package.

```bash
protoc \
  --om-jsonschema_out=./output \
  "--om-jsonschema_opt=exclude_messages=mypackage.InternalMessage;mypackage.DebugInfo" \
  your_proto_files.proto
```

Message names must be fully qualified (e.g., `package.MessageName`). This option also works for enums.

### Combining exclusions with other options

```bash
protoc \
  --om-jsonschema_out=./output \
  "--om-jsonschema_opt=strict_any,exclude_packages=google.api;openapi.v3,exclude_messages=mypackage.Internal" \
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

## Well-Known Types

Fields of google.protobuf well-known types are described in their JSON form, not their message structure, in every mode:

| Type | Schema |
|---|---|
| `Timestamp` | `{"type": "string", "format": "date-time"}` (RFC 3339) |
| `Duration` | string like `"3.5s"` |
| `DoubleValue`, `FloatValue` | `number` |
| `Int32Value`, `UInt32Value` | `integer` |
| `Int64Value`, `UInt64Value` | string of digits (64-bit integers are strings in JSON) |
| `BoolValue`, `StringValue`, `BytesValue` | `boolean`, `string`, `string` |
| `FieldMask` | string of comma separated camelCase paths |
| `Struct`, `Empty` | `object` |
| `ListValue` | `array` |
| `Value` | any value (no `type`) |
| `NullValue` | `null` |

The schema files of the google.protobuf types themselves still describe their messages.

## Field Behavior

`google.api.field_behavior` is honoured: `REQUIRED` fields are added to `required` (except fields of a oneof), `OUTPUT_ONLY` fields get `readOnly: true`, and `skip_read_only` leaves out `OUTPUT_ONLY` and `IDENTIFIER` fields.

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

Supported: `maximum`, `minimum`, `default`, `read_only`, `max_length`, `min_length` and `pattern`. `google.api.field_behavior` `OUTPUT_ONLY` is treated like `read_only`.

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
