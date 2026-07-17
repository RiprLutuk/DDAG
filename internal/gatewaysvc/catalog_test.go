package gatewaysvc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPISpecRequiresBearerToken(t *testing.T) {
	svc := &service{}
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()

	if handled := svc.serveCatalogRoutes(rec, req); !handled {
		t.Fatal("OpenAPI route was not handled")
	}
	if got := rec.Code; got != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", got, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestOpenAPISpecRejectsUnsupportedMethod(t *testing.T) {
	svc := &service{}
	req := httptest.NewRequest(http.MethodPost, "/openapi.json", nil)
	rec := httptest.NewRecorder()

	if handled := svc.serveCatalogRoutes(rec, req); handled {
		t.Fatal("POST /openapi.json must not be handled as an OpenAPI route")
	}
}

func TestQueryPreflightAdvertisesRFC10008Method(t *testing.T) {
	svc := &service{}
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/search", nil)
	req.Header.Set("Access-Control-Request-Method", "QUERY")
	rec := httptest.NewRecorder()

	svc.serve(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "QUERY") {
		t.Fatalf("Access-Control-Allow-Methods = %q, want QUERY", got)
	}
}

func TestSwaggerDocsProvideSessionScopedBearerTokenGate(t *testing.T) {
	svc := &service{}
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	rec := httptest.NewRecorder()

	if handled := svc.serveCatalogRoutes(rec, req); !handled {
		t.Fatal("Swagger docs route was not handled")
	}
	body := rec.Body.String()
	if strings.Contains(body, `persistAuthorization: ***`) {
		t.Fatal("Swagger docs contain an invalid persistAuthorization placeholder")
	}
	for _, want := range []string{
		`id="token-gate"`,
		`sessionStorage.getItem('ddag.swagger.token')`,
		`requestInterceptor`,
		`Authorization = 'Bearer ' + token`,
		`persistAuthorization: false,`,
		`<link rel="icon" type="image/svg+xml" href="/favicon.svg">`,
		`<meta name="theme-color" content="#070b14">`,
		`/* --- DDAG dark theme overrides for Swagger UI --- */`,
		`#swagger-ui .swagger-ui .opblock.opblock-query .opblock-summary-method { background: #0ea5e9; }`,
		`#swagger-ui { background: var(--docs-bg); }`,
		`id="theme-toggle"`,
		`<img src="/favicon.svg" alt="DDAG">`,
		`--docs-mono: ui-monospace`,
		`font-family: var(--docs-mono)`,
		`role="switch"`,
		`aria-checked="false"`,
		`localStorage.getItem('ddag.docs.theme')`,
		`document.documentElement.dataset.theme`,
		`aria-label="Switch to light theme"`,
		`[data-theme="light"]`,
		`<form id="token-form" novalidate>`,
		`if (!token) {`,
		`error.textContent = 'Enter an OAuth2 access token.'`,
		`input.classList.add('invalid')`,
		`req.method = "QUERY"`,
		`isQueryRequest(paths, req.url)`,
		`pathMatches(pattern, pathname)`,
		`segment.startsWith('{')`,
		`applyQueryBadges`,
		`observeQueryBadges`,
		`data-ddag-method`,
		`"x-ddag-http-method"`,
		`new URL(requestURL, window.location.origin).pathname`,
		`spec: contract`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("Swagger docs missing %q", want)
		}
	}
}
