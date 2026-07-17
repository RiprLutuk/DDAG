<script setup lang="ts">
const api = useApi()
const toast = useToast()

const rows = ref<any[]>([])
const loading = ref(true)
const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'created_at', sortDir: 'desc' })
const scopes = ref<any[]>([])
const apis = ref<any[]>([])

const columns = [
  { key: 'client_id', label: 'Client ID', mono: true },
  { key: 'client_name', label: 'Name' },
  { key: 'environment', label: 'Env' },
  { key: 'scopes', label: 'Scopes' },
  { key: 'status', label: 'Status' },
]

const modalOpen = ref(false)
const editing = ref<any>(null)
const saving = ref(false)
const secretReveal = ref('')

const blank = () => ({
  client_id: '', client_name: '', environment: 'dev', description: '',
  access_token_ttl_seconds: 3600, refresh_token_ttl_seconds: 2592000,
  status: 'active', scopes: [] as string[], apis: [] as string[],
})
const form = reactive<any>(blank())

async function load() {
  loading.value = true
  try {
    const q = new URLSearchParams({ page: String(table.page), limit: String(table.limit), search: table.search, sort_by: table.sortBy, sort_dir: table.sortDir })
    const result = await api.list(`/api/clients?${q}`)
    rows.value = result.items; table.total = result.pagination.total
    scopes.value = (await api.list('/api/scopes')).items
    apis.value = (await api.list('/api/apis')).items
  } finally { loading.value = false }
}
onMounted(load)
function queryTable(q: any) { Object.assign(table, q); load() }

function openCreate() { Object.assign(form, blank()); editing.value = null; secretReveal.value = ''; modalOpen.value = true }
async function openEdit(row: any) {
  const c = await api.get(`/api/clients/${row.id}`)
  Object.assign(form, { ...blank(), ...c, scopes: c.scopes || [], apis: c.apis || [] })
  editing.value = row; secretReveal.value = ''; modalOpen.value = true
}

async function save() {
  saving.value = true
  try {
    if (editing.value) {
      await api.put(`/api/clients/${editing.value.id}`, form)
      await api.post(`/api/clients/${editing.value.id}/scopes`, { scopes: form.scopes })
      await api.post(`/api/clients/${editing.value.id}/apis`, { apis: form.apis })
      toast.success('Client updated'); modalOpen.value = false
    } else {
      const res = await api.post('/api/clients', form)
      secretReveal.value = res.client_secret
      toast.success('Client created', 'Copy the secret now — it is shown only once')
    }
    await load()
  } catch (e: any) { toast.error('Save failed', e.message) } finally { saving.value = false }
}

async function rotate(row: any) {
  if (!confirm(`Rotate secret for "${row.client_name}"? Existing tokens keep working until expiry; refresh tokens are revoked.`)) return
  try {
    const res = await api.post(`/api/clients/${row.id}/rotate-secret`)
    secretReveal.value = res.client_secret
    editing.value = row; Object.assign(form, await api.get(`/api/clients/${row.id}`)); modalOpen.value = true
    toast.success('Secret rotated', 'Copy the new secret now')
  } catch (e: any) { toast.error('Rotate failed', e.message) }
}

async function remove(row: any) {
  if (!confirm(`Delete client "${row.client_name}"?`)) return
  try { await api.del(`/api/clients/${row.id}`); toast.success('Deleted'); await load() }
  catch (e: any) { toast.error('Delete failed', e.message) }
}

function toggle(list: string[], v: string) {
  const i = list.indexOf(v); i >= 0 ? list.splice(i, 1) : list.push(v)
}
function copy(v: string) { navigator.clipboard?.writeText(v); toast.info('Copied to clipboard') }
</script>

<template>
  <p class="page-desc">OAuth2 client applications. Each gets its own credential, scopes, API grants, and limits.</p>
  <div class="toolbar compact-mobile"><div class="spacer" /><button class="btn primary" @click="openCreate">+ New Client</button></div>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions remote v-bind="table" empty="No clients yet." @query="queryTable">
    <template #col-environment="{ value }"><span class="badge blue">{{ value }}</span></template>
    <template #col-scopes="{ row }">
      <span v-for="s in (row.scopes || []).slice(0, 3)" :key="s" class="tag">{{ s }}</span>
      <span v-if="(row.scopes || []).length > 3" class="faint">+{{ row.scopes.length - 3 }}</span>
      <span v-if="!(row.scopes || []).length" class="faint">—</span>
    </template>
    <template #col-status="{ value }"><StatusBadge :status="value" /></template>
    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon" title="Rotate secret" @click="rotate(row)"><Icon name="rotate" /></button>
        <button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button>
        <button class="btn icon danger" title="Delete" @click="remove(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>

  <UiModal :open="modalOpen" :title="editing ? 'Edit Client' : 'New Client'" wide @close="modalOpen = false">
    <div v-if="secretReveal" class="copy-secret">
      <b style="color:#fbbf24">⚠ Client secret — shown only once</b>
      <div class="code-block" style="margin-top:8px;display:flex;justify-content:space-between;align-items:center">
        <span>{{ secretReveal }}</span>
        <button class="btn sm" @click="copy(secretReveal)">Copy</button>
      </div>
    </div>

    <div class="row">
      <div class="field"><label>Client ID</label><input v-model="form.client_id" class="mono" :disabled="!!editing" placeholder="app-billing" /></div>
      <div class="field"><label>Name</label><input v-model="form.client_name" /></div>
    </div>
    <div class="row">
      <div class="field"><label>Environment</label><select v-model="form.environment"><option>dev</option><option>staging</option><option>prod</option></select></div>
      <div class="field"><label>Status</label><select v-model="form.status"><option value="active">active</option><option value="inactive">inactive</option></select></div>
      <div class="field"><label>Access TTL (s)</label><input v-model.number="form.access_token_ttl_seconds" type="number" /></div>
      <div class="field"><label>Refresh TTL (s)</label><input v-model.number="form.refresh_token_ttl_seconds" type="number" /></div>
    </div>
    <div class="field"><label>Description</label><input v-model="form.description" /></div>

    <div class="field">
      <label>Scopes</label>
      <div>
        <span v-for="s in scopes" :key="s.id" class="tag" style="cursor:pointer"
          :style="{ background: form.scopes.includes(s.scope_code) ? 'var(--primary)' : '', color: form.scopes.includes(s.scope_code) ? '#fff' : '' }"
          @click="toggle(form.scopes, s.scope_code)">{{ s.scope_code }}</span>
        <span v-if="!scopes.length" class="faint">No scopes defined</span>
      </div>
    </div>
    <div class="field">
      <label>API Access</label>
      <div>
        <span v-for="a in apis" :key="a.id" class="tag" style="cursor:pointer"
          :style="{ background: form.apis.includes(a.id) ? 'var(--primary)' : '', color: form.apis.includes(a.id) ? '#fff' : '' }"
          @click="toggle(form.apis, a.id)">{{ a.method }} {{ a.path }}</span>
        <span v-if="!apis.length" class="faint">No APIs defined</span>
      </div>
    </div>

    <template #footer>
      <button class="btn primary" @click="save" :disabled="saving"><span v-if="saving" class="spin" /> {{ editing ? 'Save' : 'Create' }}</button>
    </template>
  </UiModal>
</template>
