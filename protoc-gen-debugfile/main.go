// Package main implements a minimal protoc plugin (protoc-gen-debugfile)
// that captures the raw CodeGeneratorRequest to a file for later replay.
//
// Usage:
//
//	protoc \
//	  --debugfile_out=. \
//	  --debugfile_opt=/tmp/request.bin \
//	  --om-jsonschema_out=./schema \
//	  -I./proto_dependencies -I./proto \
//	  $(find proto -iname "*.proto")
//
// The captured file can then be replayed with other plugins:
//
//	./protoc-gen-om-jsonschema --replay-request=/tmp/request.bin
//	./protoc-gen-open-models --replay-request=/tmp/request.bin
package main

import (
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

const version = "1.0.0"

func main() {
	// Read raw stdin bytes (the CodeGeneratorRequest from protoc)
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-debugfile: failed to read stdin: %v\n", err)
		os.Exit(1)
	}

	// Unmarshal to extract the Parameter field (the output file path)
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(input, req); err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-debugfile: failed to unmarshal request: %v\n", err)
		os.Exit(1)
	}

	// The parameter is the output file path, passed via --debugfile_opt=<path>
	outputPath := req.GetParameter()
	if outputPath == "" {
		fmt.Fprintf(os.Stderr, "protoc-gen-debugfile: no output path specified. Use --debugfile_opt=<path>\n")
		os.Exit(1)
	}

	// Write the raw request bytes to the specified file
	if err := os.WriteFile(outputPath, input, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-debugfile: failed to write %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "protoc-gen-debugfile: saved CodeGeneratorRequest (%d bytes) to %s\n", len(input), outputPath)

	// Return an empty valid CodeGeneratorResponse on stdout
	resp := &pluginpb.CodeGeneratorResponse{}
	out, err := proto.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-debugfile: failed to marshal response: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
}
