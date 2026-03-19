# protoc-gen-open-models

This is a protoc plugin that generates TypeScript code from proto files for the `@furo/open-models` module.
It creates three representations per message — a Literal interface, a Transport interface, and a Model class —
along with TypeScript enums and REST service classes.

## Usage

```bash
# Build the plugin
go build -o protoc-gen-open-models .

# Run protoc with the plugin
protoc \
  -I./proto_dependencies \
  -I./proto \
  --open-models_out=./generated \
  your_proto_files.proto
```

## Generated Types

For each proto message, one `.ts` file is generated containing three types.

### Literal Interface (`I<Name>`)

A plain TypeScript interface using camelCase field names (JSON convention). All fields are optional.

**Proto:**
```protobuf
message Person {
  string first_name = 1;
  int32 age = 2;
  repeated string tags = 3;
}
```

**Generated:**
```typescript
export interface IPerson {
  firstName?: string;
  age?: number;
  tags?: string[];
}
```

### Transport Interface (`T<Name>`)

A TypeScript interface using snake_case field names (proto wire convention). All fields are optional.

```typescript
export interface TPerson {
  first_name?: string;
  age?: number;
  tags?: string[];
}
```

### Model Class (`<Name>`)

A runtime class extending `FieldNode` with getters, setters, field metadata, and registry support.

```typescript
export class Person extends FieldNode {
  private _firstName: STRING;
  private _age: INT32;
  private _tags: ARRAY<STRING, string>;

  constructor(
    initData?: IPerson,
    parent?: FieldNode,
    parentAttributeName?: string,
  )

  public get firstName(): string { ... }
  public set firstName(v: string) { ... }

  public get age(): number { ... }
  public set age(v: number) { ... }

  public get tags(): ARRAY<STRING, string> { ... }
  public set tags(v: IPerson["tags"]) { ... }

  fromLiteral(data: IPerson): void
  toLiteral(): IPerson
}

Registry.register('person.Person', Person);
```

Each field carries metadata (`__meta.nodeFields`) including the JSON name, proto name, type constructor,
optional constraints from OpenAPI annotations, and the field description from proto comments.

## Enums

Proto enums become TypeScript string enums where each value maps to its own name.

**Proto:**
```protobuf
enum Colour {
  RED = 0;
  GREEN = 1;
  BLUE = 2;
}
```

**Generated:**
```typescript
export enum Colour {
  RED = "RED",
  GREEN = "GREEN",
  BLUE = "BLUE",
}
```

In Model types, enum fields use `ENUM<EnumType>`.

## Services

RPC methods annotated with `google.api.http` are generated as `StrictFetcher` properties.

**Proto:**
```protobuf
import "google/api/annotations.proto";

service PersonService {
  rpc GetPerson(GetPersonRequest) returns (Person) {
    option (google.api.http) = {
      get: "/api/persons/{person_id}"
    };
  }

  rpc CreatePerson(Person) returns (Person) {
    option (google.api.http) = {
      post: "/api/persons"
      body: "*"
    };
  }
}
```

**Generated:**
```typescript
export class PersonService {
  public GetPerson: StrictFetcher<IGetPersonRequest, IPerson> =
    new StrictFetcher<IGetPersonRequest, IPerson>(
      API_OPTIONS,
      'GET',
      '/api/persons/{person_id}',
      GetPersonRequest,
      Person,
    );

  public CreatePerson: StrictFetcher<IPerson, IPerson> =
    new StrictFetcher<IPerson, IPerson>(
      API_OPTIONS,
      'POST',
      '/api/persons',
      Person,
      Person,
      '*',
    );
}
```

Supported HTTP verbs: `GET`, `PUT`, `POST`, `PATCH`, `DELETE`, and custom verbs.

Server-streaming RPCs wrap the response type in `AsyncIterable<>`.

## Primitive Type Mappings

| Proto Type | Literal / Transport | Model Type | Model Primitive |
|------------|---------------------|------------|-----------------|
| `string` | `string` | `STRING` | `string` |
| `bytes` | `string` | `BYTES` | `string` |
| `bool` | `boolean` | `BOOLEAN` | `boolean` |
| `int32` | `number` | `INT32` | `number` |
| `int64` | `string` | `INT64` | `bigint` |
| `double` | `number` | `DOUBLE` | `number` |
| `float` | `number` | `FLOAT` | `number` |
| `uint32` | `number` | `UINT32` | `number` |
| `uint64` | `string` | `UINT64` | `bigint` |
| `fixed32` | `number` | `FIXED32` | `number` |
| `fixed64` | `string` | `FIXED64` | `bigint` |
| `sfixed32` | `number` | `SFIXED32` | `number` |
| `sfixed64` | `string` | `SFIXED64` | `bigint` |
| `sint32` | `number` | `SINT32` | `number` |
| `sint64` | `string` | `SINT64` | `bigint` |

64-bit integer types map to `string` in Literal/Transport (JSON safe) and `bigint` in Model.

## Well-Known Types

