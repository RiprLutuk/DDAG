<template>
  <p class="page-desc">Manage per-endpoint cache rules and purge cached responses.</p>

  <div class="toolbar compact-mobile">
    <div class="spacer" />
    <button class="btn danger" @click="purgeAll">Purge All Cache</button>
  </div>

  <UiTable
    :columns="columns"
    :rows="rows"
    :loading="loading"
    :has-actions="true"
    remote v-bind="table" @query="queryTable"
    empty="No cache rules configured."
  >
    <template #col-api="{ row }">
      <span v-if="row.api_name">{{ row.api_name }}</span>
      <span v-else-if="row.api_method || row.api_path" class="mono">{{ row.api_method }} {{ row.api_path }}</span>
      <span v-else class="faint">Deleted API</span>
    </template>

    <template #col-enabled="{ row }">
      <StatusBadge :status="row.enabled ? 'enabled' : 'disabled'" />
    </template>

    <template #col-ttl="{ row }">
      {{ row.ttl_seconds }}s
    </template>

    <template #col-vary="{ row }">
      {{ row.vary_by_client ? 'yes' : 'no' }}
    </template>

    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon danger" title="Purge cache" @click="purgeOne(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>
</template>

<script setup lang="ts">

const api = useApi()
const toast = useToast()

const rows = ref([])
const loading = ref(true)
const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'created_at', sortDir: 'desc' })

const columns = [
  { key: 'api', label: 'API' },
  { key: 'enabled', label: 'Enabled' },
  { key: 'ttl', label: 'TTL' },
  { key: 'vary', label: 'Vary by client' }
]

async function load() {
  loading.value = true
  try {
    const q = new URLSearchParams({ page: String(table.page), limit: String(table.limit), search: table.search, sort_by: table.sortBy, sort_dir: table.sortDir })
    const result = await api.list(`/api/cache/rules?${q}`)
    rows.value = result.items; table.total = result.pagination.total
  } finally {
    loading.value = false
  }
}

function queryTable(q) { Object.assign(table, q); load() }

async function purgeOne(row) {
  if (!confirm('Purge cached responses for this API?')) return
  try {
    const result = await api.post('/api/cache/purge', { api_id: row.api_definition_id })
    toast.success('Cache purged', `${result.purged} entries removed`)
    await load()
  } catch (e) {
    toast.error('Purge failed', e.message)
  }
}

async function purgeAll() {
  if (!confirm('Purge ALL cached responses? This cannot be undone.')) return
  try {
    const result = await api.post('/api/cache/purge', { all: true })
    toast.success('Cache purged', `${result.purged} entries removed`)
    await load()
  } catch (e) {
    toast.error('Purge failed', e.message)
  }
}

onMounted(load)
</script>
