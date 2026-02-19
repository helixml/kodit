// Package middleware provides HTTP middleware for the API server.
package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

const maxBodyLog = 4096

// Logging returns a middleware that logs HTTP requests.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			requestID := middleware.GetReqID(r.Context())
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			body := readableBody(r)

			defer func() {
				attrs := []slog.Attr{
					slog.String("request_id", requestID),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", ww.Status()),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Int64("duration_ms", time.Since(start).Milliseconds()),
					slog.String("remote_addr", r.RemoteAddr),
				}

				if q := r.URL.RawQuery; q != "" {
					attrs = append(attrs, slog.String("query", q))
				}

				if body != "" {
					attrs = append(attrs, slog.String("body", body))
				}

				attrs = append(attrs, safeRequestHeaders(r)...)
				attrs = append(attrs, safeResponseHeaders(ww)...)

				level := logLevel(ww.Status())
				args := make([]any, len(attrs))
				for i, a := range attrs {
					args[i] = a
				}
				logger.Log(r.Context(), level, "request completed", args...)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

func logLevel(status int) slog.Level {
	if status >= 500 {
		return slog.LevelError
	}
	if status >= 400 {
		return slog.LevelWarn
	}
	return slog.LevelInfo
}

func readableBody(r *http.Request) string {
	if r.Body == nil || r.Body == http.NoBody {
		return ""
	}

	ct := r.Header.Get("Content-Type")
	if !isTextContent(ct) {
		return ""
	}

	buf, err := io.ReadAll(io.LimitReader(r.Body, maxBodyLog+1))
	if err != nil {
		return ""
	}
	// Replace the body so downstream handlers can still read it.
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf), r.Body))

	if len(buf) > maxBodyLog {
		return string(buf[:maxBodyLog]) + "...(truncated)"
	}
	return string(buf)
}

func isTextContent(ct string) bool {
	if ct == "" {
		return true // assume text if unset
	}
	ct = strings.ToLower(ct)
	for _, prefix := range []string{
		"application/json",
		"application/xml",
		"text/",
		"application/x-www-form-urlencoded",
		"multipart/form-data",
	} {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

func safeRequestHeaders(r *http.Request) []slog.Attr {
	var attrs []slog.Attr
	for _, name := range []string{
		"Content-Type",
		"Accept",
		"User-Agent",
		"X-Correlation-ID",
		"X-Forwarded-For",
		"Referer",
	} {
		if v := r.Header.Get(name); v != "" {
			attrs = append(attrs, slog.String("req_"+headerKey(name), v))
		}
	}
	return attrs
}

func safeResponseHeaders(w http.ResponseWriter) []slog.Attr {
	var attrs []slog.Attr
	for _, name := range []string{
		"Content-Type",
		"X-Correlation-ID",
		"Cache-Control",
		"Location",
		"Retry-After",
	} {
		if v := w.Header().Get(name); v != "" {
			attrs = append(attrs, slog.String("resp_"+headerKey(name), v))
		}
	}
	return attrs
}

func headerKey(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "-", "_")
}