| Proto Type | Literal / Transport | Model |
|------------|---------------------|-------|
| `google.protobuf.StringValue` | `string` | `string` |
| `google.protobuf.BytesValue` | `string` | `string` |
| `google.protobuf.BoolValue` | `boolean` | `boolean` |
| `google.protobuf.Int32Value` | `number` | `number` |
| `google.protobuf.Int64Value` | `string` | `bigint` |
| `google.protobuf.FloatValue` | `number` | `number` |
| `google.protobuf.DoubleValue` | `number` | `number` |
| `google.protobuf.UInt32Value` | `number` | `number` |
| `google.protobuf.UInt64Value` | `string` | `bigint` |
| `google.protobuf.Timestamp` | `string` | `string` |
| `google.protobuf.Duration` | `string` | `string` |
| `google.protobuf.Struct` | `JSONObject` | `JSONObject` |
| `google.protobuf.Empty` | `Record<string, never>` | `Record<string, never>` |
| `google.protobuf.FieldMask` | `string[]` | `string[]` |
| `google.protobuf.Any` | `IAny` | `IAny` |

## Complex Type Handling

### repeated (Arrays)

Repeated fields become arrays in Literal/Transport and `ARRAY<>` in Model.

**Proto:**
```protobuf
message Example {
  repeated string tags = 1;
  repeated Person people = 2;
}
```

**Literal/Transport:**
```typescript
tags?: string[];
people?: IPerson[];
```

**Model:**
```typescript
private _tags: ARRAY<STRING, string>;
private _people: ARRAY<Person, IPerson>;
```

### map<K,V> (Maps)

Map fields become object types with index signatures in Literal/Transport and `MAP<>` in Model.

**Proto:**
```protobuf
message Example {
  map<string, int32> scores = 1;
  map<string, Person> people = 2;
}
```

**Literal/Transport:**
```typescript
scores?: { [key: string]: number };
people?: { [key: string]: IPerson };
```

**Model:**
```typescript
private _scores: MAP<string, INT32, number>;
private _people: MAP<string, Person, IPerson>;
```

### oneof

Oneof is not yet fully implemented. This depends on `@furo/open-models` runtime support.

### Self-Recursion and Deep Recursion

When a message references itself (directly or through a chain), the Model type uses `RECURSION<>` to prevent infinite instantiation.

**Proto:**
```protobuf
message TreeNode {
  string label = 1;
  TreeNode child = 2;
}
```

**Model:**
```typescript
private _child: RECURSION<TreeNode, ITreeNode>;
```

The plugin performs cycle detection across the message graph to identify both direct and deep recursion.

## OpenAPI v3 Annotations

### Field-Level Constraints (`openapi.v3.property`)

Field annotations are extracted and stored as `FieldConstraints` in the Model's metadata.

```protobuf
import "openapi/v3/annotations.proto";

message Product {
  string name = 1 [(openapi.v3.property) = {
    min_length: 1
    max_length: 255
    pattern: "^[a-zA-Z]"
  }];

  int32 quantity = 2 [(openapi.v3.property) = {
    minimum: 0
    maximum: 1000
    default: {number: 1}
  }];

  float price = 3 [(openapi.v3.property) = {
    read_only: true
  }];
}
```

Supported constraint fields:
- **Numeric**: `minimum`, `maximum`, `exclusive_minimum`, `exclusive_maximum`, `multiple_of`
- **String**: `pattern`, `min_length`, `max_length`
- **Array**: `min_items`, `max_items`, `unique_items`
- **Object**: `min_properties`, `max_properties`
- **Flags**: `read_only`, `write_only`, `deprecated`, `nullable`, `required`
- **Descriptive**: `title`, `description`, `format`, `type`
- **Defaults**: `default` (applied during Model construction when no init data is provided)

### Message-Level Annotations (`openapi.v3.schema`)

Mark specific fields as required at the message level:

```protobuf
message Account {
  option (openapi.v3.schema) = {
    required: ["account_id", "display_name"]
  };
  string account_id = 1;
  string display_name = 2;
  string description = 3;
}
```

## Reserved Word Handling

TypeScript class names that collide with built-in globals are automatically prefixed with `X`:

| Proto Name | Generated Name |
|------------|----------------|
| `JSONObject` | `XJSONObject` |
| `Object` | `XObject` |
| `Any` | `XAny` |
| `String` | `XString` |
| `Number` | `XNumber` |
| `Date` | `XDate` |

This applies to the generated class, interface, and file names.

## Debugging

Use `protoc-gen-debugfile` to capture the raw `CodeGeneratorRequest`, then replay it for IDE-friendly debugging:

```bash
# Capture the request
protoc \
  --debugfile_out=. \
  --debugfile_opt=/tmp/request.bin \
  --open-models_out=./generated \
  -I./proto_dependencies -I./proto \
  $(find proto -iname "*.proto")

# Replay without protoc
cd protoc-gen-open-models
go build -o protoc-gen-open-models .
./protoc-gen-open-models --replay-request=/tmp/request.bin
```
