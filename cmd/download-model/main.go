// Standalone tool that converts the st-codesearch-distilroberta-base model
// to ONNX format for hugot embedding.
//
// The Python conversion script is embedded in the binary so this command
// works when installed via `go install`.
//
// Requires uv (https://docs.astral.sh/uv/) and Python >=3.10.
//
// Usage: download-model <dest>
package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

//go:embed convert-model.py
var script []byte

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: download-model <dest>")
		os.Exit(1)
	}
	dest := os.Args[1]

	// Skip if already converted.
	if _, err := os.Stat(filepath.Join(dest, "tokenizer.json")); err == nil {
		if _, err := os.Stat(filepath.Join(dest, "onnx", "model.onnx")); err == nil {
			fmt.Printf("Model already present at %s\n", dest)
			return
		}
	}

	// Write embedded script to a temp file.
	tmp, err := os.CreateTemp("", "convert-model-*.py")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp file: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(script); err != nil {
		fmt.Fprintf(os.Stderr, "write temp file: %v\n", err)
		os.Exit(1)
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close temp file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Converting model to %s...\n", dest)

	delay := 2 * time.Second
	for i := range 4 {
		if i > 0 {
			fmt.Fprintf(os.Stderr, "retry in %s: %v\n", delay, err)
			time.Sleep(delay)
			delay *= 2
		}

		cmd := exec.Command("uv", "run", tmp.Name(), dest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err = cmd.Run(); err == nil {
			break
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "convert model: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Model ready at %s\n", dest)
}
