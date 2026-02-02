// Package e2e provides end-to-end tests for the Kodit API server.
package e2e_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}
