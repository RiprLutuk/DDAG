<script setup lang="ts">
definePageMeta({ title: 'Audit Logs' })
const api = useApi()

const rows = ref<any[]>([])
const loading = ref(true)
const page = ref(1)
const limit = 25
const total = ref(0)
const filters = reactive({ action: '', actor_type: '', status: '' })

const detail = ref<any>(null)
const detailOpen = ref(false)

const columns = [
  { key: 'created_at', label: 'Time' },
  { key: 'actor', label: 'Actor' },
  { key: 'action', label: 'Action' },
  { key: 'resource', label: 'Resource' },
  { key: 'ip_address', label: 'IP' },
  { key: 'status', label: 'Status' },
]
const fmt = (v: string) => (v ? new Date(v).toLocaleString() : '—')

async function load() {
  loading.value = true
  const q = new URLSearchParams({ page: String(page.value), limit: String(limit) })
  if (filters.action) q.set('action', filters.action)
  if (filters.actor_type) q.set('actor_type', filters.actor_type)
  if (filters.status) q.set('status', filters.status)
  try {
    const res = await api.list(`/api/audit-logs?${q.toString()}`)
    rows.value = res.items; total.value = res.pagination.total
  } finally { loading.value = false }
}
onMounted(load)
watch(filters, () => { page.value = 1; load() })

const pages = computed(() => Math.max(1, Math.ceil(total.value / limit)))
function go(d: number) { const n = page.value + d; if (n >= 1 && n <= pages.value) { page.value = n; load() } }
function open(row: any) { detail.value = row; detailOpen.value = true }
</script>

<template>
  <p class="page-desc">Immutable, append-only record of every important action. Security events are easy to find here.</p>
  <div class="toolbar">
    <input class="search" v-model="filters.action" placeholder="Filter by action…" />
    <select v-model="filters.actor_type" style="max-width:160px"><option value="">All actors</option><option value="user">user</option><option value="client">client</option><option value="system">system</option></select>
    <select v-model="filters.status" style="max-width:140px"><option value="">All status</option><option value="success">success</option><option value="failure">failure</option></select>
    <div class="spacer" />
    <button class="btn ghost" @click="load">Refresh</button>
  </div>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions empty="No audit events.">
    <template #col-created_at="{ value }"><span class="faint mono">{{ fmt(value) }}</span></template>
    <template #col-actor="{ row }"><div>{{ row.actor_label || row.actor_id || '—' }}<span class="faint" style="display:block;font-size:11px">{{ row.actor_type }}</span></div></template>
    <template #col-action="{ value }"><span class="mono">{{ value }}</span></template>
    <template #col-resource="{ row }"><span class="faint mono">{{ row.resource_type }} {{ row.resource_id }}</span></template>
    <template #col-status="{ value }"><StatusBadge :status="value" /></template>
    <template #actions="{ row }"><span class="actions-cell"><button class="btn icon" title="Details" @click="open(row)"><Icon name="eye" /></button></span></template>
  </UiTable>

  <div class="pagination">
    <span class="faint">{{ total }} events · page {{ page }} / {{ pages }}</span>
    <button class="btn sm ghost" :disabled="page <= 1" @click="go(-1)">Prev</button>
    <button class="btn sm ghost" :disabled="page >= pages" @click="go(1)">Next</button>
  </div>

  <UiModal :open="detailOpen" title="Audit Event" wide @close="detailOpen = false">
    <pre v-if="detail" class="code-block">{{ JSON.stringify(detail, null, 2) }}</pre>
  </UiModal>
</template>
