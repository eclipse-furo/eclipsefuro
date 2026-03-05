# protoc-gen-debugfile

A minimal protoc plugin that captures the raw `CodeGeneratorRequest` to a file for replay-based debugging of any protoc plugin.

## The Problem

Debugging protoc plugins is hard. Protoc communicates with plugins through stdin/stdout using serialized protobuf messages - you can't just run a plugin binary in your IDE debugger because there's no way to provide the exact input that protoc would send. This makes setting breakpoints, stepping through code, and inspecting state during development painful.

## How It Works

`protoc-gen-debugfile` runs alongside your actual plugin during a normal `protoc` invocation. It reads the `CodeGeneratorRequest` from stdin (exactly what protoc sends to every plugin), writes the raw bytes to a file, and returns an empty response. The captured file can then be fed to any plugin's `--replay-request` flag to reproduce the exact same input without needing protoc.

## Build

```bash
cd protoc-gen-debugfile
go build -o protoc-gen-debugfile .
```

Make sure the binary is on your `$PATH` or in the same directory where you run `protoc`.

## Usage

Run `protoc` with `--debugfile_out` and `--debugfile_opt` alongside your actual plugin:

```bash
protoc \
  --debugfile_out=. \
  --debugfile_opt=/tmp/request.bin \
  --om-jsonschema_out=./schema \
  -I./proto_dependencies -I./proto \
  $(find proto -iname "*.proto")
```

- `--debugfile_out=.` enables the plugin (the output directory is unused but required by protoc)
- `--debugfile_opt=/tmp/request.bin` specifies where to save the captured request

The plugin writes a status message to stderr:

```
protoc-gen-debugfile: saved CodeGeneratorRequest (123456 bytes) to /tmp/request.bin
```

## Replaying a Captured Request

Once you have a captured `request.bin`, pass it to any plugin that supports `--replay-request`:

```bash
# Replay with protoc-gen-om-jsonschema
./protoc-gen-om-jsonschema --replay-request=/tmp/request.bin

# Replay with protoc-gen-open-models
./protoc-gen-open-models --replay-request=/tmp/request.bin
```

The plugin runs exactly as if protoc had invoked it - same file descriptors, same options, same parameters. No protoc needed.

## Adding --replay-request to Your Own Plugin

Any `protoc-gen-xxxx` plugin can support `--replay-request`. The idea is simple: instead of reading stdin, read the captured file and feed those bytes to your normal processing logic.

### For `google.golang.org/protobuf/compiler/protogen`-based plugins

Parse the flag from `os.Args`, read the file, unmarshal it as a `CodeGeneratorRequest`, and run your generation logic. Here's a minimal pattern:

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	// Check for --replay-request flag
	replayRequest := ""
	for i, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--replay-request=") {
			replayRequest = strings.TrimPrefix(arg, "--replay-request=")
			// Remove this arg so it doesn't confuse the flag parser
			os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
			break
		}
	}

	if replayRequest != "" {
		if err := runFromFile(replayRequest); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Normal mode: run as protoc plugin
	protogen.Options{}.Run(func(gen *protogen.Plugin) error {
		// ... your generation logic ...
		return nil
	})
}

func runFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read request file: %w", err)
	}

	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Feed the request to protogen
	opts := protogen.Options{}
	plugin, err := opts.New(req)
	if err != nil {
		return fmt.Errorf("failed to create plugin: %w", err)
	}

	// ... run your generation logic on plugin ...

	// Write the response to stdout
	resp := plugin.Response()
	out, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	os.Stdout.Write(out)
	return nil
}
```

See `protoc-gen-om-jsonschema/main.go` for a real-world example.

### For `bufbuild/protoplugin`-based plugins

With protoplugin, you can replace `os.Stdin` in the `Env` struct with a `bytes.Reader` containing the captured data:

```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bufbuild/protoplugin"
)

func main() {
	// Check for --replay-request flag
	replayRequest := ""
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--replay-request=") {
			replayRequest = strings.TrimPrefix(arg, "--replay-request=")
			break
		}
	}

	osEnv := protoplugin.Env{
		Args:    os.Args[1:],
		Environ: os.Environ(),
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}

	if replayRequest != "" {
		data, err := os.ReadFile(replayRequest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read replay request file: %v\n", err)
			os.Exit(1)
		}
		osEnv.Stdin = bytes.NewReader(data)
	}

	ctx := context.Background()
	if err := protoplugin.Run(ctx, osEnv, protoplugin.HandlerFunc(handle)); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

See `protoc-gen-open-models/main.go` for a real-world example.

## IDE Debugging

The whole point of `--replay-request` is to debug plugins in your IDE. Once you have a captured `request.bin`, configure your IDE to run the plugin binary with the flag.

### VS Code

Add to `.vscode/launch.json`:

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

### GoLand / IntelliJ

1. Create a "Go Build" run configuration
2. Set "Program arguments" to `--replay-request=/tmp/request.bin`
3. Set breakpoints and run in debug mode

You can now set breakpoints, step through code, and inspect all the proto descriptors, file structures, and generated output exactly as they would appear during a real protoc run.
