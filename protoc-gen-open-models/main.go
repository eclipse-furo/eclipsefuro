package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/bufbuild/protoplugin"
	"io"
	"log"
	"os"
	"os/exec"
	"protoc-gen-open-models/pkg/generator"
)

const version = "0.0.1"

func main() {
	const debugMode = false

	// osEnv is the os-based Env used in Main.
	var osEnv = protoplugin.Env{
		Args:    os.Args[1:],
		Environ: os.Environ(),
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}

	if debugMode {
		data, err := os.ReadFile("protocdata")
		if err != nil {
			// debug file does not exist, read from stream and write file
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				log.Fatal(err)
			}
			os.WriteFile("protocdata", data, 0644)
		}
		osEnv = protoplugin.Env{
			Args:    os.Args[1:],
			Environ: os.Environ(),
			Stdin:   bytes.NewReader(data),
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
		}

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
