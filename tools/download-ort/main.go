// Build-time tool that downloads the ONNX Runtime shared library and the
// HuggingFace tokenizers static library for the current platform.
//
// Required env: ORT_VERSION       (e.g. "1.23.2")
// Optional env: ORT_LIB_DIR       (default "./lib")
//               TOKENIZERS_VERSION (default "1.24.0")
//
// Usage: ORT_VERSION=1.23.2 go run ./tools/download-ort
package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	ortVersion := os.Getenv("ORT_VERSION")
	if ortVersion == "" {
		fmt.Fprintln(os.Stderr, "ORT_VERSION env var is required")
		os.Exit(1)
	}

	tokVersion := os.Getenv("TOKENIZERS_VERSION")
	if tokVersion == "" {
		tokVersion = "1.24.0"
	}

	destDir := os.Getenv("ORT_LIB_DIR")
	if destDir == "" {
		destDir = "./lib"
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create directory: %v\n", err)
		os.Exit(1)
	}

	if err := downloadORT(ortVersion, destDir); err != nil {
		fmt.Fprintf(os.Stderr, "ORT download failed: %v\n", err)
		os.Exit(1)
	}

	if err := downloadTokenizers(tokVersion, destDir); err != nil {
		fmt.Fprintf(os.Stderr, "tokenizers download failed: %v\n", err)
		os.Exit(1)
	}
}

func downloadORT(version, destDir string) error {
	archiveName, libraryName, err := ortPlatform(version)
	if err != nil {
		return err
	}

	destPath := filepath.Join(destDir, libraryName)
	if _, statErr := os.Stat(destPath); statErr == nil {
		fmt.Printf("ORT library already exists at %s, skipping\n", destPath)
		return nil
	}

	url := fmt.Sprintf(
		"https://github.com/microsoft/onnxruntime/releases/download/v%s/%s",
		version, archiveName,
	)

	fmt.Printf("Downloading ORT %s from %s\n", version, url)
	if err := fetchAndExtract(url, destDir, libraryName); err != nil {
		return err
	}

	fmt.Printf("ORT library installed to %s\n", destPath)
	return nil
}

func downloadTokenizers(version, destDir string) error {
	destPath := filepath.Join(destDir, "libtokenizers.a")
	if _, statErr := os.Stat(destPath); statErr == nil {
		fmt.Printf("tokenizers library already exists at %s, skipping\n", destPath)
		return nil
	}

	archiveName, err := tokenizersPlatform()
	if err != nil {
		return err
	}

	url := fmt.Sprintf(
		"https://github.com/daulet/tokenizers/releases/download/v%s/%s",
		version, archiveName,
	)

	fmt.Printf("Downloading tokenizers %s from %s\n", version, url)
	if err := fetchAndExtract(url, destDir, "libtokenizers.a"); err != nil {
		return err
	}

	fmt.Printf("tokenizers library installed to %s\n", destPath)
	return nil
}

func ortPlatform(version string) (archive string, library string, err error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	switch key {
	case "linux/amd64":
		return fmt.Sprintf("onnxruntime-linux-x64-%s.tgz", version), "libonnxruntime.so", nil
	case "linux/arm64":
		return fmt.Sprintf("onnxruntime-linux-aarch64-%s.tgz", version), "libonnxruntime.so", nil
	case "darwin/arm64":
		return fmt.Sprintf("onnxruntime-osx-arm64-%s.tgz", version), "libonnxruntime.dylib", nil
	case "darwin/amd64":
		return fmt.Sprintf("onnxruntime-osx-x86_64-%s.tgz", version), "libonnxruntime.dylib", nil
	default:
		return "", "", fmt.Errorf("no ORT archive for %s", key)
	}
}

func tokenizersPlatform() (string, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	switch key {
	case "linux/amd64":
		return "libtokenizers.linux-amd64.tar.gz", nil
	case "linux/arm64":
		return "libtokenizers.linux-arm64.tar.gz", nil
	case "darwin/arm64":
		return "libtokenizers.darwin-arm64.tar.gz", nil
	case "darwin/amd64":
		return "libtokenizers.darwin-x86_64.tar.gz", nil
	default:
		return "", fmt.Errorf("no tokenizers archive for %s", key)
	}
}

func fetchAndExtract(url, destDir, filename string) error {
	delay := 2 * time.Second
	var err error
	for i := 0; i < 4; i++ {
		if i > 0 {
			fmt.Fprintf(os.Stderr, "retry in %s: %v\n", delay, err)
			time.Sleep(delay)
			delay *= 2
		}
		if err = tryFetchAndExtract(url, destDir, filename); err == nil {
			return nil
		}
	}
	return err
}

func tryFetchAndExtract(url, destDir, filename string) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	return extractTgz(resp.Body, destDir, filename)
}

func extractTgz(body io.Reader, destDir, filename string) error {
	gz, err := gzip.NewReader(body)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close() //nolint:errcheck

	// Strip extension to match versioned variants like libonnxruntime.1.23.2.dylib
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Skip symlinks and directories — we want the real file
		if header.Typeflag != tar.TypeReg {
			continue
		}

		base := filepath.Base(header.Name)
		if base != filename && !strings.HasPrefix(base, nameWithoutExt+".") {
			continue
		}

		return writeFile(filepath.Join(destDir, filename), tr)
	}

	return fmt.Errorf("%s not found in archive", filename)
}

func writeFile(path string, src io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}

	return out.Close()
}
