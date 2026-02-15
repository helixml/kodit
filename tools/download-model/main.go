// Build-time tool that downloads the jina-embeddings-v2-base-code model
// to infrastructure/provider/models/ for static embedding via //go:embed.
//
// Usage: go run ./tools/download-model [dest]
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/knights-analytics/hugot"
)

func main() {
	dest := "infrastructure/provider/models"
	if len(os.Args) > 1 {
		dest = os.Args[1]
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Downloading model to %s...\n", dest)

	opts := hugot.NewDownloadOptions()
	opts.OnnxFilePath = "onnx/model.onnx"

	var modelPath string
	var err error
	delay := 2 * time.Second
	for i := 0; i < 4; i++ {
		if i > 0 {
			fmt.Fprintf(os.Stderr, "retry in %s: %v\n", delay, err)
			time.Sleep(delay)
			delay *= 2
		}
		if modelPath, err = hugot.DownloadModel("jinaai/jina-embeddings-v2-base-code", dest, opts); err == nil {
			break
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "download model: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Model downloaded to %s\n", modelPath)
}
