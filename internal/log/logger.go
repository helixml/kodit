// Package log provides structured logging with correlation IDs.
package log

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/helixml/kodit/internal/config"
)

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

// Context keys for logging.
const (
	CorrelationIDKey ContextKey = "correlation_id"
	RequestIDKey     ContextKey = "request_id"
)

// Logger wraps zerolog.Logger with convenience methods.
type Logger struct {
	logger zerolog.Logger
}

// NewLogger creates a new Logger based on configuration.
func NewLogger(cfg config.AppConfig) *Logger {
	level := parseLevel(cfg.LogLevel())

	var logger zerolog.Logger
	switch cfg.LogFormat() {
	case config.LogFormatJSON:
		logger = zerolog.New(os.Stdout).Level(level).With().Timestamp().Logger()
	default:
		w := newConsoleWriter(os.Stdout)
		logger = zerolog.New(w).Level(level).With().Timestamp().Logger()
	}

	return &Logger{logger: logger}
}

// NewLoggerWithWriter creates a Logger that writes to the specified writer.
func NewLoggerWithWriter(w io.Writer, format config.LogFormat, level string) *Logger {
	lvl := parseLevel(level)

	var logger zerolog.Logger
	switch format {
	case config.LogFormatJSON:
		logger = zerolog.New(w).Level(lvl).With().Timestamp().Logger()
	default:
		cw := newConsoleWriter(w)
		logger = zerolog.New(cw).Level(lvl).With().Timestamp().Logger()
	}

	return &Logger{logger: logger}
}

func parseLevel(level string) zerolog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return zerolog.DebugLevel
	case "WARN", "WARNING":
		return zerolog.WarnLevel
	case "ERROR":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// Zerolog returns the underlying zerolog.Logger.
func (l *Logger) Zerolog() zerolog.Logger {
	return l.logger
}

// With returns a new Logger with additional attributes.
func (l *Logger) With(args ...any) *Logger {
	ctx := l.logger.With()
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok {
			ctx = ctx.Interface(key, args[i+1])
		}
	}
	return &Logger{logger: ctx.Logger()}
}

// WithContext returns a logger with context values (correlation ID, request ID).
func (l *Logger) WithContext(ctx context.Context) *Logger {
	c := l.logger.With()
	added := false

	if corrID, ok := ctx.Value(CorrelationIDKey).(string); ok && corrID != "" {
		c = c.Str("correlation_id", corrID)
		added = true
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		c = c.Str("request_id", reqID)
		added = true
	}

	if !added {
		return l
	}
	return &Logger{logger: c.Logger()}
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) {
	addPairs(l.logger.Debug(), args).Msg(msg)
}

// DebugContext logs at debug level with context.
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	addPairs(l.WithContext(ctx).logger.Debug(), args).Msg(msg)
}

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) {
	addPairs(l.logger.Info(), args).Msg(msg)
}

// InfoContext logs at info level with context.
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	addPairs(l.WithContext(ctx).logger.Info(), args).Msg(msg)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, args ...any) {
	addPairs(l.logger.Warn(), args).Msg(msg)
}

// WarnContext logs at warn level with context.
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	addPairs(l.WithContext(ctx).logger.Warn(), args).Msg(msg)
}

// Error logs at error level.
func (l *Logger) Error(msg string, args ...any) {
	addPairs(l.logger.Error(), args).Msg(msg)
}

// ErrorContext logs at error level with context.
func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	addPairs(l.WithContext(ctx).logger.Error(), args).Msg(msg)
}

// addPairs adds key-value pairs from a variadic args list to a zerolog event.
func addPairs(event *zerolog.Event, args []any) *zerolog.Event {
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok {
			event = event.Interface(key, args[i+1])
		}
	}
	return event
}

// WithCorrelationID adds a correlation ID to the context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, id)
}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// CorrelationID extracts the correlation ID from context.
func CorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}

// RequestID extracts the request ID from context.
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// SetDefault sets the global default zerolog logger.
func (l *Logger) SetDefault() {
	zlog.Logger = l.logger
	zerolog.SetGlobalLevel(l.logger.GetLevel())
}

// defaultLogger is the package-level default logger.
var defaultLogger = &Logger{
	logger: zerolog.New(newConsoleWriter(os.Stdout)).Level(zerolog.InfoLevel).With().Timestamp().Logger(),
}

// Default returns the default logger.
func Default() *Logger {
	return defaultLogger
}

// SetDefaultLogger sets the package-level default logger.
func SetDefaultLogger(l *Logger) {
	defaultLogger = l
	l.SetDefault()
}

// Configure sets up logging based on configuration and sets as default.
func Configure(cfg config.AppConfig) *Logger {
	l := NewLogger(cfg)
	SetDefaultLogger(l)
	return l
}
