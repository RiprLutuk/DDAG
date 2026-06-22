<script setup lang="ts">
definePageMeta({ title: 'Database Connections' })
const api = useApi()
const toast = useToast()

const rows = ref<any[]>([])
const loading = ref(true)
const columns = [
  { key: 'name', label: 'Name' },
  { key: 'database_type', label: 'Type' },
  { key: 'host', label: 'Host' },
  { key: 'pool', label: 'Pool' },
  { key: 'status', label: 'Status' },
  { key: 'last_health_status', label: 'Health' },
]

const modalOpen = ref(false)
const editing = ref<any>(null)
const testResult = ref<{ ok: boolean; msg: string } | null>(null)
const saving = ref(false)
const testing = ref(false)

const blank = () => ({
  name: '', database_type: 'postgres', host: '', port: 5432, database_name: '', service_name: '',
  schema_name: '', username: '', password: '', ssl_mode: 'disable', min_pool_size: 2, max_pool_size: 10,
  connection_timeout_ms: 5000, query_timeout_ms: 30000, max_conn_lifetime_ms: 3600000,
  max_conn_idle_ms: 1800000, environment: 'dev', status: 'active', tags: [],
})
const form = reactive<any>(blank())

async function load() {
  loading.value = true
  try { rows.value = (await api.list('/api/connections')).items } finally { loading.value = false }
}
onMounted(load)

function openCreate() {
  Object.assign(form, blank()); editing.value = null; testResult.value = null; modalOpen.value = true
}
async function openEdit(row: any) {
  const c = await api.get(`/api/connections/${row.id}`)
  Object.assign(form, { ...c, password: '' }) // password not returned; leave blank to keep
  editing.value = row; testResult.value = null; modalOpen.value = true
}

async function testNow() {
  testing.value = true; testResult.value = null
  try {
    const res = await api.post('/api/connections/test', { ...form })
    testResult.value = { ok: !!res.success, msg: res.message || (res.success ? 'OK' : 'Failed') }
  } catch (e: any) { testResult.value = { ok: false, msg: e.message } } finally { testing.value = false }
}

async function save() {
  saving.value = true
  try {
    if (editing.value) await api.put(`/api/connections/${editing.value.id}`, form)
    else await api.post('/api/connections', form)
    toast.success('Connection saved')
    modalOpen.value = false; await load()
  } catch (e: any) { toast.error('Save failed', e.message) } finally { saving.value = false }
}

async function testSaved(row: any) {
  try {
    const res = await api.post(`/api/connections/${row.id}/test`)
    res.success ? toast.success('Connection healthy', `${row.name} reachable`) : toast.error('Unhealthy', res.message)
    await load()
  } catch (e: any) { toast.error('Test failed', e.message) }
}

async function remove(row: any) {
  if (!confirm(`Delete connection "${row.name}"?`)) return
  try { await api.del(`/api/connections/${row.id}`); toast.success('Deleted'); await load() }
  catch (e: any) { toast.error('Delete failed', e.message) }
}
</script>

