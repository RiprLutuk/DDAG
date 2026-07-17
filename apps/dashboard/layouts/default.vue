<script setup lang="ts">
const { state, logout, hasAny } = useAuth()
const route = useRoute()

const mobileNavOpen = ref(false)
const sidebarCollapsed = ref(false)
const { theme, initTheme, toggleTheme } = useTheme()

onMounted(() => {
  initTheme()
  sidebarCollapsed.value = localStorage.getItem('ddag.sidebar.collapsed') === '1'
})

watch(sidebarCollapsed, (collapsed) => {
  localStorage.setItem('ddag.sidebar.collapsed', collapsed ? '1' : '0')
})

watch(() => route.path, () => { mobileNavOpen.value = false })

interface NavItem { to: string; label: string; icon: string; perms?: string[] }
interface NavGroup { title: string; items: NavItem[] }

const groups: NavGroup[] = [
  { title: 'Main', items: [{ to: '/', label: 'Overview', icon: 'overview' }] },
  {
    title: 'Gateway', items: [
      { to: '/apis', label: 'API Management', icon: 'apis' },
      { to: '/connections', label: 'Database Connections', icon: 'database' },
      { to: '/clients', label: 'Clients', icon: 'clients' },
    ],
  },
  {
    title: 'Security', items: [
      { to: '/users', label: 'Users', icon: 'users', perms: ['manage_user'] },
      { to: '/roles', label: 'Roles & Permissions', icon: 'roles' },
      { to: '/scopes', label: 'Token & Scopes', icon: 'scopes' },
      { to: '/ip-whitelists', label: 'IP Whitelist', icon: 'shield', perms: ['manage_ip_whitelist'] },
      { to: '/rate-limits', label: 'Rate Limits', icon: 'speed', perms: ['manage_rate_limit'] },
    ],
  },
  {
    title: 'Operations', items: [
      { to: '/cache', label: 'Cache', icon: 'cache', perms: ['view_monitoring', 'purge_cache'] },
      { to: '/monitoring', label: 'Monitoring', icon: 'monitoring', perms: ['view_monitoring', 'view_circuit_state'] },
      { to: '/services', label: 'Services', icon: 'services', perms: ['view_monitoring'] },
      { to: '/backups', label: 'Backup & Recovery', icon: 'database', perms: ['view_monitoring'] },
      { to: '/logs', label: 'Request Logs', icon: 'logs', perms: ['view_monitoring'] },
      { to: '/audit', label: 'Audit Logs', icon: 'audit', perms: ['view_audit'] },
    ],
  },
  { title: 'System', items: [{ to: '/settings', label: 'Settings', icon: 'settings' }] },
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
  <div class="layout" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <div v-if="mobileNavOpen" class="mobile-nav-backdrop" @click="mobileNavOpen = false" />

    <aside class="sidebar" :class="{ open: mobileNavOpen }">
      <div class="brand">
        <div class="brand-mark">D</div>
        <div class="brand-copy">
          <b>DDAG</b>
          <span>Dynamic Database API Gateway</span>
        </div>
        <button class="btn icon nav-close mobile-only" title="Close navigation" @click="mobileNavOpen = false">
          <span style="font-size:18px;line-height:1">×</span>
        </button>
      </div>

      <nav class="nav" aria-label="Main navigation">
        <template v-for="g in visibleGroups" :key="g.title">
          <div class="nav-section">{{ g.title }}</div>
          <RouterLink v-for="i in g.items" :key="i.to" :to="i.to" :class="{ active: isActive(i.to) }" :title="i.label">
            <span class="ico"><Icon :name="i.icon" /></span><span class="nav-label">{{ i.label }}</span>
          </RouterLink>
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
        <div class="topbar-start">
          <button class="btn icon nav-toggle mobile-only" title="Open navigation" @click="mobileNavOpen = true">
            <span style="font-size:16px;line-height:1">☰</span>
          </button>
          <button class="btn icon nav-toggle desktop-only" :title="sidebarCollapsed ? 'Show menu' : 'Hide menu'" @click="sidebarCollapsed = !sidebarCollapsed">
            <span style="font-size:16px;line-height:1">{{ sidebarCollapsed ? '☰' : '‹' }}</span>
          </button>
          <div>
            <h1>{{ title }}</h1>
            <div class="faint topbar-sub mobile-only">{{ state.user?.email }}</div>
          </div>
        </div>
        <div class="topbar-actions">
          <button class="theme-toggle" type="button" :aria-label="theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'" @click="toggleTheme">
            <span class="theme-toggle-track">
              <span class="theme-toggle-thumb">
                <Icon :name="theme === 'dark' ? 'moon' : 'sun'" :size="14" />
              </span>
            </span>
            <span class="theme-toggle-label desktop-only">{{ theme === 'dark' ? 'Dark' : 'Light' }}</span>
          </button>
          <span class="status-dot" />
          <div class="faint desktop-only">{{ state.user?.email }}</div>
        </div>
      </header>
      <main class="content">
        <slot />
      </main>
    </div>
  </div>
</template>
