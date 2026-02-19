package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// TerminalHandler formats log records as coloured terminal output.
//
// Output format:
//
//	15:04:05.000 INF server started port=8080
type TerminalHandler struct {
	writer io.Writer
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
	mu     *sync.Mutex
}

func newTerminalHandler(w io.Writer, opts *slog.HandlerOptions) *TerminalHandler {
	var level slog.Leveler
	if opts != nil && opts.Level != nil {
		level = opts.Level
	} else {
		level = slog.LevelInfo
	}
	return &TerminalHandler{
		writer: w,
		level:  level,
		mu:     &sync.Mutex{},
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *TerminalHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle formats a log record as coloured terminal output and writes it.
func (h *TerminalHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer
	buf.Grow(256)

	ts := r.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	buf.WriteString(ansiDim)
	buf.WriteString(ts.Format("15:04:05.000"))
	buf.WriteString(ansiReset)
	buf.WriteByte(' ')

	color, label := levelStyle(r.Level)
	buf.WriteString(color)
	buf.WriteString(label)
	buf.WriteString(ansiReset)
	buf.WriteByte(' ')

	buf.WriteString(ansiBold)
	buf.WriteString(r.Message)
	buf.WriteString(ansiReset)

	for _, a := range h.attrs {
		appendAttr(&buf, a, h.groups)
	}

	r.Attrs(func(a slog.Attr) bool {
		appendAttr(&buf, a, h.groups)
		return true
	})

	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.writer.Write(buf.Bytes())
	return err
}

// WithAttrs returns a new handler whose attributes consist of both the
// existing attributes and attrs.
func (h *TerminalHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, len(h.attrs), len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	merged = append(merged, attrs...)
	return &TerminalHandler{
		writer: h.writer,
		level:  h.level,
		attrs:  merged,
		groups: h.groups,
		mu:     h.mu,
	}
}

// WithGroup returns a new handler with the given group name prepended to
// subsequent attribute keys.
func (h *TerminalHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	extended := make([]string, len(h.groups)+1)
	copy(extended, h.groups)
	extended[len(h.groups)] = name
	return &TerminalHandler{
		writer: h.writer,
		level:  h.level,
		attrs:  h.attrs,
		groups: extended,
		mu:     h.mu,
	}
}

func levelStyle(level slog.Level) (string, string) {
	switch {
	case level < slog.LevelInfo:
		return ansiCyan, "DBG"
	case level < slog.LevelWarn:
		return ansiGreen, "INF"
	case level < slog.LevelError:
		return ansiYellow, "WRN"
	default:
		return ansiRed, "ERR"
	}
}

func appendAttr(buf *bytes.Buffer, a slog.Attr, groups []string) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}

	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		var prefix []string
		if a.Key != "" {
			prefix = make([]string, len(groups)+1)
			copy(prefix, groups)
			prefix[len(groups)] = a.Key
		} else {
			prefix = groups
		}
		for _, ga := range attrs {
			appendAttr(buf, ga, prefix)
		}
		return
	}

	buf.WriteByte(' ')
	buf.WriteString(ansiDim)
	for _, g := range groups {
		buf.WriteString(g)
		buf.WriteByte('.')
	}
	buf.WriteString(a.Key)
	buf.WriteByte('=')
	buf.WriteString(ansiReset)
	buf.WriteString(formatAttrValue(a.Value))
}

func formatAttrValue(v slog.Value) string {
	if v.Kind() == slog.KindString {
		s := v.String()
		if strings.ContainsAny(s, " \t\n\"\\") {
			return fmt.Sprintf("%q", s)
		}
		return s
	}
	return v.String()
}
