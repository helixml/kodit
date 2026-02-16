package middleware

import (
	"errors"
	"fmt"
	"testing"
)

func TestAPIError(t *testing.T) {
	err := NewAPIError(404, "resource not found", nil)

	if err.Code() != 404 {
		t.Errorf("Code() = %v, want 404", err.Code())
	}
	if err.Message() != "resource not found" {
		t.Errorf("Message() = %v, want 'resource not found'", err.Message())
	}

	expected := "api error 404: resource not found"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestAPIError_WithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewAPIError(500, "internal error", cause)

	expected := "api error 500: internal error: underlying error"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}

	if err.Unwrap() != cause {
		t.Error("Unwrap() should return the cause")
	}
}

func TestAuthenticationError(t *testing.T) {
	err := NewAuthenticationError("invalid token")

	expected := "authentication failed: invalid token"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}

	// Should be matchable with errors.Is
	if !errors.Is(err, ErrAuthentication) {
		t.Error("AuthenticationError should match ErrAuthentication with errors.Is")
	}
}

func TestServerError(t *testing.T) {
	err := NewServerError(503, "service unavailable")

	if err.StatusCode() != 503 {
		t.Errorf("StatusCode() = %v, want 503", err.StatusCode())
	}
	if err.Message() != "service unavailable" {
		t.Errorf("Message() = %v, want 'service unavailable'", err.Message())
	}

	expected := "server error 503: service unavailable"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}

	// Should be matchable with errors.Is
	if !errors.Is(err, ErrServer) {
		t.Error("ServerError should match ErrServer with errors.Is")
	}
}

func TestErrors_CanBeWrapped(t *testing.T) {
	authErr := NewAuthenticationError("token expired")
	wrapped := fmt.Errorf("request failed: %w", authErr)

	if !errors.Is(wrapped, ErrAuthentication) {
		t.Error("wrapped AuthenticationError should still match ErrAuthentication")
	}

	// Should be able to extract the typed error
	var target *AuthenticationError
	if !errors.As(wrapped, &target) {
		t.Error("should be able to extract AuthenticationError with errors.As")
	}
}
