// Package log provides structured logging with correlation IDs.
package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/helixml/kodit/internal/config"
)

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

// Context keys for logging.
const (
	CorrelationIDKey ContextKey = "correlation_id"
	RequestIDKey     ContextKey = "request_id"
)

// Logger wraps slog.Logger with convenience methods.
type Logger struct {
	handler slog.Handler
	logger  *slog.Logger
}

// NewLogger creates a new Logger based on configuration.
func NewLogger(cfg config.AppConfig) *Logger {
	level := parseLevel(cfg.LogLevel())

	var handler slog.Handler
	switch cfg.LogFormat() {
	case config.LogFormatJSON:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	default:
		handler = newTerminalHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	return &Logger{
		handler: handler,
		logger:  slog.New(handler),
	}
}

// NewLoggerWithWriter creates a Logger that writes to the specified writer.
func NewLoggerWithWriter(w io.Writer, format config.LogFormat, level string) *Logger {
	lvl := parseLevel(level)

	var handler slog.Handler
	switch format {
	case config.LogFormatJSON:
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})
	default:
		handler = newTerminalHandler(w, &slog.HandlerOptions{Level: lvl})
	}

	return &Logger{
		handler: handler,
		logger:  slog.New(handler),
	}
}

func parseLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Handler returns the underlying slog.Handler.
func (l *Logger) Handler() slog.Handler {
	return l.handler
}

// Slog returns the underlying slog.Logger.
func (l *Logger) Slog() *slog.Logger {
	return l.logger
}

// With returns a new Logger with additional attributes.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		handler: l.handler,
		logger:  l.logger.With(args...),
	}
}

// WithContext returns a logger with context values (correlation ID, request ID).
func (l *Logger) WithContext(ctx context.Context) *Logger {
	attrs := make([]any, 0, 4)

	if corrID, ok := ctx.Value(CorrelationIDKey).(string); ok && corrID != "" {
		attrs = append(attrs, "correlation_id", corrID)
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		attrs = append(attrs, "request_id", reqID)
	}

	if len(attrs) == 0 {
		return l
	}
	return l.With(attrs...)
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// DebugContext logs at debug level with context.
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).logger.Debug(msg, args...)
}

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// InfoContext logs at info level with context.
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).logger.Info(msg, args...)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// WarnContext logs at warn level with context.
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).logger.Warn(msg, args...)
}

// Error logs at error level.
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// ErrorContext logs at error level with context.
func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.WithContext(ctx).logger.Error(msg, args...)
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

// SetDefault sets the global default slog logger.
func (l *Logger) SetDefault() {
	slog.SetDefault(l.logger)
}

// defaultLogger is the package-level default logger.
var defaultLogger = &Logger{
	handler: newTerminalHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
	logger:  slog.New(newTerminalHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
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
