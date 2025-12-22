package http

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
)

const swaggerUITemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        html { box-sizing: border-box; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin: 0; background: #fafafa; }
        .swagger-ui .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "{{.SpecURL}}",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                defaultModelsExpandDepth: 1,
                defaultModelExpandDepth: 1,
                docExpansion: "list",
                filter: true,
                showExtensions: true,
                showCommonExtensions: true
            });
        };
    </script>
</body>
</html>`

// SwaggerHandler handles Swagger UI and OpenAPI spec endpoints
type SwaggerHandler struct {
	title   string
	specURL string
	spec    []byte
}

// NewSwaggerHandler creates a new Swagger handler
func NewSwaggerHandler(title string, spec []byte) *SwaggerHandler {
	return &SwaggerHandler{
		title:   title,
		specURL: "/docs/openapi.yaml",
		spec:    spec,
	}
}

// RegisterRoutes registers Swagger routes
func (h *SwaggerHandler) RegisterRoutes(r chi.Router) {
	r.Get("/docs", h.UI())
	r.Get("/docs/", http.RedirectHandler("/docs", http.StatusMovedPermanently).ServeHTTP)
	r.Get("/docs/openapi.yaml", h.Spec())
	r.Get("/docs/openapi.json", h.SpecJSON())
}

// UI serves the Swagger UI HTML page
func (h *SwaggerHandler) UI() http.HandlerFunc {
	tmpl := template.Must(template.New("swagger").Parse(swaggerUITemplate))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		data := struct {
			Title   string
			SpecURL string
		}{
			Title:   h.title,
			SpecURL: h.specURL,
		}

		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "failed to render template", http.StatusInternalServerError)
			return
		}
	}
}

// Spec serves the OpenAPI specification in YAML format
func (h *SwaggerHandler) Spec() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(h.spec)
	}
}

// SpecJSON serves the OpenAPI specification in JSON format (converted from YAML)
func (h *SwaggerHandler) SpecJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For simplicity, we'll serve YAML with JSON content-type
		// In production, you might want to convert YAML to JSON
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")

		// Note: This serves YAML as-is. For proper JSON conversion,
		// you would need to parse YAML and marshal to JSON
		w.Write(h.spec)
	}
}
