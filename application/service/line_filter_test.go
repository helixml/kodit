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
	expected := "a\nb\n...\nd\n...\nf\ng"
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

func TestLineFilter_ApplyWithLineNumbers(t *testing.T) {
	content := []byte("alpha\nbeta\ngamma\ndelta")

	f, err := NewLineFilter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := string(f.ApplyWithLineNumbers(content))
	expected := "1\talpha\n2\tbeta\n3\tgamma\n4\tdelta"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestLineFilter_ApplyWithLineNumbers_Range(t *testing.T) {
	content := []byte("alpha\nbeta\ngamma\ndelta\nepsilon")

	f, err := NewLineFilter("L2-L4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := string(f.ApplyWithLineNumbers(content))
	// Line numbers should reflect the original positions, not 1-based within the filtered output.
	expected := "2\tbeta\n3\tgamma\n4\tdelta"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestLineFilter_ApplyWithLineNumbers_MultipleRanges(t *testing.T) {
	content := []byte("a\nb\nc\nd\ne\nf\ng")

	f, err := NewLineFilter("L1-L2,L5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := string(f.ApplyWithLineNumbers(content))
	expected := "1\ta\n2\tb\n...\n5\te"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestLineFilter_Apply_ContiguousRanges_NoEllipsis(t *testing.T) {
	f, err := NewLineFilter("L1-L3,L4-L5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := []byte("a\nb\nc\nd\ne\nf")
	result := string(f.Apply(content))
	// Ranges are contiguous (L3 → L4), so no ellipsis between them.
	expected := "a\nb\nc\nd\ne"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestLineFilter_ApplyWithLineNumbers_ContiguousRanges_NoEllipsis(t *testing.T) {
	f, err := NewLineFilter("L1-L2,L3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := []byte("a\nb\nc\nd")
	result := string(f.ApplyWithLineNumbers(content))
	// L2 → L3 is contiguous, no ellipsis.
	expected := "1\ta\n2\tb\n3\tc"
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
