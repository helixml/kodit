// Package middleware provides HTTP middleware for the API server.
package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

const maxBodyLog = 4096

// Logging returns a middleware that logs HTTP requests.
func Logging(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			requestID := middleware.GetReqID(r.Context())
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			body := readableBody(r)

			defer func() {
				level := logLevel(ww.Status())
				event := logger.WithLevel(level).
					Str("request_id", requestID).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Int("status", ww.Status()).
					Int("bytes", ww.BytesWritten()).
					Int64("duration_ms", time.Since(start).Milliseconds()).
					Str("remote_addr", r.RemoteAddr)

				if q := r.URL.RawQuery; q != "" {
					event = event.Str("query", q)
				}

				if body != "" {
					event = event.Str("body", body)
				}

				event = addRequestHeaders(event, r)
				event = addResponseHeaders(event, ww)

				event.Msg("request completed")
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

func logLevel(status int) zerolog.Level {
	if status >= 500 {
		return zerolog.ErrorLevel
	}
	if status >= 400 {
		return zerolog.WarnLevel
	}
	return zerolog.InfoLevel
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

func addRequestHeaders(event *zerolog.Event, r *http.Request) *zerolog.Event {
	for _, name := range []string{
		"Content-Type",
		"Accept",
		"User-Agent",
		"X-Correlation-ID",
		"X-Forwarded-For",
		"Referer",
	} {
		if v := r.Header.Get(name); v != "" {
			event = event.Str("req_"+headerKey(name), v)
		}
	}
	return event
}

func addResponseHeaders(event *zerolog.Event, w http.ResponseWriter) *zerolog.Event {
	for _, name := range []string{
		"Content-Type",
		"X-Correlation-ID",
		"Cache-Control",
		"Location",
		"Retry-After",
	} {
		if v := w.Header().Get(name); v != "" {
			event = event.Str("resp_"+headerKey(name), v)
		}
	}
	return event
}

func headerKey(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "-", "_")
}
