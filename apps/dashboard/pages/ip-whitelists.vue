<template>
  <p class="page-desc">Empty whitelist = no restriction (allow all).</p>

  <div class="toolbar">
    <div class="spacer" />
    <button class="btn primary" @click="openCreate">+ New</button>
  </div>

  <UiTable
    :columns="columns"
    :rows="rows"
    :loading="loading"
    :has-actions="true"
    empty="No IP whitelist entries."
  >
    <template #col-ip_cidr="{ value }">
      <span class="mono">{{ value }}</span>
    </template>

    <template #col-scope_level="{ value }">
      <span class="badge" :class="scopeClass(value)">{{ value || '—' }}</span>
    </template>

    <template #col-client_id="{ value }">
      {{ clientName(value) }}
    </template>

    <template #col-api_definition_id="{ value }">
      {{ apiName(value) }}
    </template>

    <template #col-status="{ value }">
      <StatusBadge :status="value" />
    </template>

    <template #col-description="{ value }">
      <span :class="{ faint: !value }">{{ value || '—' }}</span>
    </template>

    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button>
        <button class="btn icon danger" title="Delete" @click="remove(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>

  <UiModal :open="modalOpen" :title="editing ? 'Edit IP Whitelist' : 'New IP Whitelist'" @close="modalOpen = false">
    <div class="field">
      <label>IP / CIDR</label>
      <input v-model="form.ip_cidr" type="text" placeholder="10.0.0.0/24 or 1.2.3.4" />
    </div>

    <div class="field">
      <label>Scope Level</label>
      <select v-model="form.scope_level">
        <option value="global">global</option>
        <option value="client">client</option>
        <option value="api">api</option>
      </select>
    </div>

    <div class="row">
      <div class="field">
        <label>Client (optional)</label>
        <select v-model="form.client_id">
          <option :value="null">any</option>
          <option v-for="c in clients" :key="c.id" :value="c.id">{{ clientLabel(c) }}</option>
        </select>
      </div>

      <div class="field">
        <label>API (optional)</label>
        <select v-model="form.api_definition_id">
          <option :value="null">any</option>
          <option v-for="a in apis" :key="a.id" :value="a.id">{{ apiLabel(a) }}</option>
        </select>
      </div>
    </div>

    <div class="field">
      <label>Status</label>
      <select v-model="form.status">
        <option value="active">active</option>
        <option value="inactive">inactive</option>
      </select>
    </div>

    <div class="field">
      <label>Description</label>
      <textarea v-model="form.description" rows="3" placeholder="Optional description" />
    </div>

    <template #footer>
      <button class="btn ghost" @click="modalOpen = false">Cancel</button>
      <button class="btn primary" :disabled="saving" @click="save">
        {{ saving ? 'Saving…' : 'Save' }}
      </button>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
definePageMeta({ title: 'IP Whitelist' })

const api = useApi()
const toast = useToast()

const columns = [
  { key: 'ip_cidr', label: 'IP / CIDR', mono: true },
  { key: 'scope_level', label: 'Scope Level' },
  { key: 'client_id', label: 'Client' },
  { key: 'api_definition_id', label: 'API' },
  { key: 'status', label: 'Status' },
  { key: 'description', label: 'Description' },
]

const rows = ref<any[]>([])
const loading = ref(true)

const clients = ref<any[]>([])
const apis = ref<any[]>([])

const clientMap = computed<Record<string, string>>(() => {
  const m: Record<string, string> = {}
  for (const c of clients.value) m[c.id] = clientLabel(c)
  return m
})

const apiMap = computed<Record<string, string>>(() => {
  const m: Record<string, string> = {}
  for (const a of apis.value) m[a.id] = apiLabel(a)
  return m
})

function clientLabel(c: any) {
  return c.name || c.client_name || c.label || c.id
}

function apiLabel(a: any) {
  return a.name || a.label || a.title || a.id
}

function clientName(id: any) {
  if (!id) return 'any'
  return clientMap.value[id] || id
}

function apiName(id: any) {
  if (!id) return 'any'
  return apiMap.value[id] || id
}

function scopeClass(scope: string) {
  if (scope === 'global') return 'blue'
  if (scope === 'client') return 'amber'
  if (scope === 'api') return 'green'
  return 'gray'
}

async function load() {
  loading.value = true
  try {
    rows.value = (await api.list('/api/ip-whitelists')).items
  } finally {
    loading.value = false
  }
}

async function loadRefs() {
  try {
    const [c, a] = await Promise.all([
      api.list('/api/clients'),
      api.list('/api/apis'),
    ])
    clients.value = c.items
    apis.value = a.items
  } catch (e: any) {
    toast.error('Failed to load references', e.message)
  }
}

const modalOpen = ref(false)
const editing = ref<any>(null)
const saving = ref(false)

function blank() {
  return {
    client_id: null as string | null,
    api_definition_id: null as string | null,
    ip_cidr: '',
    scope_level: 'global',
    status: 'active',
    description: '',
  }
}

const form = reactive(blank())

function resetForm() {
  Object.assign(form, blank())
}

function openCreate() {
  editing.value = null
  resetForm()
  modalOpen.value = true
}

async function openEdit(row: any) {
  editing.value = row
  resetForm()
  modalOpen.value = true
  try {
    const data = await api.get(`/api/ip-whitelists/${row.id}`)
    Object.assign(form, {
      client_id: data.client_id ?? null,
      api_definition_id: data.api_definition_id ?? null,
      ip_cidr: data.ip_cidr ?? '',
      scope_level: data.scope_level ?? 'global',
      status: data.status ?? 'active',
      description: data.description ?? '',
    })
  } catch (e: any) {
    toast.error('Failed to load entry', e.message)
  }
}

async function save() {
  saving.value = true
  try {
    const payload = {
      client_id: form.client_id || null,
      api_definition_id: form.api_definition_id || null,
      ip_cidr: form.ip_cidr,
      scope_level: form.scope_level,
      status: form.status,
      description: form.description,
    }
    if (editing.value) {
      await api.put(`/api/ip-whitelists/${editing.value.id}`, payload)
      toast.success('Whitelist entry updated')
    } else {
      await api.post('/api/ip-whitelists', payload)
      toast.success('Whitelist entry created')
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
  if (!confirm(`Delete whitelist entry "${row.ip_cidr}"?`)) return
  try {
    await api.del(`/api/ip-whitelists/${row.id}`)
    toast.success('Whitelist entry deleted')
    await load()
  } catch (e: any) {
    toast.error('Delete failed', e.message)
  }
}

onMounted(() => {
  load()
  loadRefs()
})
</script>
