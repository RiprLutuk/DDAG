<script setup lang="ts">
const api = useApi()
const toast = useToast()
const rows = ref<any[]>([])
const loading = ref(true)
const refreshing = ref<string | null>(null)
let serviceTimer: any = null
const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'updated_at', sortDir: 'desc' })
const columns = [
  { key: 'name', label: 'Service' }, { key: 'kind', label: 'Role' }, { key: 'version', label: 'Version' },
  { key: 'last_health_status', label: 'Health' }, { key: 'last_seen_at', label: 'Last check' }, { key: 'capabilities', label: 'Capabilities' },
]
async function load() {
  loading.value = true
  try {
    const result = await api.list(`/api/services?${new URLSearchParams({ page: String(table.page), limit: String(table.limit), search: table.search, sort_by: table.sortBy, sort_dir: table.sortDir })}`)
    rows.value = result.items; table.total = result.pagination.total
  } catch (e: any) { toast.error('Cannot load services', e.message) } finally { loading.value = false }
}
onMounted(() => { load(); serviceTimer = setInterval(load, 60000) })
onUnmounted(() => { if (serviceTimer) clearInterval(serviceTimer) })
function queryTable(q: any) { Object.assign(table, q); load() }
async function refresh(row: any) {
  refreshing.value = row.id
  try { await api.post(`/api/services/${row.id}/refresh`); toast.success('Health check refreshed', row.name); await load() }
  catch (e: any) { toast.error('Refresh failed', e.message) } finally { refreshing.value = null }
}
function capabilityText(value: any) { return Object.entries(value || {}).map(([key, val]) => `${key}: ${val}`).join(', ') || '—' }
function checked(value: string | null) { return value ? new Date(value).toLocaleString() : 'Never' }
function isInternalEndpoint(value: string | null | undefined) {
  if (!value) return false
  try {
    const host = new URL(value).hostname.toLowerCase()
    return host === 'localhost' || host === '127.0.0.1' || host === '::1' || host.endsWith('.internal') || host.endsWith('.local') || host.startsWith('10.') || host.startsWith('192.168.') || /^172\.(1[6-9]|2\d|3[0-1])\./.test(host)
  } catch { return false }
}
function canOpenEndpoint(value: string | null | undefined) { return Boolean(value) && !isInternalEndpoint(value) }
</script>

<template>
  <PageHeader eyebrow="CONTROL PLANE" title="Services" icon="cpu"
    description="Internal service registry for DDAG. Browser users do not open localhost/intranet endpoints; backend service-checker probes them safely in the background." />
  <div class="service-warning-banner">
    <b>Intranet mode:</b> localhost/private connector URLs are checked by the DDAG backend scheduler/manual checker. Direct browser open is disabled for internal endpoints.
  </div>
  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions remote v-bind="table" empty="No services registered yet." @query="queryTable">
    <template #col-name="{ row }"><div><b>{{ row.name }}</b><div class="faint mono">{{ row.base_url || 'No endpoint configured' }}</div><span v-if="isInternalEndpoint(row.base_url)" class="tag internal">Internal only</span></div></template>
    <template #col-kind="{ value }"><span class="badge blue">{{ value }}</span></template>
    <template #col-version="{ row }"><span>{{ row.version || '—' }}</span><div v-if="row.commit_sha" class="faint mono">{{ row.commit_sha.slice(0, 12) }}</div></template>
    <template #col-last_health_status="{ value }"><StatusBadge :status="value" /></template>
    <template #col-last_seen_at="{ value }">{{ checked(value) }}</template>
    <template #col-capabilities="{ value }"><span class="faint">{{ capabilityText(value) }}</span></template>
    <template #actions="{ row }"><span class="actions-cell">
      <a v-if="canOpenEndpoint(row.health_url)" class="btn icon" :href="row.health_url" target="_blank" title="Open public health endpoint">↗</a>
      <button v-else-if="row.health_url" class="btn icon muted-action" disabled title="Internal endpoint: checked by backend service checker">↗</button>
      <a v-if="canOpenEndpoint(row.ready_url)" class="btn icon" :href="row.ready_url" target="_blank" title="Open public ready endpoint">✓</a>
      <button v-else-if="row.ready_url" class="btn icon muted-action" disabled title="Internal endpoint: checked by backend service checker">✓</button>
      <button class="btn icon go" title="Run backend service check now" :disabled="refreshing === row.id" @click="refresh(row)"><span :class="{ spin: refreshing === row.id }">↻</span></button>
    </span></template>
  </UiTable>
</template>
