//go:build ORT

package provider

import (
	"os"
	"path/filepath"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/options"
)

func newHugotSession() (*hugot.Session, error) {
	opts := []options.WithOption{}
	if ortLibDir := resolveORTLibDir(); ortLibDir != "" {
		opts = append(opts, options.WithOnnxLibraryPath(ortLibDir))
	}
	return hugot.NewORTSession(opts...)
}

// resolveORTLibDir finds the ONNX Runtime shared library directory.
// It checks ORT_LIB_DIR env var, then lib/ alongside the executable,
// then lib/ relative to the working directory.
// Returns empty string to let hugot use platform defaults.
func resolveORTLibDir() string {
	if dir := os.Getenv("ORT_LIB_DIR"); dir != "" {
		return dir
	}

	candidates := []string{}

	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "lib"))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "lib"))
	}

	for _, candidate := range candidates {
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate
		}
	}

	return ""
}
