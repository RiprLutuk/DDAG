package gatewaysvc

import (
	"net/http"

	"github.com/ddag/ddag/internal/gateway"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/policy"
)

func (s *service) serveCatalogRoutes(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/openapi.json":
		httpx.WriteJSON(w, http.StatusOK, gateway.GenerateOpenAPISpec(s.visibleAPIs(r)))
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/openapi.yaml":
		b, err := gateway.GenerateOpenAPIYAML(s.visibleAPIs(r))
		if err != nil {
			httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to generate OpenAPI YAML")
			return true
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api-catalog":
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    s.visibleAPIs(r),
		})
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/docs":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(swaggerHTML))
		return true
	default:
		return false
	}
}

func (s *service) visibleAPIs(r *http.Request) []models.APIDefinition {
	apis := s.router.APIs()
	claims, err := s.authenticate(r)
	if err != nil {
		return apis
	}
	out := make([]models.APIDefinition, 0, len(apis))
	for _, api := range apis {
		if policy.HasScope(claims.Scope, api.RequiredScope) {
			out = append(out, api)
		}
	}
	return out
}

const swaggerHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>DDAG API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: '/openapi.json', dom_id: '#swagger-ui' });
  </script>
</body>
</html>`
