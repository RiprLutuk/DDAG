<script setup lang="ts">
const api = useApi()
const toast = useToast()

const rows = ref<any[]>([])
const poolStats = ref<any[]>([])
const loading = ref(true)
const error = ref('')
const services = ref<any[]>([])
const serviceWarning = computed(() => {
  const available = services.value.filter((s) => s.enabled !== false && s.last_health_status !== 'unhealthy')
  return available.some((s) => Object.keys(s.capabilities || {}).some((key) => key.startsWith('connector:')))
    ? '' : 'No healthy connector service is registered. Connections can still be saved, but tests and runtime traffic may be unavailable.'
})
const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'created_at', sortDir: 'desc' })
const columns = [
  { key: 'name', label: 'Name' },
  { key: 'database_type', label: 'Type' },
  { key: 'host', label: 'Host' },
  { key: 'pool', label: 'Pool' },
  { key: 'pool_usage', label: 'Usage' },
  { key: 'status', label: 'Status' },
  { key: 'last_health_status', label: 'Health' },
  { key: 'last_health_at', label: 'Last checked' },
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
  error.value = ''
  try {
    const [connRows, pools, serviceRows] = await Promise.all([
      api.list(`/api/connections?${new URLSearchParams({ page: String(table.page), limit: String(table.limit), search: table.search, sort_by: table.sortBy, sort_dir: table.sortDir })}`),
      api.get('/api/pool-stats').catch(() => []),
      api.list('/api/services?limit=100'),
    ])
    rows.value = connRows.items; table.total = connRows.pagination.total
    poolStats.value = pools || []
    services.value = serviceRows.items || []
  } catch (e: any) { error.value = e.message || 'Request failed' } finally { loading.value = false }
}
let healthTimer: any = null
onMounted(() => { load(); healthTimer = setInterval(load, 60000) })
onUnmounted(() => { if (healthTimer) clearInterval(healthTimer) })
function queryTable(q: any) { Object.assign(table, q); load() }
const checked = (value: string | null) => value ? new Date(value).toLocaleString() : 'Pending'

function poolFor(id: string) { return poolStats.value.find((p) => p.connection_id === id) || null }
function applyPreset(kind: string) {
  const presets: Record<string, any> = {
    vps: { min_pool_size: 0, max_pool_size: 3, connection_timeout_ms: 3000, query_timeout_ms: 12000, max_conn_lifetime_ms: 1800000, max_conn_idle_ms: 300000 },
    normal: { min_pool_size: 2, max_pool_size: 10, connection_timeout_ms: 5000, query_timeout_ms: 30000, max_conn_lifetime_ms: 3600000, max_conn_idle_ms: 1800000 },
    staging: { min_pool_size: 2, max_pool_size: 15, connection_timeout_ms: 5000, query_timeout_ms: 30000, max_conn_lifetime_ms: 3600000, max_conn_idle_ms: 1800000 },
    prod: { min_pool_size: 4, max_pool_size: 25, connection_timeout_ms: 5000, query_timeout_ms: 45000, max_conn_lifetime_ms: 3600000, max_conn_idle_ms: 1800000 },
  }
  Object.assign(form, presets[kind])
}

function openCreate() {
  Object.assign(form, blank()); editing.value = null; testResult.value = null; modalOpen.value = true
}

function onDbTypeChange() {
  const ports: Record<string, number> = { postgres: 5432, mysql: 3306, oracle: 1521, sqlserver: 1433 }
  if (!editing.value || form.port === 0 || [5432, 3306, 1521, 1433].includes(form.port)) {
    form.port = ports[form.database_type] || 5432
  }
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
  <PageHeader eyebrow="DATA SOURCES" title="Database connections" icon="database"
    description="Register external source databases, tune pool sizing, and validate connectivity before routing gateway traffic.">
    <template #actions>
      <button class="btn primary" @click="openCreate"><Icon name="plus" /> New Connection</button>
    </template>
  </PageHeader>

  <div v-if="serviceWarning" class="service-warning-banner">{{ serviceWarning }}</div>
  <UiTable :columns="columns" :rows="rows" :loading="loading" :error="error" :retry="load" has-actions remote v-bind="table" empty="No database connections yet." @query="queryTable">
    <template #col-name="{ row }"><div><b>{{ row.name }}</b><div class="faint">{{ row.environment || 'unknown environment' }}</div></div></template>
    <template #col-database_type="{ value }"><span class="badge blue">{{ value }}</span></template>
    <template #col-host="{ row }"><span class="mono">{{ row.host }}:{{ row.port }}/{{ row.database_name || row.service_name }}</span></template>
    <template #col-pool="{ row }"><span class="mono">{{ row.min_pool_size }}–{{ row.max_pool_size }}</span></template>
    <template #col-pool_usage="{ row }">
      <span v-if="poolFor(row.id)" class="mono">{{ poolFor(row.id).in_use }}/{{ poolFor(row.id).idle }}/{{ poolFor(row.id).max }}</span>
      <span v-else class="faint">—</span>
    </template>
    <template #col-status="{ value }"><StatusBadge :status="value" /></template>
    <template #col-last_health_status="{ value }"><StatusBadge :status="value || 'unknown'" /></template>
    <template #col-last_health_at="{ value }"><span class="faint">{{ checked(value) }}</span></template>
    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon go" title="Test connection" @click="testSaved(row)"><Icon name="test" /></button>
        <button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button>
        <button class="btn icon danger" title="Delete" @click="remove(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>

  <UiModal :open="modalOpen" :title="editing ? 'Edit Connection' : 'New Connection'" wide @close="modalOpen = false">
    <div class="form-section">
      <h4 class="form-section-title">Basic connection settings</h4>
      <div class="row">
        <div class="field"><label>Name</label><input v-model="form.name" placeholder="prod-billing-postgres" /></div>
        <div class="field"><label>Database Type</label>
          <select v-model="form.database_type" @change="onDbTypeChange">
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
        <div v-if="form.database_type !== 'oracle'" class="field"><label>Database Name</label><input v-model="form.database_name" placeholder="ddag_prod" /></div>
        <div v-if="form.database_type === 'oracle'" class="field"><label>Service Name / SID</label><input v-model="form.service_name" placeholder="ORCL" /></div>
        <div v-if="['postgres', 'oracle', 'sqlserver'].includes(form.database_type)" class="field"><label>Schema (optional)</label><input v-model="form.schema_name" placeholder="public" /></div>
      </div>
    </div>

    <div class="form-section">
      <h4 class="form-section-title">Credentials & security</h4>
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
    </div>

    <div class="form-section">
      <h4 class="form-section-title">Connection pool</h4>
      <div class="toolbar compact-mobile" style="padding:0;margin-bottom:10px">
      <button class="btn sm ghost" type="button" @click="applyPreset('vps')">Low VPS</button>
      <button class="btn sm ghost" type="button" @click="applyPreset('normal')">Normal</button>
      <button class="btn sm ghost" type="button" @click="applyPreset('staging')">Staging</button>
      <button class="btn sm ghost" type="button" @click="applyPreset('prod')">Production</button>
      <div class="spacer" />
      </div>
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
      <p class="form-hint">Use Low VPS for constrained hosts. Use Production only when you really need higher concurrency.</p>
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
