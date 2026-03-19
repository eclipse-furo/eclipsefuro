# フロー Furo

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A **protobuf-first framework** that uses Protocol Buffers as the sole Interface Definition Language (IDL). Furo provides protoc plugins that generate **TypeScript code** and **JSON Schema** files from `.proto` definitions.

## Repository Structure

| Directory | Description |
|-----------|-------------|
| [`protoc-gen-open-models/`](protoc-gen-open-models/) | Protoc plugin generating TypeScript for `@furo/open-models` (Literal/Transport/Model types, enums, services) |
| [`protoc-gen-om-jsonschema/`](protoc-gen-om-jsonschema/) | Protoc plugin generating JSON Schema (Draft 2020-12) from proto messages |
| [`protoc-gen-debugfile/`](protoc-gen-debugfile/) | Protoc plugin that captures the raw `CodeGeneratorRequest` for replay-based debugging |
| [`BEC/`](BEC/) | Build Essentials Container — Docker image bundling protoc, Go, Node, and all furo tooling |

Each plugin is a self-contained Go module with vendored dependencies.

## Quick Start

Build a plugin and run it with protoc:

```bash
cd protoc-gen-open-models
go build -o protoc-gen-open-models .

protoc \
  -I./proto_dependencies \
  -I./proto \
  --open-models_out=./generated \
  your_proto_files.proto
```

## Plugins

### protoc-gen-open-models (v1.48.0)

Generates TypeScript from proto files for the [`@furo/open-models`](https://www.npmjs.com/package/@furo/open-models) runtime. For each proto message, it produces a single `.ts` file containing:

- **Literal interface** (`IPerson`) — plain TypeScript interface, camelCase fields
- **Transport interface** (`TPerson`) — wire-format interface, snake_case fields
- **Model class** (`Person`) — runtime class extending `FieldNode` with getters, setters, and metadata

Also generates TypeScript enums and REST service classes from `google.api.http` annotations.

See [protoc-gen-open-models/README.md](protoc-gen-open-models/README.md) for full documentation.

### protoc-gen-om-jsonschema

Generates JSON Schema (Draft 2020-12) files from proto messages, optimized for AI consumption. One schema file per message/enum type.

Supports plugin options: `strict_any`, `strict_map`, `strict_oneof`, `file_extension`, `ref_prefix`, `exclude_packages`, `exclude_messages`.

```bash
cd protoc-gen-om-jsonschema
go build -o protoc-gen-om-jsonschema .

protoc \
  -I./proto_dependencies \
  -I./proto \
  --om-jsonschema_out=./schemas \
  --om-jsonschema_opt=file_extension=.json \
  your_proto_files.proto
```

See [protoc-gen-om-jsonschema/README.md](protoc-gen-om-jsonschema/README.md) for full documentation.

### protoc-gen-debugfile (v1.0.0)

Captures the raw `CodeGeneratorRequest` that protoc sends to plugins, enabling replay-based debugging in your IDE without needing protoc or the user's proto setup.

```bash
protoc \
  --debugfile_out=. \
  --debugfile_opt=/tmp/request.bin \
  --open-models_out=./generated \
  -I./proto_dependencies -I./proto \
  $(find proto -iname "*.proto")

# Later, replay without protoc:
./protoc-gen-open-models --replay-request=/tmp/request.bin
```

See [protoc-gen-debugfile/README.md](protoc-gen-debugfile/README.md) for full documentation.

## License

[MIT](LICENSE) — Copyright (c) 2021 Eclipse Foundation
