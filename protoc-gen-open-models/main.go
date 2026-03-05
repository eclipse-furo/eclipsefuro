package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bufbuild/protoplugin"
	"github.com/eclipse-furo/eclipsefuro/protoc-gen-open-models/pkg/generator"
)

const version = "1.48.0"

func main() {
	// Check for --replay-request flag to replay a previously captured request
	// (from protoc-gen-debugfile) without needing protoc.
	replayRequest := ""
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--replay-request=") {
			replayRequest = strings.TrimPrefix(arg, "--replay-request=")
			break
		}
	}

	// osEnv is the os-based Env used in Main.
	var osEnv = protoplugin.Env{
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

	ctx, cancel := context.WithCancel(context.Background())
	if err := protoplugin.Run(ctx, osEnv, protoplugin.HandlerFunc(handle), protoplugin.WithVersion(version)); err != nil {
		exitError := &exec.ExitError{}
		if errors.As(err, &exitError) {
			cancel()
			// Swallow error message - it was printed via os.Stderr redirection.
			os.Exit(exitError.ExitCode())
		}
		if errString := err.Error(); errString != "" {
			_, _ = fmt.Fprintln(os.Stderr, errString)
		}
		cancel()
		os.Exit(1)
	}

	cancel()
}

func handle(
	_ context.Context,
	_ protoplugin.PluginEnv,
	responseWriter protoplugin.ResponseWriter,
	request protoplugin.Request,
) error {
	// Set the flag indicating that we support proto3 optionals. We don't even use them in this
	// plugin, but protoc will error if it encounters a proto3 file with an optional but the
	// plugin has not indicated it will support it.
	responseWriter.SetFeatureProto3Optional()
	generator.GenerateAll(responseWriter, request)

	return nil
}
