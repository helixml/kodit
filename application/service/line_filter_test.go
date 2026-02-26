package service

import (
	"testing"
)

func TestNewLineFilter_EmptyParam(t *testing.T) {
	f, err := NewLineFilter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.Empty() {
		t.Error("expected empty filter")
	}
}

func TestNewLineFilter_SingleLine(t *testing.T) {
	f, err := NewLineFilter("L5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Empty() {
		t.Error("expected non-empty filter")
	}

	content := []byte("line1\nline2\nline3\nline4\nline5\nline6")
	result := string(f.Apply(content))
	if result != "line5" {
		t.Errorf("expected %q, got %q", "line5", result)
	}
}

func TestNewLineFilter_Range(t *testing.T) {
	f, err := NewLineFilter("L2-L4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := []byte("line1\nline2\nline3\nline4\nline5")
	result := string(f.Apply(content))
	if result != "line2\nline3\nline4" {
		t.Errorf("expected %q, got %q", "line2\nline3\nline4", result)
	}
}

func TestNewLineFilter_MultipleRanges(t *testing.T) {
	f, err := NewLineFilter("L1-L2,L4,L6-L7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := []byte("a\nb\nc\nd\ne\nf\ng\nh")
	result := string(f.Apply(content))
	expected := "a\nb\nd\nf\ng"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestNewLineFilter_PassThrough(t *testing.T) {
	f, err := NewLineFilter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := []byte("hello\nworld")
	result := f.Apply(content)
	if string(result) != string(content) {
		t.Errorf("pass-through should return original content")
	}
}

func TestNewLineFilter_BeyondEnd(t *testing.T) {
	f, err := NewLineFilter("L5-L100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := []byte("a\nb\nc\nd\ne\nf")
	result := string(f.Apply(content))
	expected := "e\nf"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestNewLineFilter_InvalidFormat(t *testing.T) {
	tests := []string{
		"abc",
		"L0",
		"L5-L2",
		"L-1",
	}
	for _, input := range tests {
		_, err := NewLineFilter(input)
		if err == nil {
			t.Errorf("expected error for input %q", input)
		}
	}
}
