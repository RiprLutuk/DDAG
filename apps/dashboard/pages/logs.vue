<script setup lang="ts">
definePageMeta({ title: 'Request Logs' })
const api = useApi()

const rows = ref<any[]>([])
const loading = ref(true)
const page = ref(1)
const limit = 25
const total = ref(0)

const columns = [
  { key: 'created_at', label: 'Time' },
  { key: 'method', label: 'Method' },
  { key: 'path', label: 'Path', mono: true },
  { key: 'status_code', label: 'Status' },
  { key: 'latency_ms', label: 'Latency' },
  { key: 'source_db_duration_ms', label: 'Source DB' },
  { key: 'cached', label: 'Cached' },
  { key: 'client_label', label: 'Client' },
  { key: 'ip_address', label: 'IP' },
]
const fmt = (v: string) => (v ? new Date(v).toLocaleString() : '—')

async function load() {
  loading.value = true
  try {
    const res = await api.list(`/api/request-logs?page=${page.value}&limit=${limit}`)
    rows.value = res.items; total.value = res.pagination.total
  } finally { loading.value = false }
}
onMounted(load)
const pages = computed(() => Math.max(1, Math.ceil(total.value / limit)))
function go(d: number) { const n = page.value + d; if (n >= 1 && n <= pages.value) { page.value = n; load() } }
</script>

<template>
  <p class="page-desc">Every data-plane request through the gateway, with latency, cache and source-DB timing.</p>
  <div class="toolbar"><div class="spacer" /><button class="btn ghost" @click="load">Refresh</button></div>

  <UiTable :columns="columns" :rows="rows" :loading="loading" empty="No requests logged yet.">
    <template #col-created_at="{ value }"><span class="faint mono">{{ fmt(value) }}</span></template>
    <template #col-method="{ value }"><span class="badge gray">{{ value }}</span></template>
    <template #col-status_code="{ value }"><span class="badge" :class="value < 400 ? 'green' : 'red'">{{ value }}</span></template>
    <template #col-latency_ms="{ value }">{{ value }} ms</template>
    <template #col-source_db_duration_ms="{ value }"><span class="faint">{{ value }} ms</span></template>
    <template #col-cached="{ value }"><span class="badge" :class="value ? 'blue' : 'gray'">{{ value ? 'yes' : 'no' }}</span></template>
    <template #col-client_label="{ value }">{{ value || '—' }}</template>
  </UiTable>

  <div class="pagination">
    <span class="faint">{{ total }} requests · page {{ page }} / {{ pages }}</span>
    <button class="btn sm ghost" :disabled="page <= 1" @click="go(-1)">Prev</button>
    <button class="btn sm ghost" :disabled="page >= pages" @click="go(1)">Next</button>
  </div>
</template>
