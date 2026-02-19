// Build-time tool that converts the st-codesearch-distilroberta-base model
// to ONNX format for static embedding via //go:embed.
//
// Requires uv (https://docs.astral.sh/uv/) and Python >=3.10.
//
// Usage: go run ./tools/download-model [dest]
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	dest := "infrastructure/provider/models/flax-sentence-embeddings_st-codesearch-distilroberta-base"
	if len(os.Args) > 1 {
		dest = os.Args[1]
	}

	// Skip if already converted
	if _, err := os.Stat(filepath.Join(dest, "tokenizer.json")); err == nil {
		if _, err := os.Stat(filepath.Join(dest, "onnx", "model.onnx")); err == nil {
			fmt.Printf("Model already present at %s\n", dest)
			return
		}
	}

	fmt.Printf("Converting model to %s...\n", dest)

	var err error
	delay := 2 * time.Second
	for i := range 4 {
		if i > 0 {
			fmt.Fprintf(os.Stderr, "retry in %s: %v\n", delay, err)
			time.Sleep(delay)
			delay *= 2
		}

		cmd := exec.Command("uv", "run", "tools/convert-model.py", dest)
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
