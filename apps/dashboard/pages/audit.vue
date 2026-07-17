<script setup lang="ts">
const api = useApi()
const rows = ref<any[]>([])
const loading = ref(true)
const error = ref('')
const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'created_at', sortDir: 'desc' })
const filters = reactive({ action: '', actor_type: '', actor_id: '', resource_type: '', resource_id: '', request_id: '', status: '', from: '', to: '' })
const detail = ref<any>(null)
const detailOpen = ref(false)
const columns = [
  { key: 'created_at', label: 'Time', sortable: true }, { key: 'actor', label: 'Actor', sortable: true },
  { key: 'action', label: 'Action', sortable: true }, { key: 'resource', label: 'Resource' },
  { key: 'request_id', label: 'Request ID', mono: true }, { key: 'ip_address', label: 'IP' }, { key: 'status', label: 'Status', sortable: true },
]
const fmt = (v: string) => (v ? new Date(v).toLocaleString() : '—')
async function load(q = table) {
  Object.assign(table, q); loading.value = true; error.value = ''
  const p = new URLSearchParams({ page: String(table.page), limit: String(table.limit), sort_by: table.sortBy, sort_dir: table.sortDir, search: table.search })
  Object.entries(filters).forEach(([k, v]) => {
    if (!v) return
    if ((k === 'from' || k === 'to') && typeof v === 'string') p.set(k, new Date(v).toISOString())
    else p.set(k, String(v))
  })
  try {
    const res = await api.list(`/api/audit-logs?${p}`)
    rows.value = res.items
    table.total = res.pagination?.total ?? rows.value.length
  } catch (e: any) {
    rows.value = []
    table.total = 0
    error.value = e?.message || 'Unable to load audit logs.'
  } finally { loading.value = false }
}
onMounted(() => load())
let filterTimer: ReturnType<typeof setTimeout> | undefined
watch(filters, () => {
  clearTimeout(filterTimer)
  filterTimer = setTimeout(() => load({ ...table, page: 1 }), 350)
}, { deep: true })
onUnmounted(() => clearTimeout(filterTimer))
function open(row: any) { detail.value = row; detailOpen.value = true }
function exportLogs(format: 'csv' | 'json') {
  const p = new URLSearchParams({ format })
  Object.entries(filters).forEach(([k, v]) => {
    if (!v) return
    if ((k === 'from' || k === 'to') && typeof v === 'string') p.set(k, new Date(v).toISOString())
    else p.set(k, String(v))
  })
  window.open(`/api/audit-logs/export?${p.toString()}`, '_blank', 'noopener')
}
</script>
<template>
  <PageHeader
    title="Audit Logs"
    eyebrow="SECURITY TRAIL"
    icon="audit"
    description="Immutable, append-only record of every important action. Filter by actor, resource, request, status, and time window."
  >
    <template #actions>
      <button class="btn ghost" @click="exportLogs('json')"><Icon name="download" /> JSON</button>
      <button class="btn ghost" @click="exportLogs('csv')"><Icon name="download" /> CSV</button>
      <button class="btn primary" @click="load"><Icon name="rotate" /> Refresh</button>
    </template>
  </PageHeader>

  <section class="audit-filter-card">
    <div class="audit-filter-head">
      <div>
        <div class="eyebrow">FILTER CONTROLS</div>
        <h3>Find audit events</h3>
      </div>
      <button class="btn ghost sm" type="button" @click="Object.keys(filters).forEach((k) => (filters[k as keyof typeof filters] = ''))">Clear filters</button>
    </div>
    <div class="audit-filter-grid">
      <label class="filter-field wide">
        <span>Action</span>
        <input v-model="filters.action" placeholder="login, create, update…" />
      </label>
      <label class="filter-field">
        <span>Actor type</span>
        <select v-model="filters.actor_type"><option value="">All actors</option><option value="user">user</option><option value="client">client</option><option value="system">system</option></select>
      </label>
      <label class="filter-field">
        <span>Status</span>
        <select v-model="filters.status"><option value="">All status</option><option value="success">success</option><option value="failure">failure</option></select>
      </label>
      <label class="filter-field wide">
        <span>Request ID</span>
        <input v-model="filters.request_id" placeholder="req_…" />
      </label>
      <label class="filter-field">
        <span>Actor ID</span>
        <input v-model="filters.actor_id" placeholder="user/client id" />
      </label>
      <label class="filter-field">
        <span>Resource type</span>
        <input v-model="filters.resource_type" placeholder="api, user, role…" />
      </label>
      <label class="filter-field">
        <span>Resource ID</span>
        <input v-model="filters.resource_id" placeholder="resource id" />
      </label>
      <label class="filter-field">
        <span>From</span>
        <input v-model="filters.from" type="datetime-local" />
      </label>
      <label class="filter-field">
        <span>To</span>
        <input v-model="filters.to" type="datetime-local" />
      </label>
    </div>
  </section>

  <UiTable remote has-actions :columns="columns" :rows="rows" :loading="loading" :error="error" :retry="load" :page="table.page" :limit="table.limit" :total="table.total" :search="table.search" :sort-by="table.sortBy" :sort-dir="table.sortDir" empty="No audit events." @query="load">
    <template #col-created_at="{ value }"><span class="faint mono">{{ fmt(value) }}</span></template>
    <template #col-actor="{ row }"><div>{{ row.actor_label || row.actor_id || '—' }}<span class="faint" style="display:block;font-size:11px">{{ row.actor_type }}</span></div></template>
    <template #col-action="{ value }"><span class="mono">{{ value }}</span></template>
    <template #col-resource="{ row }"><span class="faint mono">{{ row.resource_type }} {{ row.resource_id }}</span></template>
    <template #col-request_id="{ value }"><span class="faint mono">{{ value || '—' }}</span></template>
    <template #col-status="{ value }"><StatusBadge :status="value" /></template>
    <template #actions="{ row }"><span class="actions-cell"><button class="btn icon" title="Details" @click="open(row)"><Icon name="eye" /></button></span></template>
  </UiTable>
  <UiModal :open="detailOpen" title="Audit Event" wide @close="detailOpen = false"><pre v-if="detail" class="code-block">{{ JSON.stringify(detail, null, 2) }}</pre></UiModal>
</template>
