// DDAG admin dashboard (Nuxt 3). Single-page-style admin app that talks to the
// admin-backend control-plane API. No mock data — every page is wired to real
// endpoints.
export default defineNuxtConfig({
  ssr: false, // SPA: the dashboard is a pure client of the admin-backend API
  devtools: { enabled: false },
  css: ['~/assets/css/main.css'],
  app: {
    head: {
      title: 'DDAG — Dynamic Database API Gateway',
      meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }],
    },
  },
  runtimeConfig: {
    public: {
      // admin-backend (control plane) and api-gateway (data plane) base URLs.
      apiBase: 'http://localhost:8080',
      gatewayBase: 'http://localhost:8082',
      authBase: 'http://localhost:8081',
    },
  },
  devServer: { port: 3000 },
  compatibilityDate: '2025-01-01',
})
