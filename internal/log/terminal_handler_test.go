package log

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestConsoleWriter_Format(t *testing.T) {
	var buf bytes.Buffer
	w := newConsoleWriter(&buf)
	logger := zerolog.New(w).With().Timestamp().Logger()

	logger.Info().Str("port", "8080").Msg("server started")

	output := buf.String()

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
}

func TestConsoleWriter_Levels(t *testing.T) {
	tests := []struct {
		level    zerolog.Level
		expected string
	}{
		{zerolog.DebugLevel, "DBG"},
		{zerolog.InfoLevel, "INF"},
		{zerolog.WarnLevel, "WRN"},
		{zerolog.ErrorLevel, "ERR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			var buf bytes.Buffer
			w := newConsoleWriter(&buf)
			logger := zerolog.New(w).Level(zerolog.DebugLevel)

			logger.WithLevel(tt.level).Msg("msg")

			if !strings.Contains(buf.String(), tt.expected) {
				t.Errorf("expected %s in output, got: %s", tt.expected, buf.String())
			}
		})
	}
}

func TestConsoleWriter_ColourCodes(t *testing.T) {
	var buf bytes.Buffer
	w := newConsoleWriter(&buf)
	logger := zerolog.New(w).Level(zerolog.DebugLevel)

	logger.Error().Msg("fail")

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

func TestConsoleWriter_FiltersByLevel(t *testing.T) {
	var buf bytes.Buffer
	w := newConsoleWriter(&buf)
	logger := zerolog.New(w).Level(zerolog.WarnLevel)

	logger.Debug().Msg("debug")
	logger.Info().Msg("info")
	logger.Warn().Msg("warn")
	logger.Error().Msg("error")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d: %s", len(lines), output)
	}
}

func TestConsoleWriter_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	w := newConsoleWriter(&buf)
	logger := zerolog.New(w).Level(zerolog.DebugLevel).With().Str("component", "api").Logger()

	logger.Info().Int("status", 200).Msg("request")

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
