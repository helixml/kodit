package main

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestORTPlatform(t *testing.T) {
	tests := []struct {
		goos, goarch, archivePath, library string
	}{
		{"linux", "amd64", "runtimes/linux-x64/native/libonnxruntime.so", "libonnxruntime.so"},
		{"linux", "arm64", "runtimes/linux-arm64/native/libonnxruntime.so", "libonnxruntime.so"},
		{"darwin", "arm64", "runtimes/osx-arm64/native/libonnxruntime.dylib", "libonnxruntime.dylib"},
	}
	for _, test := range tests {
		archivePath, library, err := ortPlatform(test.goos, test.goarch)
		if err != nil {
			t.Fatal(err)
		}
		if archivePath != test.archivePath || library != test.library {
			t.Fatalf("ortPlatform(%q, %q) = %q, %q; want %q, %q", test.goos, test.goarch, archivePath, library, test.archivePath, test.library)
		}
	}
	if _, _, err := ortPlatform("windows", "amd64"); err == nil {
		t.Fatal("ortPlatform(windows, amd64) succeeded")
	}
}

func TestExtractZipSelectsPlatform(t *testing.T) {
	packagePath := testZip(t, map[string]string{
		"runtimes/linux-x64/native/libonnxruntime.so":   "x64",
		"runtimes/linux-arm64/native/libonnxruntime.so": "arm64",
	})
	destDir := t.TempDir()
	if err := extractZip(packagePath, destDir, "runtimes/linux-arm64/native/libonnxruntime.so"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, "libonnxruntime.so"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "arm64" {
		t.Fatalf("extracted %q; want arm64", got)
	}
}

func TestExtractZipRequiresPlatform(t *testing.T) {
	packagePath := testZip(t, map[string]string{
		"runtimes/linux-x64/native/libonnxruntime.so": "x64",
	})
	if err := extractZip(packagePath, t.TempDir(), "runtimes/linux-arm64/native/libonnxruntime.so"); err == nil {
		t.Fatal("extractZip succeeded without the requested platform")
	}
}

func TestExtractZipRejectsSymlink(t *testing.T) {
	packagePath := filepath.Join(t.TempDir(), "runtime.nupkg")
	packageFile, err := os.Create(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(packageFile)
	header := &zip.FileHeader{Name: "runtimes/linux-arm64/native/libonnxruntime.so"}
	header.SetMode(os.ModeSymlink | 0o777)
	file, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("/tmp/not-a-library")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := packageFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(packagePath, t.TempDir(), header.Name); err == nil {
		t.Fatal("extractZip accepted a symlink")
	}
}

func TestTryFetchAndExtractNuGet(t *testing.T) {
	archivePath := "runtimes/linux-arm64/native/libonnxruntime.so"
	packagePath := testZip(t, map[string]string{archivePath: "runtime"})
	packageData, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(packageData)
	}))
	defer server.Close()

	destDir := t.TempDir()
	if err := tryFetchAndExtract(server.URL+"/runtime.nupkg", destDir, archivePath); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, "libonnxruntime.so"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "runtime" {
		t.Fatalf("extracted %q; want runtime", got)
	}
	tempPackages, err := filepath.Glob(filepath.Join(destDir, ".ort-*.nupkg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tempPackages) != 0 {
		t.Fatalf("temporary packages left behind: %v", tempPackages)
	}
}

func testZip(t *testing.T, files map[string]string) string {
	t.Helper()
	packagePath := filepath.Join(t.TempDir(), "runtime.nupkg")
	packageFile, err := os.Create(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(packageFile)
	for name, contents := range files {
		file, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := packageFile.Close(); err != nil {
		t.Fatal(err)
	}
	return packagePath
}
