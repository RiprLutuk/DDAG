package gatewaysvc

import (
	"net/http"

	"github.com/ddag/ddag/internal/gateway"
	"github.com/ddag/ddag/internal/httpx"
	"github.com/ddag/ddag/internal/models"
	"github.com/ddag/ddag/internal/policy"
)

func (s *service) serveCatalogRoutes(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}

	switch r.URL.Path {
	case "/openapi.json", "/openapi.yaml", "/api-catalog":
		apis, apiErr := s.visibleAPIs(r)
		if apiErr != nil {
			httpx.Error(w, r, apiErr)
			return true
		}
		switch r.URL.Path {
		case "/openapi.json":
			httpx.WriteJSON(w, http.StatusOK, gateway.GenerateOpenAPISpec(apis))
		case "/openapi.yaml":
			b, err := gateway.GenerateOpenAPIYAML(apis)
			if err != nil {
				httpx.ErrorCode(w, r, httpx.CodeInternal, "failed to generate OpenAPI YAML")
				return true
			}
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(b)
		case "/api-catalog":
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "data": apis})
		}
		return true
	case "/docs":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(swaggerHTML))
		return true
	default:
		return false
	}
}

// visibleAPIs authenticates a caller and returns only APIs granted to that
// client and its token scopes. OpenAPI documents are generated at request time
// from this live route snapshot, so published metadata changes are reflected
// without a redeploy and inaccessible endpoint metadata is never disclosed.
func (s *service) visibleAPIs(r *http.Request) ([]models.APIDefinition, *httpx.APIError) {
	claims, apiErr := s.authenticate(r)
	if apiErr != nil {
		return nil, apiErr
	}
	client, err := s.clientByClientID(r.Context(), claims.ClientID)
	if err != nil || client.Status != "active" {
		return nil, httpx.NewError(httpx.CodeForbidden, "Client is not active")
	}
	apis := s.router.APIs()
	out := make([]models.APIDefinition, 0, len(apis))
	for _, api := range apis {
		if !policy.HasScope(claims.Scope, api.RequiredScope) {
			continue
		}
		allowed, err := s.clientHasAPIAccess(r.Context(), client.ID, api.ID)
		if err == nil && allowed {
			out = append(out, api)
		}
	}
	return out, nil
}

const swaggerHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>DDAG API Docs</title>
  <meta name="theme-color" content="#070b14">
  <link rel="icon" type="image/svg+xml" href="/favicon.svg">
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
  :root { color-scheme: dark; --docs-bg: #070b14; --docs-panel: #101827; --docs-panel-2: #152033; --docs-border: rgba(148,163,184,.16); --docs-text: #e6edf7; --docs-muted: #9aa8bd; --docs-faint: #64748b; --docs-primary: #f59e0b; --docs-primary-2: #d97706; --docs-accent: #6d72ff; --docs-info: #38bdf8; --docs-mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace; --docs-radius: 14px; --docs-radius-sm: 10px; --docs-shadow: 0 18px 58px rgba(0,0,0,.34); }
  :root[data-theme="light"] { color-scheme: light; --docs-bg: #f8fafc; --docs-panel: #ffffff; --docs-panel-2: #f1f5f9; --docs-border: rgba(100,116,139,.24); --docs-text: #0f172a; --docs-muted: #475569; --docs-faint: #64748b; --docs-primary: #d97706; --docs-primary-2: #b45309; --docs-accent: #575cf0; --docs-info: #0284c7; --docs-shadow: 0 18px 58px rgba(15,23,42,.12); }
  * { box-sizing: border-box; }
  body { margin: 0; background: var(--docs-bg); color: var(--docs-text); font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
  #docs-toolbar { position: sticky; z-index: 50; top: 0; display: flex; align-items: center; justify-content: space-between; gap: 16px; min-height: 64px; padding: 12px max(24px, calc((100% - 1440px) / 2)); background: color-mix(in srgb, var(--docs-bg) 88%, transparent); border-bottom: 1px solid var(--docs-border); backdrop-filter: blur(18px); }
  #docs-toolbar .brand { display: inline-flex; align-items: center; gap: 10px; color: var(--docs-text); font-size: 18px; font-weight: 800; letter-spacing: -.02em; }
  #docs-toolbar .brand-mark { width: 30px; height: 30px; }
  #docs-toolbar .brand-mark img { width: 100%; height: 100%; object-fit: contain; }
  #theme-toggle { position: relative; width: 42px; height: 24px; border: 1px solid var(--docs-border); border-radius: 999px; background: var(--docs-panel-2); cursor: pointer; appearance: none; -webkit-appearance: none; flex: 0 0 auto; transition: background .2s, border-color .2s; }
  #theme-toggle::before { content: ''; position: absolute; top: 3px; left: 3px; width: 16px; height: 16px; border-radius: 50%; background: var(--docs-primary); transition: transform .2s; }
  #theme-toggle::after { content: '☾'; position: absolute; top: 2px; left: 5px; width: 14px; height: 18px; color: #fff; font-size: 11px; text-align: center; line-height: 18px; transition: transform .2s; }
  :root[data-theme="light"] #theme-toggle { background: rgba(109,114,255,.12); border-color: rgba(87,92,240,.35); }
  :root[data-theme="light"] #theme-toggle::before { transform: translateX(18px); background: var(--docs-warning); }
  :root[data-theme="light"] #theme-toggle::after { content: '☀'; transform: translateX(18px); color: #fff; }
  :root[data-theme="light"] body { background: var(--docs-bg); color: var(--docs-text); }
  :root[data-theme="light"] #docs-toolbar { background: color-mix(in srgb, var(--docs-bg) 90%, transparent); }
    #token-gate { max-width: 620px; margin: 10vh auto; padding: 28px; background: var(--docs-panel); border: 1px solid var(--docs-border); border-radius: 16px; box-shadow: var(--docs-shadow); }
    #token-gate h1 { margin: 0 0 8px; color: var(--docs-text); font-size: 24px; letter-spacing: -.02em; }
    #token-gate p { color: var(--docs-muted); line-height: 1.55; }
    #token-gate input { width: 100%; box-sizing: border-box; padding: 12px; color: var(--docs-text); background: var(--docs-bg); border: 1px solid var(--docs-border); border-radius: var(--docs-radius-sm); outline: none; transition: border-color .18s, box-shadow .18s; }
    #token-gate input:focus { border-color: var(--docs-primary); box-shadow: 0 0 0 3px rgba(245,158,11,.18); }
    #token-gate input.invalid { border-color: #f87171; box-shadow: 0 0 0 3px rgba(248,113,113,.14); }
    #token-gate button { margin-top: 12px; padding: 11px 16px; color: #0a0a0a; font-weight: 700; background: linear-gradient(135deg,var(--docs-primary),var(--docs-primary-2)); border: 0; border-radius: var(--docs-radius-sm); box-shadow: 0 12px 28px rgba(245,158,11,.24); cursor: pointer; }
    #token-error { display: flex; align-items: center; gap: 7px; margin-top: 9px; color: #fca5a5; min-height: 20px; font-size: 14px; }
    #token-error:not(:empty)::before { content: '!'; display: inline-grid; place-items: center; width: 18px; height: 18px; color: #111827; background: #f87171; border-radius: 50%; font-size: 12px; font-weight: 800; }
    #swagger-ui { display: none; min-height: 100vh; }
    /* --- DDAG dark theme overrides for Swagger UI --- */
    #swagger-ui { background: var(--docs-bg); }
    #swagger-ui .swagger-ui { color: var(--docs-text); font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
    #swagger-ui .swagger-ui .wrapper { padding: 24px; max-width: 1440px; margin: 0 auto; }
    #swagger-ui .swagger-ui .info { margin: 30px 0 20px; }
    #swagger-ui .swagger-ui .info .title { color: #f8fafc; font-weight: 800; font-size: 32px; }
    #swagger-ui .swagger-ui .info .base-url { color: #9aa8bd; font-family: var(--docs-mono); font-size: 13px; }
    #swagger-ui .swagger-ui .info .description p { color: #b8c2d6; line-height: 1.6; }
    #swagger-ui .swagger-ui .scheme-container { background: transparent; box-shadow: none; padding: 12px 0; }
    #swagger-ui .swagger-ui .scheme-container .schemes { padding: 0; }
    #swagger-ui .swagger-ui .opblock.opblock-get .opblock-summary-method { background: #2563eb; }
    #swagger-ui .swagger-ui .opblock.opblock-post .opblock-summary-method { background: #16a34a; }
    #swagger-ui .swagger-ui .opblock.opblock-put .opblock-summary-method { background: #f59e0b; color: #0a0a0a; }
    #swagger-ui .swagger-ui .opblock.opblock-patch .opblock-summary-method { background: #a855f7; }
    #swagger-ui .swagger-ui .opblock.opblock-delete .opblock-summary-method { background: #ef4444; }
    #swagger-ui .swagger-ui .opblock.opblock-query .opblock-summary-method { background: #0ea5e9; }
    #swagger-ui .swagger-ui .opblock { background: rgba(22,28,43,0.75); border: 1px solid #263247; border-radius: 14px; margin-bottom: 20px; box-shadow: 0 10px 30px rgba(0,0,0,0.2); overflow: hidden; }
    #swagger-ui .swagger-ui .opblock .opblock-section-header { background: rgba(15,23,42,0.85); border-bottom: 1px solid #263247; padding: 12px 20px; }
    #swagger-ui .swagger-ui .opblock .opblock-section-header h4 { color: #f8fafc; font-size: 14px; font-weight: 700; }
    #swagger-ui .swagger-ui .opblock .opblock-summary { border-color: #263247; padding: 14px 20px; outline: none !important; border-top: none !important; border-left: none !important; border-right: none !important; }
    #swagger-ui .swagger-ui .opblock .opblock-summary:focus, #swagger-ui .swagger-ui .opblock .opblock-summary:active, #swagger-ui .swagger-ui .opblock .opblock-summary:focus-visible { outline: none !important; box-shadow: none !important; }
    #swagger-ui .swagger-ui *:focus, #swagger-ui .swagger-ui *:focus-visible { outline: none !important; }
    #swagger-ui .swagger-ui .opblock .opblock-summary-method { border: 0; border-radius: 8px; color: #fff; font-weight: 800; min-width: 80px; padding: 6px 12px; text-align: center; }
    #swagger-ui .swagger-ui .opblock .opblock-summary-description { color: #cbd5e1; font-size: 13px; }
    #swagger-ui .swagger-ui .opblock .opblock-body { color: #e2e8f0; padding: 20px; }
    #swagger-ui .swagger-ui .opblock .opblock-body .opblock-body-parameter__name { color: #f1f5f9; font-weight: 700; }
    #swagger-ui .swagger-ui .opblock .opblock-body .opblock-body-parameter__name-required::after { color: #f87171; }
    #swagger-ui .swagger-ui .opblock .opblock-body .opblock-body-parameter__in { color: #93c5fd; font-family: var(--docs-mono); font-size: 11px; }
    #swagger-ui .swagger-ui .opblock .opblock-body .tabitem { color: #cbd5e1; font-weight: 600; }
    #swagger-ui .swagger-ui .opblock .opblock-body h4 { color: #f8fafc; }
    #swagger-ui .swagger-ui .opblock .opblock-body .opblock-section-header h4 { color: #f8fafc; }
    #swagger-ui .swagger-ui .opblock .opblock-body pre.microlight { color: #d1d5db; font-family: var(--docs-mono); font-size: 13px; }
    #swagger-ui .swagger-ui .opblock .opblock-body pre.microlight .token.string { color: #a5f3a5; }
    #swagger-ui .swagger-ui .opblock .opblock-body .response-col_description .markdown p,
    #swagger-ui .swagger-ui .opblock .opblock-body .response-col_description .renderedMarkdown p { color: #cbd5e1; }
    #swagger-ui .swagger-ui .opblock .opblock-body pre { background: #0b1220; color: #c6d7e8; border: 1px solid #263247; border-radius: 12px; padding: 16px; font-family: var(--docs-mono); font-size: 13px; }
    #swagger-ui .swagger-ui .response-control-media-type__accept-message { color: var(--docs-muted); font-size: 12px; border: 0; padding: 0; margin: 4px 0 0; }
    #swagger-ui .swagger-ui .response-control-media-type__accept-message select { background: var(--docs-panel-2); color: var(--docs-text); border: 1px solid var(--docs-border); border-radius: var(--docs-radius-sm); padding: 4px 8px; font-size: 12px; box-shadow: none; }
    #swagger-ui .swagger-ui .responses-header h4, #swagger-ui .swagger-ui .responses h4 { color: #f8fafc; font-size: 14px; font-weight: 700; }
    #swagger-ui .swagger-ui .opblock-body .microname { color: #93a4c2; }
    #swagger-ui .swagger-ui table { margin-bottom: 20px; }
    #swagger-ui .swagger-ui table thead tr td, #swagger-ui .swagger-ui table thead tr th { color: #94a3b8; border-bottom: 1px solid #334155; font-size: 12px; font-weight: 700; text-transform: uppercase; padding: 12px 8px; }
    #swagger-ui .swagger-ui table tbody tr td { color: #dbeafe; border-bottom: 1px solid rgba(255,255,255,0.06); padding: 16px 8px; vertical-align: top; }
    #swagger-ui .swagger-ui .parameters-col_name { color: #f1f5f9; font-weight: 700; font-size: 13px; }
    #swagger-ui .swagger-ui .parameters-col_type { color: #93c5fd; font-family: var(--docs-mono); font-size: 12px; }
    #swagger-ui .swagger-ui .parameters-col_description input[type=text], #swagger-ui .swagger-ui .parameters-col_description select, #swagger-ui .swagger-ui .parameters-col_description textarea { background: #0b1220; color: #e6edf7; border: 1px solid #334155; border-radius: 8px; padding: 10px 14px; font-size: 13px; outline: none; transition: border-color .15s; }
    #swagger-ui .swagger-ui .parameters-col_description input[type=text]:focus, #swagger-ui .swagger-ui .parameters-col_description select:focus, #swagger-ui .swagger-ui .parameters-col_description textarea:focus { border-color: #60a5fa; box-shadow: 0 0 0 3px rgba(96,165,250,.15); }
    #swagger-ui .swagger-ui .parameters-col_description .markdown p, #swagger-ui .swagger-ui .parameters-col_description .renderedMarkdown p { color: #cbd5e1; }
    #swagger-ui .swagger-ui .response-col_status { color: #f59e0b; font-weight: 700; font-size: 14px; }
    #swagger-ui .swagger-ui .response-col_description .markdown p, #swagger-ui .swagger-ui .response-col_description .renderedMarkdown p { color: #cbd5e1; }
    #swagger-ui .swagger-ui .modelbox { background: #0b1220; border: 1px solid #263247; border-radius: 10px; padding: 12px; }
    #swagger-ui .swagger-ui .models-control { color: #cbd5e1; }
    #swagger-ui .swagger-ui .section.models { background: transparent; border: 1px solid #263247; border-radius: 14px; margin-top: 30px; }
    #swagger-ui .swagger-ui .section.models h4 { color: #f8fafc; font-size: 16px; font-weight: 700; padding: 16px 20px; }
    #swagger-ui .swagger-ui .model { color: #c6d7e8; }
    #swagger-ui .swagger-ui .model .property { color: #f59e0b; }
    #swagger-ui .swagger-ui .model .prop { color: #93a4c2; }
    #swagger-ui .swagger-ui .model-toggle-icon { fill: #9aa8bd; }
    #swagger-ui .swagger-ui .btn.authorize { color: var(--docs-primary); border-color: var(--docs-primary); background: transparent; border-radius: var(--docs-radius-sm); font-weight: 600; }
    #swagger-ui .swagger-ui .btn.authorize svg { fill: var(--docs-primary); }
    #swagger-ui .swagger-ui .btn.cancel { color: #f87171; border-color: #f87171; background: transparent; font-weight: 600; }
    #swagger-ui .swagger-ui .btn.execute { background: linear-gradient(135deg, var(--docs-primary), var(--docs-primary-2)); color: #0a0a0a; border: 0; border-radius: var(--docs-radius-sm); font-weight: 800; padding: 10px 24px; box-shadow: 0 4px 14px rgba(245,158,11,.25); transition: transform .15s, box-shadow .15s; }
    #swagger-ui .swagger-ui .btn.execute:hover { transform: translateY(-1px); box-shadow: 0 6px 18px rgba(245,158,11,.35); }
    #swagger-ui .swagger-ui .btn { border-radius: var(--docs-radius-sm); }
    #swagger-ui .swagger-ui .errors-wrapper { background: rgba(239,68,68,0.12); border: 1px solid rgba(239,68,68,0.3); border-radius: 10px; color: #fca5a5; padding: 14px 18px; margin: 16px 0; }
    #swagger-ui .swagger-ui .errors-wrapper h4 { color: #f87171; font-size: 14px; margin-bottom: 6px; }
    #swagger-ui .swagger-ui .download-url-wrapper { display: none; }
    #swagger-ui .swagger-ui .topbar { display: none; }
    #swagger-ui .swagger-ui .information-container { padding: 0; }
    #swagger-ui .swagger-ui .servers title { color: #9aa8bd; }
    #swagger-ui .swagger-ui label { color: #cbd5e1; font-weight: 600; }
    #swagger-ui .swagger-ui .response-col_links a { color: #60a5fa; }
    #swagger-ui ::-webkit-scrollbar { width: 10px; height: 10px; }
    #swagger-ui ::-webkit-scrollbar-track { background: #0b1220; }
    #swagger-ui ::-webkit-scrollbar-thumb { background: #334155; border-radius: 5px; }
    #swagger-ui ::-webkit-scrollbar-thumb:hover { background: #475569; }
    #swagger-ui .swagger-ui .opblock-tag { color: #f8fafc; border-bottom: 1px solid #263247; font-size: 20px; font-weight: 700; padding: 16px 0 10px; margin: 24px 0 16px; }
    #swagger-ui .swagger-ui .opblock-tag small { color: #94a3b8; font-size: 13px; font-weight: 400; }
    #swagger-ui .swagger-ui .opblock-summary-path, #swagger-ui .swagger-ui .opblock-summary-path__deprecated { color: #f1f5f9; font-family: var(--docs-mono); font-size: 14px; font-weight: 600; }
    #swagger-ui .swagger-ui .opblock-summary-path a { color: inherit; }
    #swagger-ui .swagger-ui svg { fill: #cbd5e1; }
    :root[data-theme="light"] #token-gate { background: #fff; border-color: #cbd5e1; color: #0f172a; }
    :root[data-theme="light"] #token-gate p { color: #475569; }
    :root[data-theme="light"] #token-gate input { background: #fff; color: #0f172a; border-color: #94a3b8; }
    :root[data-theme="light"] #swagger-ui { background: #f8fafc; }
    :root[data-theme="light"] #swagger-ui .swagger-ui { color: #0f172a; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .info .title, :root[data-theme="light"] #swagger-ui .swagger-ui .opblock-tag, :root[data-theme="light"] #swagger-ui .swagger-ui .opblock .opblock-section-header h4, :root[data-theme="light"] #swagger-ui .swagger-ui .section.models h4 { color: #0f172a; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .info .base-url, :root[data-theme="light"] #swagger-ui .swagger-ui .info .description p, :root[data-theme="light"] #swagger-ui .swagger-ui .opblock .opblock-summary-description, :root[data-theme="light"] #swagger-ui .swagger-ui .opblock-tag small, :root[data-theme="light"] #swagger-ui .swagger-ui label { color: #475569; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .opblock { background: #fff; border-color: #cbd5e1; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .opblock .opblock-summary { border-bottom-color: #e2e8f0 !important; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .opblock .opblock-section-header { background: #f1f5f9; border-color: #cbd5e1; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .opblock-tag { border-bottom-color: #cbd5e1; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .opblock-summary-path, :root[data-theme="light"] #swagger-ui .swagger-ui .opblock-summary-path__deprecated { color: #1e293b; }
    :root[data-theme="light"] #swagger-ui .swagger-ui table tbody tr td, :root[data-theme="light"] #swagger-ui .swagger-ui .model { color: #334155; }
    :root[data-theme="light"] #swagger-ui .swagger-ui svg { fill: #334155; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .modelbox, :root[data-theme="light"] #swagger-ui .swagger-ui .opblock .opblock-body pre { background: #f1f5f9; color: #1e293b; border-color: #cbd5e1; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .parameters-col_description input[type=text], :root[data-theme="light"] #swagger-ui .swagger-ui .parameters-col_description select { background: #fff; color: #0f172a; border-color: #cbd5e1; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .parameters-col_name { color: #0f172a; font-weight: 700; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .parameters-col_type { color: var(--docs-primary); }
    :root[data-theme="light"] #swagger-ui .swagger-ui .opblock .opblock-body .opblock-body-parameter__name { color: var(--docs-text); font-weight: 700; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .opblock .opblock-body .opblock-body-parameter__in { color: var(--docs-info); }
    :root[data-theme="light"] #swagger-ui .swagger-ui .btn.authorize { color: var(--docs-primary); border-color: var(--docs-primary); }
    :root[data-theme="light"] #swagger-ui .swagger-ui .btn.authorize svg { fill: var(--docs-primary); }
    :root[data-theme="light"] #swagger-ui .swagger-ui .response-col_status { color: var(--docs-primary); }
    :root[data-theme="light"] #swagger-ui .swagger-ui table thead tr td, :root[data-theme="light"] #swagger-ui .swagger-ui table thead tr th { color: #475569; border-bottom-color: #cbd5e1; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .parameters-col_description .markdown p, :root[data-theme="light"] #swagger-ui .swagger-ui .parameters-col_description .renderedMarkdown p, :root[data-theme="light"] #swagger-ui .swagger-ui .response-col_description .markdown p, :root[data-theme="light"] #swagger-ui .swagger-ui .response-col_description .renderedMarkdown p { color: #475569; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .model .property { color: #d97706; }
    :root[data-theme="light"] #swagger-ui .swagger-ui .model .prop { color: #64748b; }
    @media (max-width: 640px) { #docs-toolbar { position: static; margin: 12px; justify-content: flex-end; } #swagger-ui .swagger-ui .wrapper { padding: 8px; } }
  </style>
</head>
<body>
  <header id="docs-toolbar">
    <div class="brand"><div class="brand-mark"><img src="/favicon.svg" alt="DDAG"></div><span>DDAG API Docs</span></div>
    <button id="theme-toggle" role="switch" aria-checked="false" aria-label="Switch to light theme" title="Switch theme"></button>
  </header>
  <main id="token-gate">
    <h1>DDAG API Documentation</h1>
    <p>Enter an OAuth2 access token. The token stays only in this browser tab and is sent to DDAG to load the API contract permitted for your client.</p>
    <form id="token-form" novalidate>
      <input id="token" type="password" autocomplete="off" placeholder="OAuth2 access token" aria-describedby="token-error" aria-invalid="false">
      <button type="submit">Open API Documentation</button>
    </form>
    <div id="token-error" role="alert"></div>
  </main>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    const tokenKey = 'ddag.swagger.token';
    const form = document.getElementById('token-form');
    const input = document.getElementById('token');
    const gate = document.getElementById('token-gate');
    const docs = document.getElementById('swagger-ui');
    const error = document.getElementById('token-error');
    const themeToggle = document.getElementById('theme-toggle');
    const themeKey = 'ddag.docs.theme';

    function applyTheme(preference) {
      const resolved = preference === 'system'
        ? (window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark')
        : preference;
      document.documentElement.dataset.theme = resolved;
      const isDark = resolved === 'dark';
      themeToggle.setAttribute('aria-checked', !isDark);
      themeToggle.setAttribute('aria-label', isDark ? 'Switch to light theme' : 'Switch to dark theme');
    }

    let currentTheme = localStorage.getItem('ddag.docs.theme') || 'system';
    applyTheme(currentTheme);
    themeToggle.addEventListener('click', function() {
      const isDark = document.documentElement.dataset.theme === 'dark';
      currentTheme = isDark ? 'light' : 'dark';
      localStorage.setItem(themeKey, currentTheme);
      applyTheme(currentTheme);
    });
    window.matchMedia('(prefers-color-scheme: light)').addEventListener('change', function() {
      if (currentTheme === 'system') applyTheme('system');
    });

    const metaTheme = document.querySelector('meta[name="theme-color"]');
    const observer = new MutationObserver(function() {
      if (metaTheme) metaTheme.setAttribute('content', document.documentElement.dataset.theme === 'light' ? '#f8fafc' : '#070b14');
    });
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    if (metaTheme) metaTheme.setAttribute('content', document.documentElement.dataset.theme === 'light' ? '#f8fafc' : '#070b14');

    const cssQUERY = "opblock-query";

    function queryPaths(contract) {
      const result = new Set();
      if (!contract || !contract.paths) return result;
      Object.keys(contract.paths).forEach(function(pathKey) {
        const postOp = contract.paths[pathKey] && contract.paths[pathKey].post;
        if (postOp && String(postOp["x-ddag-http-method"]).toUpperCase() === "QUERY") result.add(pathKey);
      });
      return result;
    }

    function pathMatches(pattern, pathname) {
      const expected = String(pattern).split('/').filter(Boolean);
      const actual = String(pathname).split('/').filter(Boolean);
      if (expected.length !== actual.length) return false;
      return expected.every(function(segment, index) {
        return (segment.startsWith('{') && segment.endsWith('}')) || decodeURIComponent(actual[index]) === segment;
      });
    }

    function isQueryRequest(paths, requestURL) {
      const pathname = new URL(requestURL, window.location.origin).pathname;
      return Array.from(paths).some(function(pattern) { return pathMatches(pattern, pathname); });
    }

    function applyQueryBadges(paths) {
      document.querySelectorAll('.opblock.opblock-post').forEach(function(el) {
        const pathLabel = el.querySelector('.opblock-summary-path');
        if (!pathLabel || !paths.has(pathLabel.textContent.trim())) return;
        el.classList.remove('opblock-post');
        el.classList.add(cssQUERY);
        el.setAttribute('data-ddag-method', 'QUERY');
        const badge = el.querySelector('.opblock-summary-method');
        if (badge && badge.textContent.trim() !== 'QUERY') badge.textContent = 'QUERY';
      });
    }

    function observeQueryBadges(paths) {
      let queued = false;
      const observer = new MutationObserver(function() {
        if (queued) return;
        queued = true;
        requestAnimationFrame(function() {
          queued = false;
          applyQueryBadges(paths);
        });
      });
      observer.observe(docs, { childList: true, subtree: true });
      applyQueryBadges(paths);
      return observer;
    }

    function openDocs(token, contract) {
      const paths = queryPaths(contract);
      observeQueryBadges(paths);
      window.ui = SwaggerUIBundle({
        spec: contract,
        dom_id: '#swagger-ui',
        persistAuthorization: false,
        requestInterceptor: function(req) {
          req.headers = req.headers || {};
          req.headers.Authorization = 'Bearer ' + token;
          if (isQueryRequest(paths, req.url) && String(req.method).toUpperCase() === "POST") req.method = "QUERY";
          return req;
        },
        onComplete: function() {
          gate.style.display = 'none';
          docs.style.display = 'block';
          applyQueryBadges(paths);
          setTimeout(function() { applyQueryBadges(paths); }, 500);
        }
      });
    }

    form.addEventListener('submit', async function(event) {
      event.preventDefault();
      const token = input.value.trim().replace(/^Bearer\s+/i, '');
      error.textContent = '';
      input.classList.remove('invalid');
      input.setAttribute('aria-invalid', 'false');
      if (!token) {
        error.textContent = 'Enter an OAuth2 access token.';
        input.classList.add('invalid');
        input.setAttribute('aria-invalid', 'true');
        input.focus();
        return;
      }
      try {
        const response = await fetch('/openapi.json', { headers: { Authorization: 'Bearer ' + token } });
        if (!response.ok) throw new Error(response.status === 401 ? 'Token is invalid or expired.' : 'Unable to load the API contract.');
        const contract = await response.json();
        sessionStorage.setItem(tokenKey, token);
        openDocs(token, contract);
      } catch (err) {
        error.textContent = err.message || 'Authorization failed.';
        input.classList.add('invalid');
        input.setAttribute('aria-invalid', 'true');
      }
    });

    input.addEventListener('input', function() {
      if (!input.value.trim()) return;
      error.textContent = '';
      input.classList.remove('invalid');
      input.setAttribute('aria-invalid', 'false');
    });

    const savedToken = sessionStorage.getItem('ddag.swagger.token');
    if (savedToken) {
      input.value = savedToken;
      form.requestSubmit();
    }
  </script>
</body>
</html>`
