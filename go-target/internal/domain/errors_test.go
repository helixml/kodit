package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrEmptySource(t *testing.T) {
	if ErrEmptySource == nil {
		t.Error("ErrEmptySource should not be nil")
	}

	// Verify error can be wrapped and unwrapped
	wrapped := fmt.Errorf("processing failed: %w", ErrEmptySource)
	if !errors.Is(wrapped, ErrEmptySource) {
		t.Error("wrapped error should match ErrEmptySource with errors.Is")
	}
}

func TestErrEmptySource_Message(t *testing.T) {
	if ErrEmptySource.Error() != "source is empty" {
		t.Errorf("ErrEmptySource message = %q, want 'source is empty'", ErrEmptySource.Error())
	}
}
