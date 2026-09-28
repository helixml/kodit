package extraction_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/helixml/kodit/infrastructure/extraction"
)

func TestSourceText_PlainUTF8Unchanged(t *testing.T) {
	src := extraction.NewSourceText()
	result, err := src.Text([]byte("package main\n\nfunc main() {}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "package main\n\nfunc main() {}\n" {
		t.Fatalf("expected content unchanged, got %q", result)
	}
}

func TestSourceText_BinaryContentReturnsEmpty(t *testing.T) {
	src := extraction.NewSourceText()
	result, err := src.Text([]byte("abc\x00def"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty string for binary content, got %q", result)
	}
}

func TestSourceText_NonUTF8BytesAreSanitized(t *testing.T) {
	// 0xe4, 0xf6, 0xfc are 'ä', 'ö', 'ü' in Windows-1252/Latin-1 -- not valid UTF-8 on their own.
	// Matches the real bytes seen in CBST-2721's Postgres error (SQLSTATE 22021).
	latin1 := []byte("Kommentar: gr\xf6\xdfer als \xe4hnlich")

	src := extraction.NewSourceText()
	result, err := src.Text(latin1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.ValidString(result) {
		t.Fatalf("expected sanitized result to be valid UTF-8, got %q", result)
	}
	if !strings.Contains(result, "Kommentar: gr") {
		t.Fatalf("expected valid ASCII portions preserved, got %q", result)
	}
	if !strings.ContainsRune(result, '�') {
		t.Fatalf("expected invalid bytes replaced with U+FFFD, got %q", result)
	}
}