<template>
  <p class="page-desc">Connections to external source databases. Each has its own connection pool, tuned independently.</p>
  <div class="toolbar">
    <div class="spacer" />
    <button class="btn primary" @click="openCreate">+ New Connection</button>
  </div>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions empty="No database connections yet.">
    <template #col-database_type="{ value }"><span class="badge blue">{{ value }}</span></template>
    <template #col-host="{ row }"><span class="mono">{{ row.host }}:{{ row.port }}/{{ row.database_name || row.service_name }}</span></template>
    <template #col-pool="{ row }"><span class="mono">{{ row.min_pool_size }}–{{ row.max_pool_size }}</span></template>
    <template #col-status="{ value }"><StatusBadge :status="value" /></template>
    <template #col-last_health_status="{ value }"><StatusBadge :status="value" /></template>
    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon go" title="Test connection" @click="testSaved(row)"><Icon name="test" /></button>
        <button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button>
        <button class="btn icon danger" title="Delete" @click="remove(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>

  <UiModal :open="modalOpen" :title="editing ? 'Edit Connection' : 'New Connection'" wide @close="modalOpen = false">
    <div class="row">
      <div class="field"><label>Name</label><input v-model="form.name" placeholder="prod-billing-postgres" /></div>
      <div class="field"><label>Database Type</label>
        <select v-model="form.database_type">
          <option value="postgres">PostgreSQL</option>
          <option value="mysql">MySQL / MariaDB</option>
          <option value="oracle">Oracle</option>
          <option value="sqlserver">SQL Server</option>
        </select>
      </div>
    </div>
    <div class="row">
      <div class="field" style="flex:2"><label>Host</label><input v-model="form.host" placeholder="db.internal" /></div>
      <div class="field"><label>Port</label><input v-model.number="form.port" type="number" /></div>
    </div>
    <div class="row">
      <div class="field"><label>Database Name</label><input v-model="form.database_name" /></div>
      <div class="field"><label>Service Name / SID (Oracle)</label><input v-model="form.service_name" /></div>
      <div class="field"><label>Schema</label><input v-model="form.schema_name" /></div>
    </div>
    <div class="row">
      <div class="field"><label>Username</label><input v-model="form.username" autocomplete="off" /></div>
      <div class="field"><label>Password {{ editing ? '(leave blank to keep)' : '' }}</label><input v-model="form.password" type="password" autocomplete="new-password" /></div>
      <div class="field"><label>SSL Mode</label>
        <select v-model="form.ssl_mode">
          <option value="disable">disable</option><option value="require">require</option>
          <option value="verify-ca">verify-ca</option><option value="verify-full">verify-full</option>
        </select>
      </div>
    </div>
    <h4 style="margin:8px 0 10px;color:var(--text-dim)">Connection Pool</h4>
    <div class="row">
      <div class="field"><label>Min Pool</label><input v-model.number="form.min_pool_size" type="number" /></div>
      <div class="field"><label>Max Pool</label><input v-model.number="form.max_pool_size" type="number" /></div>
      <div class="field"><label>Connect Timeout (ms)</label><input v-model.number="form.connection_timeout_ms" type="number" /></div>
      <div class="field"><label>Query Timeout (ms)</label><input v-model.number="form.query_timeout_ms" type="number" /></div>
    </div>
    <div class="row">
      <div class="field"><label>Max Conn Lifetime (ms)</label><input v-model.number="form.max_conn_lifetime_ms" type="number" /></div>
      <div class="field"><label>Max Conn Idle (ms)</label><input v-model.number="form.max_conn_idle_ms" type="number" /></div>
      <div class="field"><label>Environment</label>
        <select v-model="form.environment"><option>dev</option><option>staging</option><option>prod</option></select>
      </div>
      <div class="field"><label>Status</label>
        <select v-model="form.status"><option value="active">active</option><option value="inactive">inactive</option></select>
      </div>
    </div>

    <div v-if="testResult" class="copy-secret" :style="{ borderColor: testResult.ok ? 'rgba(34,197,94,.5)' : 'rgba(239,68,68,.5)', background: testResult.ok ? 'rgba(34,197,94,.08)' : 'rgba(239,68,68,.08)' }">
      <b :style="{ color: testResult.ok ? '#4ade80' : '#f87171' }">{{ testResult.ok ? '✓ Connection successful' : '✗ Connection failed' }}</b>
      <div class="muted">{{ testResult.msg }}</div>
    </div>

    <template #footer>
      <button class="btn ghost" @click="testNow" :disabled="testing"><span v-if="testing" class="spin" /> Test Connection</button>
      <button class="btn primary" @click="save" :disabled="saving"><span v-if="saving" class="spin" /> Save</button>
    </template>
  </UiModal>
</template>
