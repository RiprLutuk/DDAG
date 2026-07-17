<script setup lang="ts">
const api = useApi()
const rows = ref<any[]>([])
const loading = ref(true)
const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'created_at', sortDir: 'desc' })
const filters = reactive({ client_id: '', api_id: '', status_code: '', min_latency_ms: '', max_latency_ms: '', cached: '', request_id: '' })
const columns = [
  { key: 'created_at', label: 'Time', sortable: true }, { key: 'method', label: 'Method', sortable: true },
  { key: 'ip_address', label: 'IP', mono: true, sortable: true }, { key: 'api_label', label: 'API', sortable: true },
  { key: 'status_code', label: 'Status', sortable: true }, { key: 'latency_ms', label: 'Latency', sortable: true },
  { key: 'source_db_duration_ms', label: 'Source DB', sortable: true }, { key: 'cached', label: 'Cached', sortable: true },
  { key: 'client_label', label: 'Client', sortable: true }, { key: 'request_id', label: 'Request ID', mono: true },
]
const fmt = (v: string) => (v ? new Date(v).toLocaleString() : '—')
const activeFilterCount = computed(() => Object.values(filters).filter(Boolean).length + (table.search ? 1 : 0))
async function load(q = table) {
  Object.assign(table, q); loading.value = true
  try {
    const params = new URLSearchParams({ page: String(table.page), limit: String(table.limit), search: table.search, sort_by: table.sortBy, sort_dir: table.sortDir })
    Object.entries(filters).forEach(([k, v]) => { if (v !== '') params.set(k, String(v)) })
    const res = await api.list(`/api/request-logs?${params.toString()}`)
    rows.value = res.items
    table.total = res.pagination?.total ?? rows.value.length
  } finally { loading.value = false }
}
function resetFilters() {
  Object.assign(filters, { client_id: '', api_id: '', status_code: '', min_latency_ms: '', max_latency_ms: '', cached: '', request_id: '' })
  table.search = ''
}
onMounted(() => load())
watch(filters, () => load({ ...table, page: 1 }), { deep: true })
</script>

<template>
  <PageHeader eyebrow="GATEWAY ACTIVITY" title="Request logs" icon="logs"
    description="Trace every data-plane request with response status, cache usage, and source database timing.">
    <template #actions>
      <button class="btn ghost refresh-btn" @click="load" :disabled="loading"><span>↻</span> Refresh</button>
    </template>
  </PageHeader>

  <section class="log-filter card">
    <div class="card-head">
      <div>
        <h3>Filter requests</h3>
        <p>Find a request by endpoint, client, status, or performance window.</p>
      </div>
      <div class="log-filter-actions">
        <span v-if="activeFilterCount" class="filter-count">{{ activeFilterCount }} active</span>
        <button class="btn sm ghost" :disabled="!activeFilterCount" @click="resetFilters">Clear filters</button>
      </div>
    </div>
    <div class="log-filter-body">
      <div class="log-filter-grid log-filter-primary">
        <label class="filter-field filter-wide"><span>Search</span><input v-model="table.search" placeholder="Path, API name, or client…" /></label>
        <label class="filter-field"><span>Request ID</span><input v-model="filters.request_id" placeholder="req-…" /></label>
        <label class="filter-field"><span>Status code</span><input v-model="filters.status_code" type="number" placeholder="e.g. 200" /></label>
        <label class="filter-field"><span>Cache</span><select v-model="filters.cached"><option value="">Any response</option><option value="true">Cached only</option><option value="false">Not cached</option></select></label>
      </div>
      <details class="advanced-filters">
        <summary>Advanced filters <span>Client, API and latency thresholds</span></summary>
        <div class="log-filter-grid">
          <label class="filter-field"><span>Client ID</span><input v-model="filters.client_id" placeholder="Client UUID…" /></label>
          <label class="filter-field"><span>API ID</span><input v-model="filters.api_id" placeholder="API UUID…" /></label>
          <label class="filter-field"><span>Minimum latency</span><input v-model="filters.min_latency_ms" type="number" placeholder="0 ms" /></label>
          <label class="filter-field"><span>Maximum latency</span><input v-model="filters.max_latency_ms" type="number" placeholder="1000 ms" /></label>
        </div>
      </details>
    </div>
  </section>

  <UiTable remote :columns="columns" :rows="rows" :loading="loading" :page="table.page" :limit="table.limit" :total="table.total" :search="table.search" :sort-by="table.sortBy" :sort-dir="table.sortDir" empty="No requests logged yet." @query="load">
    <template #col-created_at="{ value }"><span class="faint mono">{{ fmt(value) }}</span></template>
    <template #col-method="{ value }"><span class="badge gray">{{ value }}</span></template>
    <template #col-ip_address="{ value }"><span class="faint mono">{{ value || '—' }}</span></template>
    <template #col-status_code="{ value }"><span class="badge" :class="value < 400 ? 'green' : 'red'">{{ value }}</span></template>
    <template #col-latency_ms="{ value }">{{ value }} ms</template>
    <template #col-source_db_duration_ms="{ value }"><span class="faint">{{ value }} ms</span></template>
    <template #col-cached="{ value }"><span class="badge" :class="value ? 'blue' : 'gray'">{{ value ? 'yes' : 'no' }}</span></template>
    <template #col-client_label="{ value }">{{ value || '—' }}</template>
    <template #col-request_id="{ value }"><span class="faint mono">{{ value }}</span></template>
  </UiTable>
</template>
