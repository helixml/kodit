package mcp

import "testing"

func TestFileURI_BasicPath(t *testing.T) {
	uri := NewFileURI(1, "abc123", "src/main.go")
	expected := "file://1/abc123/src/main.go"
	if uri.String() != expected {
		t.Errorf("expected %s, got %s", expected, uri.String())
	}
}

func TestFileURI_WithLineRange(t *testing.T) {
	uri := NewFileURI(1, "abc123", "src/main.go").WithLineRange(10, 25)
	expected := "file://1/abc123/src/main.go?lines=L10-L25&line_numbers=true"
	if uri.String() != expected {
		t.Errorf("expected %s, got %s", expected, uri.String())
	}
}

func TestFileURI_WithoutLineRange(t *testing.T) {
	uri := NewFileURI(1, "abc123", "src/main.go")
	got := uri.String()
	if containsStr(got, "?") {
		t.Errorf("expected no query params, got %s", got)
	}
}

func TestFileURI_NestedPath(t *testing.T) {
	uri := NewFileURI(1, "abc123", "pkg/api/v1/handler.go")
	expected := "file://1/abc123/pkg/api/v1/handler.go"
	if uri.String() != expected {
		t.Errorf("expected %s, got %s", expected, uri.String())
	}
}
