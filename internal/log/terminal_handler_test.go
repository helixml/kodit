package log

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestTerminalHandler_Format(t *testing.T) {
	var buf bytes.Buffer
	h := newTerminalHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(h)

	ts := time.Date(2026, 1, 15, 10, 30, 45, 123000000, time.UTC)
	r := slog.NewRecord(ts, slog.LevelInfo, "server started", 0)
	r.AddAttrs(slog.String("port", "8080"))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "10:30:45.123") {
		t.Errorf("expected timestamp, got: %s", output)
	}
	if !strings.Contains(output, "INF") {
		t.Errorf("expected INF level, got: %s", output)
	}
	if !strings.Contains(output, "server started") {
		t.Errorf("expected message, got: %s", output)
	}
	if !strings.Contains(output, "port=") {
		t.Errorf("expected port attr, got: %s", output)
	}
	if !strings.Contains(output, "8080") {
		t.Errorf("expected port value, got: %s", output)
	}

	// Verify it uses the logger interface correctly
	buf.Reset()
	logger.Info("test")
	if buf.Len() == 0 {
		t.Error("expected output from logger")
	}
}

func TestTerminalHandler_Levels(t *testing.T) {
	tests := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelDebug, "DBG"},
		{slog.LevelInfo, "INF"},
		{slog.LevelWarn, "WRN"},
		{slog.LevelError, "ERR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			var buf bytes.Buffer
			h := newTerminalHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

			r := slog.NewRecord(time.Now(), tt.level, "msg", 0)
			if err := h.Handle(context.Background(), r); err != nil {
				t.Fatalf("Handle() error: %v", err)
			}

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected %s in output, got: %s", tt.expected, buf.String())
			}
		})
	}
}

func TestTerminalHandler_ColourCodes(t *testing.T) {
	var buf bytes.Buffer
	h := newTerminalHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	r := slog.NewRecord(time.Now(), slog.LevelError, "fail", 0)
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, ansiRed) {
		t.Error("expected red colour for ERROR level")
	}
	if !strings.Contains(output, ansiReset) {
		t.Error("expected reset code")
	}
	if !strings.Contains(output, ansiBold) {
		t.Error("expected bold for message")
	}
}

func TestTerminalHandler_Enabled(t *testing.T) {
	h := newTerminalHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})

	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("DEBUG should be disabled at WARN level")
	}
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("INFO should be disabled at WARN level")
	}
	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("WARN should be enabled at WARN level")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("ERROR should be enabled at WARN level")
	}
}

func TestTerminalHandler_FiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	h := newTerminalHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(h)

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d: %s", len(lines), output)
	}
}

func TestTerminalHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := newTerminalHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h2 := h.WithAttrs([]slog.Attr{slog.String("component", "api")})

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "request", 0)
	r.AddAttrs(slog.Int("status", 200))

	if err := h2.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "component=") {
		t.Errorf("expected component attr, got: %s", output)
	}
	if !strings.Contains(output, "api") {
		t.Errorf("expected api value, got: %s", output)
	}
	if !strings.Contains(output, "status=") {
		t.Errorf("expected status attr, got: %s", output)
	}
}

func TestTerminalHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	h := newTerminalHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h2 := h.WithGroup("http")

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "request", 0)
	r.AddAttrs(slog.String("method", "GET"))

	if err := h2.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "http.method=") {
		t.Errorf("expected grouped attr http.method, got: %s", output)
	}
}

func TestTerminalHandler_QuotesStringsWithSpaces(t *testing.T) {
	var buf bytes.Buffer
	h := newTerminalHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	r.AddAttrs(slog.String("error", "connection refused"))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"connection refused"`) {
		t.Errorf("expected quoted string value, got: %s", output)
	}
}

func TestTerminalHandler_DefaultLevel(t *testing.T) {
	h := newTerminalHandler(&bytes.Buffer{}, nil)

	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("default level should be INFO")
	}
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("DEBUG should be disabled at default INFO level")
	}
}

func TestTerminalHandler_EmptyGroup(t *testing.T) {
	var buf bytes.Buffer
	h := newTerminalHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	// WithGroup("") should return the same handler
	h2 := h.WithGroup("")
	if h2 != h {
		t.Error("WithGroup with empty string should return same handler")
	}
}

func TestTerminalHandler_GroupAttr(t *testing.T) {
	var buf bytes.Buffer
	h := newTerminalHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	r.AddAttrs(slog.Group("request",
		slog.String("method", "POST"),
		slog.Int("status", 201),
	))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "request.method=") {
		t.Errorf("expected grouped request.method, got: %s", output)
	}
	if !strings.Contains(output, "request.status=") {
		t.Errorf("expected grouped request.status, got: %s", output)
	}
}
