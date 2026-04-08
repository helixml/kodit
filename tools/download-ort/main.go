// Build-time tool that downloads the ONNX Runtime shared library and the
// HuggingFace tokenizers static library for the current platform.
//
// Optional env: ORT_VERSION        (defaults to the version pinned in .ort-version)
//
//	TOKENIZERS_VERSION (defaults to the version pinned in .tokenizers-version)
//	ORT_LIB_DIR        (default "./lib")
//
// Usage: go run github.com/helixml/kodit/tools/download-ort
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
	"runtime/debug"
	"strings"
	"time"
)

func main() {
	ortVersion := os.Getenv("ORT_VERSION")
	if ortVersion == "" {
		v, err := readVersionFile(".ort-version")
		if err != nil {
			fmt.Fprintf(os.Stderr, "ORT_VERSION not set and could not read .ort-version: %v\n", err)
			os.Exit(1)
		}
		ortVersion = v
	}

	tokVersion := os.Getenv("TOKENIZERS_VERSION")
	if tokVersion == "" {
		v, err := readVersionFile(".tokenizers-version")
		if err != nil {
			fmt.Fprintf(os.Stderr, "TOKENIZERS_VERSION not set and could not read .tokenizers-version: %v\n", err)
			os.Exit(1)
		}
		tokVersion = v
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
	stamp := filepath.Join(destDir, ".ort-version")
	if cached, readErr := os.ReadFile(stamp); readErr == nil && string(cached) == version {
		fmt.Printf("ORT %s already exists at %s, skipping\n", version, destPath)
		return nil
	}

	// Remove stale library from a previous version or corrupt download.
	_ = os.Remove(destPath)

	url := fmt.Sprintf(
		"https://github.com/microsoft/onnxruntime/releases/download/v%s/%s",
		version, archiveName,
	)

	fmt.Printf("Downloading ORT %s from %s\n", version, url)
	if err := fetchAndExtract(url, destDir, libraryName); err != nil {
		return err
	}

	if err := os.WriteFile(stamp, []byte(version), 0o644); err != nil {
		return fmt.Errorf("write version stamp: %w", err)
	}

	fmt.Printf("ORT library installed to %s\n", destPath)
	return nil
}

func downloadTokenizers(version, destDir string) error {
	destPath := filepath.Join(destDir, "libtokenizers.a")
	stamp := filepath.Join(destDir, ".tokenizers-version")
	if cached, readErr := os.ReadFile(stamp); readErr == nil && string(cached) == version {
		fmt.Printf("tokenizers %s already exists at %s, skipping\n", version, destPath)
		return nil
	}

	// Remove stale library from a previous version or corrupt download.
	_ = os.Remove(destPath)

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

	if err := os.WriteFile(stamp, []byte(version), 0o644); err != nil {
		return fmt.Errorf("write version stamp: %w", err)
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

	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Skip symlinks and directories — we want the real file.
		if header.Typeflag != tar.TypeReg {
			continue
		}

		base := filepath.Base(header.Name)
		if base == filename {
			return writeFile(filepath.Join(destDir, filename), tr)
		}

		// Match versioned variants like libonnxruntime.so.1.24.1 or
		// libonnxruntime.1.24.1.dylib, but reject unrelated files
		// like libonnxruntime.pc that share the same prefix.
		if strings.HasPrefix(base, nameWithoutExt+".") && strings.Contains(base, ext) {
			return writeFile(filepath.Join(destDir, filename), tr)
		}
	}

	return fmt.Errorf("%s not found in archive", filename)
}

func writeFile(path string, src io.Reader) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write %s: %w", path, err)
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close %s: %w", path, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename to %s: %w", path, err)
	}

	return nil
}

const modulePath = "github.com/helixml/kodit"

// readVersionFile locates a version file (e.g. ".ort-version") shipped with the
// kodit module. It first checks the working directory (covers local development),
// then falls back to the Go module cache (covers go run ...@version).
func readVersionFile(name string) (string, error) {
	// Local repo: version file in the current working directory.
	if b, err := os.ReadFile(name); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" {
			return v, nil
		}
	}

	// Module cache: find the kodit module directory via build info.
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("%s not found in working directory and build info unavailable", name)
	}

	modDir := moduleDir(info)
	if modDir == "" {
		return "", fmt.Errorf("%s not found in working directory and could not locate module cache path", name)
	}

	b, err := os.ReadFile(filepath.Join(modDir, name))
	if err != nil {
		return "", fmt.Errorf("read %s/%s: %w", modDir, name, err)
	}

	if v := strings.TrimSpace(string(b)); v != "" {
		return v, nil
	}

	return "", fmt.Errorf("%s is empty", name)
}

// moduleDir returns the on-disk directory for the kodit module. When the tool
// is the main module (local dev), it returns "" so the caller falls through to
// the working-directory check. When kodit appears as a dependency (go run ...@v),
// it constructs the module cache path.
func moduleDir(info *debug.BuildInfo) string {
	if info.Main.Path == modulePath && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return filepath.Join(goModCache(), info.Main.Path+"@"+info.Main.Version)
	}

	for _, dep := range info.Deps {
		if dep.Path == modulePath {
			return filepath.Join(goModCache(), dep.Path+"@"+dep.Version)
		}
	}

	return ""
}

func goModCache() string {
	if v := os.Getenv("GOMODCACHE"); v != "" {
		return v
	}
	if v := os.Getenv("GOPATH"); v != "" {
		return filepath.Join(v, "pkg", "mod")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "go", "pkg", "mod")
}
