// Package api provides HTTP server and API documentation.
package api

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed openapi.json
var openapiSpec embed.FS

// SwaggerUIHTML returns the HTML template for Swagger UI.
func SwaggerUIHTML(specURL string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Kodit API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin:0; background: #fafafa; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" charset="UTF-8"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js" charset="UTF-8"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: "` + specURL + `",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
            window.ui = ui;
        };
    </script>
</body>
</html>`
}

// DocsRouter sets up documentation routes.
type DocsRouter struct {
	specURL string
}

// NewDocsRouter creates a new documentation router.
func NewDocsRouter(specURL string) *DocsRouter {
	return &DocsRouter{specURL: specURL}
}

// Routes returns the chi router for documentation endpoints.
func (d *DocsRouter) Routes() chi.Router {
	router := chi.NewRouter()

	// Serve Swagger UI HTML
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(SwaggerUIHTML(d.specURL)))
	})

	// Serve OpenAPI spec with the server URL rewritten to match the
	// incoming request so that Swagger UI "Try it out" works on any host.
	router.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, err := fs.ReadFile(openapiSpec, "openapi.json")
		if err != nil {
			http.Error(w, "Spec not found", http.StatusNotFound)
			return
		}
		scheme := "https"
		if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
			scheme = forwarded
		} else if r.TLS == nil {
			scheme = "http"
		}
		host := r.Host
		if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
			host = forwarded
		}
		serverURL := fmt.Sprintf("%s://%s/api/v1", scheme, host)
		data = bytes.ReplaceAll(data,
			[]byte(`"url": "//localhost:8080/api/v1"`),
			[]byte(fmt.Sprintf(`"url": "%s"`, serverURL)),
		)
		_, _ = w.Write(data)
	})

	return router
}
