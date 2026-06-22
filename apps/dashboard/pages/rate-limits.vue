<template>
  <p class="page-desc">
    Configure rate limit policies applied to specific clients, APIs, IP addresses, or globally.
    Set zero on any window to leave it unlimited.
  </p>

  <div class="toolbar">
    <div class="spacer" />
    <button class="btn primary" @click="openCreate">+ New</button>
  </div>

  <UiTable
    :columns="columns"
    :rows="rows"
    :loading="loading"
    :has-actions="true"
    empty="No rate limits defined yet."
  >
    <template #col-applies_to="{ value }">
      <StatusBadge :status="value" />
    </template>

    <template #col-client_id="{ value }">
      <span :class="{ faint: !value }">{{ clientLabel(value) }}</span>
    </template>

    <template #col-api_definition_id="{ value }">
      <span :class="{ faint: !value }" class="mono">{{ apiLabel(value) }}</span>
    </template>

    <template #col-requests_per_second="{ value }">
      <span :class="{ faint: !value }">{{ rateLabel(value) }}</span>
    </template>

    <template #col-requests_per_minute="{ value }">
      <span :class="{ faint: !value }">{{ rateLabel(value) }}</span>
    </template>

    <template #col-requests_per_hour="{ value }">
      <span :class="{ faint: !value }">{{ rateLabel(value) }}</span>
    </template>

    <template #col-requests_per_day="{ value }">
      <span :class="{ faint: !value }">{{ rateLabel(value) }}</span>
    </template>

    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button>
        <button class="btn icon danger" title="Delete" @click="remove(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>

  <UiModal :open="modalOpen" :title="editingId ? 'Edit Rate Limit' : 'New Rate Limit'" @close="modalOpen = false">
    <div class="field">
      <label>Applies To</label>
      <select v-model="form.applies_to">
        <option value="client">client</option>
        <option value="api">api</option>
        <option value="ip">ip</option>
        <option value="global">global</option>
      </select>
    </div>

    <div class="field">
      <label>Client</label>
      <select v-model="form.client_id">
        <option value="">— any —</option>
        <option v-for="c in clients" :key="c.id" :value="c.id">
          {{ c.client_name }} ({{ c.client_id }})
        </option>
      </select>
    </div>

    <div class="field">
      <label>API</label>
      <select v-model="form.api_definition_id">
        <option value="">— any —</option>
        <option v-for="a in apis" :key="a.id" :value="a.id">
          {{ a.method }} {{ a.path }}
        </option>
      </select>
    </div>

    <div class="row">
      <div class="field">
        <label>Requests / second</label>
        <input v-model.number="form.requests_per_second" type="number" min="0" placeholder="0 = unlimited" />
      </div>
      <div class="field">
        <label>Requests / minute</label>
        <input v-model.number="form.requests_per_minute" type="number" min="0" placeholder="0 = unlimited" />
      </div>
    </div>

    <div class="row">
      <div class="field">
        <label>Requests / hour</label>
        <input v-model.number="form.requests_per_hour" type="number" min="0" placeholder="0 = unlimited" />
      </div>
      <div class="field">
        <label>Requests / day</label>
        <input v-model.number="form.requests_per_day" type="number" min="0" placeholder="0 = unlimited" />
      </div>
    </div>

    <p class="faint">A value of 0 leaves that window unlimited.</p>

    <template #footer>
      <button class="btn ghost" @click="modalOpen = false">Cancel</button>
      <button class="btn primary" :disabled="saving" @click="save">
        {{ saving ? 'Saving…' : 'Save' }}
      </button>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
definePageMeta({ title: 'Rate Limits' })

const api = useApi()
const toast = useToast()

const rows = ref<any[]>([])
const loading = ref(true)

const clients = ref<any[]>([])
const apis = ref<any[]>([])

const clientMap = computed<Record<string, string>>(() => {
  const m: Record<string, string> = {}
  for (const c of clients.value) m[c.id] = c.client_name
  return m
})

const apiMap = computed<Record<string, string>>(() => {
  const m: Record<string, string> = {}
  for (const a of apis.value) m[a.id] = `${a.method} ${a.path}`
  return m
})

const columns = [
  { key: 'applies_to', label: 'Applies To' },
  { key: 'client_id', label: 'Client' },
  { key: 'api_definition_id', label: 'API' },
  { key: 'requests_per_second', label: 'Per Second' },
  { key: 'requests_per_minute', label: 'Per Minute' },
  { key: 'requests_per_hour', label: 'Per Hour' },
  { key: 'requests_per_day', label: 'Per Day' }
]

function clientLabel(id: string | null) {
  return id ? (clientMap.value[id] || id) : 'any'
}

function apiLabel(id: string | null) {
  return id ? (apiMap.value[id] || id) : 'any'
}

function rateLabel(v: number | null) {
  return v ? String(v) : '∞'
}

async function load() {
  loading.value = true
  try {
    const [rl, cl, ap] = await Promise.all([
      api.list('/api/rate-limits'),
      api.list('/api/clients'),
      api.list('/api/apis')
    ])
    rows.value = rl.items
    clients.value = cl.items
    apis.value = ap.items
  } catch (e: any) {
    toast.error('Failed to load rate limits', e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)

const modalOpen = ref(false)
const saving = ref(false)
const editingId = ref<string | null>(null)

function blank() {
  return {
    client_id: '',
    api_definition_id: '',
    applies_to: 'client',
    requests_per_second: 0,
    requests_per_minute: 0,
    requests_per_hour: 0,
    requests_per_day: 0
  }
}

const form = reactive(blank())

function reset() {
  Object.assign(form, blank())
}

function openCreate() {
  editingId.value = null
  reset()
  modalOpen.value = true
}

async function openEdit(row: any) {
  editingId.value = row.id
  reset()
  try {
    const data = await api.get(`/api/rate-limits/${row.id}`)
    Object.assign(form, {
      client_id: data.client_id || '',
      api_definition_id: data.api_definition_id || '',
      applies_to: data.applies_to || 'client',
      requests_per_second: data.requests_per_second || 0,
      requests_per_minute: data.requests_per_minute || 0,
      requests_per_hour: data.requests_per_hour || 0,
      requests_per_day: data.requests_per_day || 0
    })
    modalOpen.value = true
  } catch (e: any) {
    toast.error('Failed to load rate limit', e.message)
  }
}

async function save() {
  saving.value = true
  try {
    const payload = {
      client_id: form.client_id,
      api_definition_id: form.api_definition_id,
      applies_to: form.applies_to,
      requests_per_second: Number(form.requests_per_second) || 0,
      requests_per_minute: Number(form.requests_per_minute) || 0,
      requests_per_hour: Number(form.requests_per_hour) || 0,
      requests_per_day: Number(form.requests_per_day) || 0
    }
    if (editingId.value) {
      await api.put(`/api/rate-limits/${editingId.value}`, payload)
      toast.success('Rate limit updated')
    } else {
      await api.post('/api/rate-limits', payload)
      toast.success('Rate limit created')
    }
    modalOpen.value = false
    await load()
  } catch (e: any) {
    toast.error('Save failed', e.message)
  } finally {
    saving.value = false
  }
}

async function remove(row: any) {
  if (!confirm('Delete this rate limit? This cannot be undone.')) return
  try {
    await api.del(`/api/rate-limits/${row.id}`)
    toast.success('Rate limit deleted')
    await load()
  } catch (e: any) {
    toast.error('Delete failed', e.message)
  }
}
</script>
