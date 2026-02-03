// Package api provides HTTP server and API documentation.
package api

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed swagger.json
var swaggerSpec embed.FS

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

	// Serve OpenAPI spec
	router.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, err := fs.ReadFile(swaggerSpec, "swagger.json")
		if err != nil {
			http.Error(w, "Spec not found", http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	})

	return router
}
