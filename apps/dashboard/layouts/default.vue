<script setup lang="ts">
const { state, logout, hasAny } = useAuth()
const route = useRoute()

interface NavItem { to: string; label: string; ico: string; perms?: string[] }
interface NavGroup { title: string; items: NavItem[] }

const groups: NavGroup[] = [
  { title: 'Main', items: [{ to: '/', label: 'Overview', ico: '◧' }] },
  {
    title: 'Gateway', items: [
      { to: '/apis', label: 'API Management', ico: '◈' },
      { to: '/connections', label: 'Database Connections', ico: '⛁' },
      { to: '/clients', label: 'Clients', ico: '◆' },
    ],
  },
  {
    title: 'Security', items: [
      { to: '/users', label: 'Users', ico: '☷', perms: ['manage_user'] },
      { to: '/roles', label: 'Roles & Permissions', ico: '⚿' },
      { to: '/scopes', label: 'Token & Scopes', ico: '⚷' },
      { to: '/ip-whitelists', label: 'IP Whitelist', ico: '⛨', perms: ['manage_ip_whitelist'] },
      { to: '/rate-limits', label: 'Rate Limits', ico: '◔', perms: ['manage_rate_limit'] },
    ],
  },
  {
    title: 'Operations', items: [
      { to: '/cache', label: 'Cache', ico: '⚡', perms: ['view_monitoring', 'purge_cache'] },
      { to: '/monitoring', label: 'Monitoring', ico: '◴', perms: ['view_monitoring'] },
      { to: '/logs', label: 'Request Logs', ico: '☰', perms: ['view_monitoring'] },
      { to: '/audit', label: 'Audit Logs', ico: '✓', perms: ['view_audit'] },
    ],
  },
  { title: 'System', items: [{ to: '/settings', label: 'Settings', ico: '⚙' }] },
]

const visibleGroups = computed(() =>
  groups
    .map((g) => ({ ...g, items: g.items.filter((i) => !i.perms || hasAny(...i.perms)) }))
    .filter((g) => g.items.length > 0),
)

const isActive = (to: string) => (to === '/' ? route.path === '/' : route.path.startsWith(to))
const title = computed(() => (route.meta.title as string) || 'DDAG')
</script>

<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="brand">
        <b>DDAG</b>
        <span>Dynamic Database API Gateway</span>
      </div>
      <nav class="nav">
        <template v-for="g in visibleGroups" :key="g.title">
          <div class="nav-section">{{ g.title }}</div>
          <NuxtLink v-for="i in g.items" :key="i.to" :to="i.to" :class="{ active: isActive(i.to) }">
            <span class="ico">{{ i.ico }}</span>{{ i.label }}
          </NuxtLink>
        </template>
      </nav>
      <div class="userbox">
        <div class="u">{{ state.user?.name || state.user?.username }}</div>
        <div class="r">{{ (state.roles || []).join(', ') }}</div>
        <button class="btn ghost sm" style="margin-top:10px;width:100%" @click="logout">Sign out</button>
      </div>
    </aside>

    <div class="main">
      <header class="topbar">
        <h1>{{ title }}</h1>
        <div class="faint">{{ state.user?.email }}</div>
      </header>
      <main class="content">
        <slot />
      </main>
    </div>
  </div>
</template>
