import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { authGuard } from './auth-guard'

const routes: RouteRecordRaw[] = [
  { path: '/', component: () => import('../pages/index.vue'), meta: { title: 'Overview' } },
  { path: '/apis', component: () => import('../pages/apis.vue'), meta: { title: 'API Management' } },
  { path: '/connections', component: () => import('../pages/connections.vue'), meta: { title: 'Database Connections' } },
  { path: '/clients', component: () => import('../pages/clients.vue'), meta: { title: 'Clients' } },
  { path: '/users', component: () => import('../pages/users.vue'), meta: { title: 'Users' } },
  { path: '/roles', component: () => import('../pages/roles.vue'), meta: { title: 'Roles & Permissions' } },
  { path: '/scopes', component: () => import('../pages/scopes.vue'), meta: { title: 'Token & Scopes' } },
  { path: '/ip-whitelists', component: () => import('../pages/ip-whitelists.vue'), meta: { title: 'IP Whitelist' } },
  { path: '/rate-limits', component: () => import('../pages/rate-limits.vue'), meta: { title: 'Rate Limits' } },
  { path: '/cache', component: () => import('../pages/cache.vue'), meta: { title: 'Cache' } },
  { path: '/monitoring', component: () => import('../pages/monitoring.vue'), meta: { title: 'Monitoring' } },
  { path: '/services', component: () => import('../pages/services.vue'), meta: { title: 'Services' } },
  { path: '/logs', component: () => import('../pages/logs.vue'), meta: { title: 'Request Logs' } },
  { path: '/audit', component: () => import('../pages/audit.vue'), meta: { title: 'Audit Logs' } },
  { path: '/jobs', component: () => import('../pages/jobs.vue'), meta: { title: 'Jobs' } },
  { path: '/backups', component: () => import('../pages/backups.vue'), meta: { title: 'Backups' } },
  { path: '/settings', component: () => import('../pages/settings.vue'), meta: { title: 'Settings' } },
  { path: '/login', component: () => import('../pages/login.vue'), meta: { layout: 'blank', title: 'Sign in' } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({ history: createWebHistory(), routes })
router.beforeEach(authGuard)
router.afterEach((to) => { document.title = `${to.meta.title || 'DDAG'} — DDAG` })

export default router
