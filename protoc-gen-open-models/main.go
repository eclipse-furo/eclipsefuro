package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/eclipse-furo/eclipsefuro/protoc-gen-open-models/pkg/generator"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

const version = "1.48.0"

func main() {
	replayRequest := ""
	for i, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--replay-request=") {
			replayRequest = strings.TrimPrefix(arg, "--replay-request=")
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

	runAsPlugin()
}

func runFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read request file: %w", err)
	}
	return processRequest(data, os.Stdout)
}

func processRequest(data []byte, output io.Writer) error {
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return fmt.Errorf("failed to unmarshal request: %w", err)
	}

	opts := protogen.Options{}
	plugin, err := opts.New(req)
	if err != nil {
		return fmt.Errorf("failed to create plugin: %w", err)
	}

	plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

	generator.GenerateAll(plugin)

	resp := plugin.Response()
	out, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	output.Write(out)
	return nil
}

func runAsPlugin() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read stdin: %v\n", err)
		os.Exit(1)
	}

	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(input, req); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal request: %v\n", err)
		os.Exit(1)
	}

	opts := protogen.Options{}
	plugin, err := opts.New(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create plugin: %v\n", err)
		os.Exit(1)
	}

	plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

	generator.GenerateAll(plugin)

	resp := plugin.Response()
	out, err := proto.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal response: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
}
