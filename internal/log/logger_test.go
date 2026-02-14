package log

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helixml/kodit/internal/config"
)

func TestNewLogger_TextFormat(t *testing.T) {
	cfg := config.NewAppConfigWithOptions(
		config.WithLogLevel("INFO"),
		config.WithLogFormat(config.LogFormatPretty),
	)

	logger := NewLogger(cfg)

	if logger == nil {
		t.Fatal("NewLogger should not return nil")
	}
	if logger.Slog() == nil {
		t.Error("Slog() should not return nil")
	}
}

func TestNewLogger_JSONFormat(t *testing.T) {
	cfg := config.NewAppConfigWithOptions(
		config.WithLogLevel("DEBUG"),
		config.WithLogFormat(config.LogFormatJSON),
	)

	logger := NewLogger(cfg)

	if logger == nil {
		t.Fatal("NewLogger should not return nil")
	}
}

func TestLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, config.LogFormatJSON, "DEBUG")

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 4 {
		t.Errorf("expected 4 log lines, got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var data map[string]any
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, config.LogFormatJSON, "INFO")

	loggerWithComponent := logger.With("component", "test")
	loggerWithComponent.Info("test message")

	var data map[string]any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if data["component"] != "test" {
		t.Errorf("expected component=test, got %v", data["component"])
	}
}

func TestLogger_WithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, config.LogFormatJSON, "INFO")

	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "corr-123")
	ctx = WithRequestID(ctx, "req-456")

	logger.InfoContext(ctx, "test message")

	var data map[string]any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if data["correlation_id"] != "corr-123" {
		t.Errorf("expected correlation_id=corr-123, got %v", data["correlation_id"])
	}
	if data["request_id"] != "req-456" {
		t.Errorf("expected request_id=req-456, got %v", data["request_id"])
	}
}

func TestWithCorrelationID(t *testing.T) {
	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "test-corr-id")

	if CorrelationID(ctx) != "test-corr-id" {
		t.Errorf("CorrelationID() = %v, want 'test-corr-id'", CorrelationID(ctx))
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-req-id")

	if RequestID(ctx) != "test-req-id" {
		t.Errorf("RequestID() = %v, want 'test-req-id'", RequestID(ctx))
	}
}

func TestCorrelationID_NotSet(t *testing.T) {
	ctx := context.Background()
	if CorrelationID(ctx) != "" {
		t.Errorf("CorrelationID() should be empty when not set")
	}
}

func TestRequestID_NotSet(t *testing.T) {
	ctx := context.Background()
	if RequestID(ctx) != "" {
		t.Errorf("RequestID() should be empty when not set")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"DEBUG", "DEBUG"},
		{"debug", "DEBUG"},
		{"INFO", "INFO"},
		{"info", "INFO"},
		{"WARN", "WARN"},
		{"warn", "WARN"},
		{"WARNING", "WARN"},
		{"ERROR", "ERROR"},
		{"error", "ERROR"},
		{"unknown", "INFO"}, // defaults to INFO
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLevel(tt.input)
			if level.String() != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, level.String(), tt.expected)
			}
		})
	}
}

func TestLogger_FiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, config.LogFormatJSON, "WARN")

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should only have WARN and ERROR
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d: %s", len(lines), output)
	}
}

func TestDefault(t *testing.T) {
	logger := Default()
	if logger == nil {
		t.Error("Default() should not return nil")
	}
}

func TestConfigure(t *testing.T) {
	cfg := config.NewAppConfigWithOptions(
		config.WithLogLevel("DEBUG"),
		config.WithLogFormat(config.LogFormatJSON),
	)

	logger := Configure(cfg)

	if logger == nil {
		t.Error("Configure() should not return nil")
	}
	if Default() != logger {
		t.Error("Configure() should set the default logger")
	}
}

func TestLogger_WithContext_NoContextValues(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, config.LogFormatJSON, "INFO")

	// Empty context - should not add extra fields
	ctx := context.Background()
	logger.InfoContext(ctx, "test message")

	var data map[string]any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should not have correlation_id or request_id
	if _, ok := data["correlation_id"]; ok {
		t.Error("should not have correlation_id when not set")
	}
	if _, ok := data["request_id"]; ok {
		t.Error("should not have request_id when not set")
	}
}
