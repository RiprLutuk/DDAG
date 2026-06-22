<template>
  <p class="page-desc">Manage per-endpoint cache rules and purge cached responses.</p>

  <div class="toolbar">
    <div class="spacer" />
    <button class="btn danger" @click="purgeAll">Purge All Cache</button>
  </div>

  <UiTable
    :columns="columns"
    :rows="rows"
    :loading="loading"
    :has-actions="true"
    empty="No cache rules configured."
  >
    <template #col-api="{ row }">
      <span v-if="apiMap[row.api_definition_id]" class="mono">{{ apiMap[row.api_definition_id] }}</span>
      <span v-else class="mono faint">{{ row.api_definition_id }}</span>
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
definePageMeta({ title: 'Cache' })

const api = useApi()
const toast = useToast()

const rows = ref([])
const loading = ref(true)
const apiMap = reactive({})

const columns = [
  { key: 'api', label: 'API' },
  { key: 'enabled', label: 'Enabled' },
  { key: 'ttl', label: 'TTL' },
  { key: 'vary', label: 'Vary by client' }
]

async function loadApis() {
  try {
    const { items } = await api.list('/api/apis')
    for (const a of items) {
      apiMap[a.id] = `${a.method} ${a.path}`
    }
  } catch (e) {
    toast.error('Failed to load APIs', e.message)
  }
}

async function load() {
  loading.value = true
  try {
    await loadApis()
    rows.value = (await api.list('/api/cache/rules')).items
  } finally {
    loading.value = false
  }
}

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
