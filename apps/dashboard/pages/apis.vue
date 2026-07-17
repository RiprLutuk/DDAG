<script setup lang="ts">
import { joinGatewayPath } from '../src/url-utils.js'

const api = useApi()
const toast = useToast()
const gatewayBase = import.meta.env.VITE_GATEWAY_BASE || '/api/v1'
const authBase = import.meta.env.VITE_AUTH_BASE || '/oauth'

const rows = ref<any[]>([])
const loading = ref(true)
const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'created_at', sortDir: 'desc' })
const connections = ref<any[]>([])
const scopes = ref<any[]>([])

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'route', label: 'Route' },
  { key: 'connection_name', label: 'Connection' },
  { key: 'required_scope', label: 'Scope' },
  { key: 'status', label: 'Status' },
]

const modalOpen = ref(false)
const editing = ref<any>(null)
const saving = ref(false)
const approvalComment = ref('')
const diffModal = ref(false)
const diffResult = ref<any | null>(null)
const promotionModal = ref(false)
const promotionPayload = ref('')
const promotionResult = ref<any | null>(null)
const testing = ref(false)
const previewing = ref(false)
const explaining = ref(false)
const testRows = ref<any[] | null>(null)
const testError = ref('')
const previewResult = ref<any | null>(null)
const previewError = ref('')
const explainResult = ref<any | null>(null)
const previewQueryString = ref('')
const sampleParams = reactive<Record<string, any>>({})

const blank = () => ({
  name: '', namespace: '', path: '', method: 'GET', description: '', database_connection_id: '',
  query_template: '', required_scope: '', default_limit: 100, max_limit: 1000, is_write: false,
  response_mapping: null as any, query_builder_json: '',
  parameters: [] as any[],
})
const form = reactive<any>(blank())

const cacheModal = ref(false)
const cacheApi = ref<any>(null)
const cacheForm = reactive<any>({ enabled: false, ttl_seconds: 300, vary_by_client: true, cache_key_strategy: 'client_id:path:query_params' })

async function load() {
  loading.value = true
  try {
    const q = new URLSearchParams({ page: String(table.page), limit: String(table.limit), search: table.search, sort_by: table.sortBy, sort_dir: table.sortDir })
    const result = await api.list(`/api/apis?${q}`)
    rows.value = result.items; table.total = result.pagination.total
    connections.value = (await api.list('/api/connections')).items
    scopes.value = (await api.list('/api/scopes')).items
  } finally { loading.value = false }
}
onMounted(load)
function queryTable(q: any) { Object.assign(table, q); load() }

function openCreate() { Object.assign(form, blank()); editing.value = null; testRows.value = null; testError.value = ''; modalOpen.value = true }
async function openEdit(row: any) {
  const a = await api.get(`/api/apis/${row.id}`)
  const qb = a.response_mapping?.query_builder || null
  Object.assign(form, { ...blank(), ...a, database_connection_id: a.database_connection_id || '', parameters: a.parameters || [], query_builder_json: qb ? JSON.stringify(qb, null, 2) : '' })
  editing.value = row; testRows.value = null; testError.value = ''; previewResult.value = null; previewError.value = ''; explainResult.value = null; modalOpen.value = true
}
function addParam() { form.parameters.push({ name: '', source: 'path', param_type: 'string', required: true, max_length: null, default_value: null, validation_rule: null }) }
function removeParam(i: number) { form.parameters.splice(i, 1) }

function parseQueryString() {
  const out: Record<string, string> = {}
  const qs = previewQueryString.value.trim().replace(/^\?/, '')
  if (!qs) return out
  for (const [k, v] of new URLSearchParams(qs).entries()) out[k] = v
  return out
}

function responseMapping() {
  const raw = (form.query_builder_json || '').trim()
  if (!raw) return null
  const parsed = JSON.parse(raw)
  return parsed.query_builder ? parsed : { query_builder: parsed }
}

function apiPayload() {
  const payload: any = { ...form }
  payload.is_write = ['POST', 'PUT', 'PATCH', 'DELETE'].includes(payload.method)
  delete payload.query_builder_json
  payload.response_mapping = responseMapping()
  return payload
}

async function runTest() {
  testing.value = true; testRows.value = null; testError.value = ''
  try {
    const res = await api.post('/api/apis/test', {
      connection_id: form.database_connection_id, query_template: form.query_template,
      response_mapping: responseMapping(), query: parseQueryString(),
      parameters: { ...sampleParams }, limit: form.default_limit,
    })
    if (res.success) testRows.value = res.rows || []
    else testError.value = res.error?.message || 'Query failed'
  } catch (e: any) { testError.value = e.message } finally { testing.value = false }
}

async function previewSQL() {
  previewing.value = true; previewResult.value = null; previewError.value = ''
  try {
    previewResult.value = await api.post('/api/apis/preview', {
      api: apiPayload(), query: parseQueryString(), parameters: { ...sampleParams },
    })
  } catch (e: any) { previewError.value = e.message } finally { previewing.value = false }
}

async function explainSQL() {
  explaining.value = true; explainResult.value = null; previewError.value = ''
  try {
    explainResult.value = await api.post('/api/apis/explain', {
      connection_id: form.database_connection_id, api: apiPayload(), query: parseQueryString(), parameters: { ...sampleParams },
    })
  } catch (e: any) { previewError.value = e.message } finally { explaining.value = false }
}

async function save() {
  saving.value = true
  try {
    const payload = apiPayload()
    if (editing.value) await api.put(`/api/apis/${editing.value.id}`, payload)
    else await api.post('/api/apis', payload)
    toast.success('API saved as draft')
    modalOpen.value = false; await load()
  } catch (e: any) { toast.error('Save failed', e.message) } finally { saving.value = false }
}

async function publish(row: any) {
  try { await api.post(`/api/apis/${row.id}/publish`); toast.success('Published', row.name); await load() }
  catch (e: any) { toast.error('Publish blocked', e.message) }
}
async function sendForReview(row: any) {
  try { await api.post(`/api/apis/${row.id}/review`); toast.info('Sent for review', row.name); await load() }
  catch (e: any) { toast.error('Review failed', e.message) }
}
async function approve(row: any) {
  try {
    await api.post(`/api/apis/${row.id}/approve`, { comment: approvalComment.value || 'Approved from dashboard' })
    toast.success('Approved', row.name)
    approvalComment.value = ''
    await load()
  } catch (e: any) { toast.error('Approve failed', e.message) }
}
async function showDiff(row: any) {
  try {
    diffResult.value = await api.get(`/api/apis/${row.id}/diff`)
    diffModal.value = true
  } catch (e: any) { toast.error('Diff unavailable', e.message) }
}
async function exportPromotionBundle() {
  try {
    const bundle = await api.get('/api/apis/promotion/export')
    promotionPayload.value = JSON.stringify(bundle, null, 2)
    promotionResult.value = null
    promotionModal.value = true
  } catch (e: any) { toast.error('Export failed', e.message) }
}
async function dryRunPromotion() {
  try {
    promotionResult.value = await api.post('/api/apis/promotion/import/dry-run', JSON.parse(promotionPayload.value || '{}'))
    toast.success('Dry-run complete')
  } catch (e: any) { toast.error('Dry-run failed', e.message) }
}
async function disable(row: any) {
  try { await api.post(`/api/apis/${row.id}/disable`); toast.info('Disabled', row.name); await load() }
  catch (e: any) { toast.error('Failed', e.message) }
}
async function remove(row: any) {
  if (!confirm(`Delete API "${row.name}"?`)) return
  try { await api.del(`/api/apis/${row.id}`); toast.success('Deleted'); await load() }
  catch (e: any) { toast.error('Delete failed', e.message) }
}

async function openCache(row: any) {
  cacheApi.value = row
  const cr = await api.get(`/api/apis/${row.id}/cache`)
  Object.assign(cacheForm, cr)
  cacheModal.value = true
}
async function saveCache() {
  try { await api.put(`/api/apis/${cacheApi.value.id}/cache`, cacheForm); toast.success('Cache rule saved'); cacheModal.value = false }
  catch (e: any) { toast.error('Failed', e.message) }
}

// ---- API consumption docs (Swagger-style) ----
const docsModal = ref(false)
const docsApi = ref<any>(null)

// ---- Live API playground ----
const playgroundModal = ref(false)
const playgroundApi = ref<any>(null)
const playgroundMethod = ref('GET')
const playgroundHeaders = ref([{ key: 'Authorization', value: 'Bearer ' }])
const playgroundParams = ref<{ key: string; value: string }[]>([])
const playgroundBody = ref('{}')
const playgroundSending = ref(false)
const playgroundError = ref('')
const playgroundResponse = ref<any>(null)

function resetPlaygroundRequest(endpoint: any) {
  playgroundMethod.value = endpoint.method || 'GET'
  playgroundHeaders.value = [{ key: 'Authorization', value: 'Bearer ' }]
  playgroundParams.value = (endpoint.parameters || [])
    .filter((p: any) => p.source === 'query')
    .map((p: any) => ({ key: p.name, value: '' }))
  const body: Record<string, string> = {}
  for (const p of (endpoint.parameters || []).filter((p: any) => p.source === 'body')) body[p.name] = ''
  playgroundBody.value = JSON.stringify(body, null, 2)
  playgroundError.value = ''
  playgroundResponse.value = null
}

async function openPlayground(row: any) {
  if (row.status !== 'PUBLISHED') {
    toast.error('Playground unavailable', 'Only published APIs can be called through the live gateway.')
    return
  }
  try {
    playgroundApi.value = await api.get(`/api/apis/${row.id}`)
    resetPlaygroundRequest(playgroundApi.value)
    playgroundModal.value = true
  } catch (e: any) { toast.error('Could not open playground', e.message) }
}

function addPlaygroundHeader() { playgroundHeaders.value.push({ key: '', value: '' }) }
function addPlaygroundParam() { playgroundParams.value.push({ key: '', value: '' }) }
function removePlaygroundRow(rows: { key: string; value: string }[], index: number) { rows.splice(index, 1) }

const playgroundUrl = computed(() => {
  const endpoint = playgroundApi.value
  if (!endpoint) return ''
  let path = endpoint.path
  for (const p of (endpoint.parameters || []).filter((p: any) => p.source === 'path')) {
    path = path.replace(`{${p.name}}`, encodeURIComponent(sampleVal(p)))
  }
  const query = new URLSearchParams()
  for (const param of playgroundParams.value) if (param.key.trim()) query.append(param.key.trim(), param.value)
  return `${joinGatewayPath(gatewayBase, path)}${query.toString() ? `?${query}` : ''}`
})

async function sendPlaygroundRequest() {
  playgroundError.value = ''
  playgroundResponse.value = null
  const headers: Record<string, string> = {}
  for (const header of playgroundHeaders.value) if (header.key.trim()) headers[header.key.trim()] = header.value

  const method = playgroundMethod.value.toUpperCase()
  const usesBody = !['GET', 'HEAD'].includes(method)
  let body: string | undefined
  if (usesBody && playgroundBody.value.trim()) {
    try { body = JSON.stringify(JSON.parse(playgroundBody.value)) }
    catch { playgroundError.value = 'Request body must be valid JSON.'; return }
    if (!Object.keys(headers).some(key => key.toLowerCase() === 'content-type')) headers['Content-Type'] = 'application/json'
  }

  playgroundSending.value = true
  const startedAt = performance.now()
  try {
    const response = await fetch(playgroundUrl.value, { method, headers, body })
    const latency = Math.round(performance.now() - startedAt)
    const text = await response.text()
    let data: any = text
    try { data = text ? JSON.parse(text) : null } catch { /* Preserve non-JSON gateway responses. */ }
    playgroundResponse.value = {
      status: response.status,
      statusText: response.statusText,
      ok: response.ok,
      latency,
      headers: Object.fromEntries(response.headers.entries()),
      body: data,
    }
  } catch (e: any) {
    playgroundError.value = e.message || 'The request could not reach the gateway.'
  } finally { playgroundSending.value = false }
}

async function openDocs(row: any) {
  docsApi.value = await api.get(`/api/apis/${row.id}`)
  docsModal.value = true
}
function sampleVal(p: any) {
  switch (p.param_type) {
    case 'int': case 'number': return '123'
    case 'bool': return 'true'
    case 'uuid': return '00000000-0000-0000-0000-000000000000'
    default: return p.name === 'site_id' ? 'ABC123' : `<${p.name}>`
  }
}
function paramsBy(src: string) { return (docsApi.value?.parameters || []).filter((p: any) => p.source === src) }

const docsUrl = computed(() => {
  const a = docsApi.value; if (!a) return ''
  let path = a.path
  for (const p of paramsBy('path')) path = path.replace(`{${p.name}}`, sampleVal(p))
  const q = paramsBy('query').map((p: any) => `${p.name}=${sampleVal(p)}`).join('&')
  return joinGatewayPath(gatewayBase, path) + (q ? `?${q}` : '')
})
const tokenCurl = computed(() =>
  `curl -X POST "${authBase}/oauth/token" \\\n  -H "Content-Type: application/json" \\\n` +
  `  -d '{"client_id":"YOUR_CLIENT_ID","client_secret":"YOUR_SECRET","grant_type":"client_credentials"}'`)
const requestCurl = computed(() => {
  const a = docsApi.value; if (!a) return ''
  let c = `curl -X ${a.method} "${docsUrl.value}" \\\n  -H "Authorization: Bearer $TOKEN"`
  if (a.method === 'POST') {
    const body: Record<string, any> = {}
    for (const p of paramsBy('body')) body[p.name] = sampleVal(p)
    c += ` \\\n  -H "Content-Type: application/json" \\\n  -d '${JSON.stringify(body)}'`
  }
  return c
})
const responseExample = `{
  "success": true,
  "request_id": "req-xxxxxxxx",
  "data": { "...": "rows or object" },
  "meta": { "cached": false, "duration_ms": 53 }
}`

function copy(v: string) { navigator.clipboard?.writeText(v); toast.info('Copied to clipboard') }

function oaType(t: string) { return t === 'int' ? 'integer' : t === 'number' ? 'number' : t === 'bool' ? 'boolean' : 'string' }
async function exportOpenAPI() {
  const published = rows.value.filter((r) => r.status === 'PUBLISHED')
  if (!published.length) { toast.error('Nothing to export', 'No published APIs'); return }
  const full = await Promise.all(published.map((r) => api.get(`/api/apis/${r.id}`)))
  const paths: Record<string, any> = {}
  for (const a of full) {
    const op: any = {
      summary: a.name, description: a.description || undefined,
      security: [{ OAuth2: a.required_scope ? [a.required_scope] : [] }],
      parameters: (a.parameters || []).filter((p: any) => p.source !== 'body').map((p: any) => ({
        name: p.name, in: p.source === 'path' ? 'path' : 'query', required: !!p.required,
        schema: { type: oaType(p.param_type) },
      })),
      responses: { '200': { description: 'Success', content: { 'application/json': { schema: { type: 'object' } } } } },
    }
    const body = (a.parameters || []).filter((p: any) => p.source === 'body')
    if (a.method === 'POST' && body.length) {
      op.requestBody = { content: { 'application/json': { schema: {
        type: 'object',
        properties: Object.fromEntries(body.map((p: any) => [p.name, { type: oaType(p.param_type) }])),
        required: body.filter((p: any) => p.required).map((p: any) => p.name),
      } } } }
    }
    paths[a.path] = paths[a.path] || {}
    paths[a.path][a.method.toLowerCase()] = op
  }
  const spec = {
    openapi: '3.0.3',
    info: { title: 'DDAG Dynamic APIs', version: '1.0.0', description: 'Auto-generated from published DDAG API definitions.' },
    servers: [{ url: gatewayBase }],
    components: {
      securitySchemes: {
        OAuth2: { type: 'oauth2', flows: { clientCredentials: { tokenUrl: `${authBase}/oauth/token`, scopes: {} } } },
        bearerAuth: { type: 'http', scheme: 'bearer', bearerFormat: 'JWT' },
      },
    },
    security: [{ bearerAuth: [] }],
    paths,
  }
  const blob = new Blob([JSON.stringify(spec, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url; link.download = 'ddag-openapi.json'; link.click()
  URL.revokeObjectURL(url)
  toast.success('OpenAPI exported', `${published.length} published APIs`)
}
</script>

<template>
  <PageHeader eyebrow="GATEWAY BUILDER" title="API management" icon="apis"
    description="Build dynamic, read-only APIs from parameter-bound SQL. Draft, review, publish, and promote gateway endpoints without redeploying services.">
    <template #actions>
      <button class="btn ghost" @click="exportPromotionBundle">
        <Icon name="download" /> Promotion Bundle
      </button>
      <button class="btn ghost" title="Download OpenAPI 3.0 spec for all published APIs" @click="exportOpenAPI">
        <Icon name="download" /> OpenAPI
      </button>
      <button class="btn primary" @click="openCreate"><Icon name="plus" /> Create API</button>
    </template>
  </PageHeader>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions remote v-bind="table" empty="No APIs defined yet." @query="queryTable">
    <template #col-name="{ row }"><div><b>{{ row.name }}</b><div class="faint">{{ row.description || 'No description provided' }}</div></div></template>
    <template #col-route="{ row }"><span class="mono"><span class="badge gray">{{ row.method }}</span> {{ row.path }}</span></template>
    <template #col-required_scope="{ value }"><span class="mono faint">{{ value || '—' }}</span></template>
    <template #col-connection_name="{ value }"><span class="badge blue">{{ value || 'Unbound' }}</span></template>
    <template #col-status="{ value }"><StatusBadge :status="value" /></template>
    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon" title="API docs" @click="openDocs(row)"><Icon name="docs" /></button>
        <button class="btn icon go" title="Open live API playground" @click="openPlayground(row)"><Icon name="play" /></button>
        <button v-if="row.status === 'DRAFT'" class="btn icon" title="Send for review" @click="sendForReview(row)"><Icon name="upload" /></button>
        <button v-if="row.status === 'REVIEW'" class="btn icon" title="Approve" @click="approve(row)"><Icon name="check" /></button>
        <button v-if="row.status !== 'PUBLISHED'" class="btn icon" title="Diff vs published" @click="showDiff(row)"><Icon name="compare" /></button>
        <button v-if="row.status === 'APPROVED'" class="btn icon go" title="Publish" @click="publish(row)"><Icon name="publish" /></button>
        <button v-else-if="row.status === 'PUBLISHED'" class="btn icon" title="Disable" @click="disable(row)"><Icon name="disable" /></button>
        <button class="btn icon" title="Cache rule" @click="openCache(row)"><Icon name="cache" /></button>
        <button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button>
        <button class="btn icon danger" title="Delete" @click="remove(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>

  <!-- Builder -->
  <UiModal :open="modalOpen" :title="editing ? 'Edit API' : 'Create API'" wide @close="modalOpen = false">
    <div class="form-section">
      <h4 class="form-section-title">API identity</h4>
      <div class="row">
        <div class="field" style="flex:2"><label>Name</label><input v-model="form.name" placeholder="Get BRIM Site" /></div>
        <div class="field"><label>Namespace</label><input v-model="form.namespace" placeholder="brim" /></div>
        <div class="field"><label>Method</label><select v-model="form.method"><option value="GET">GET — simple read</option><option value="QUERY">QUERY — RFC 10008 safe read with JSON body</option><option value="POST">POST — compatible action/search</option><option value="PUT">PUT — replace (write)</option><option value="PATCH">PATCH — partial update (write)</option><option value="DELETE">DELETE — remove (write)</option></select></div>
      </div>
      <div class="field"><label>Path</label><input v-model="form.path" class="mono" placeholder="/api/v1/brim/sites/{site_id}" /></div>
      <div class="field"><label>Description</label><input v-model="form.description" /></div>
    </div>

    <div class="form-section">
      <h4 class="form-section-title">Connection & access</h4>
      <div class="row">
        <div class="field"><label>Database Connection</label>
          <select v-model="form.database_connection_id">
            <option value="" disabled>Select…</option>
            <option v-for="c in connections" :key="c.id" :value="c.id">{{ c.name }} ({{ c.database_type }})</option>
          </select>
        </div>
        <div class="field"><label>Required Scope</label>
          <select v-model="form.required_scope">
            <option value="">— none —</option>
            <option v-for="s in scopes" :key="s.id" :value="s.scope_code">{{ s.scope_code }}</option>
          </select>
        </div>
        <div class="field"><label>Default Limit</label><input v-model.number="form.default_limit" type="number" /></div>
        <div class="field"><label>Max Limit</label><input v-model.number="form.max_limit" type="number" /></div>
      </div>
    </div>

    <div class="form-section">
      <h4 class="form-section-title">Query design</h4>
      <div class="field" v-if="['POST', 'PUT', 'PATCH', 'DELETE'].includes(form.method)"><label class="checkbox"><input :checked="true" type="checkbox" disabled /> Write operation <span class="faint">(auto-detected from HTTP method —auto-enabled)</span></label></div>
      <p v-if="['GET', 'QUERY'].includes(form.method)" class="faint" style="margin:0 0 12px">{{ form.method === 'QUERY' ? 'QUERY is read-only and accepts typed body parameters as JSON.' : 'GET is read-only and cache-friendly; use QUERY for complex JSON filters.' }}</p>
      <div class="field">
        <label>Query Template <span class="faint">(use :param binding — never concatenate user input)</span></label>
        <textarea v-model="form.query_template" rows="5" placeholder="SELECT * FROM table WHERE id = :id"></textarea>
      </div>
      <div class="field">
        <label>Query Builder JSON</label>
        <textarea v-model="form.query_builder_json" rows="6" placeholder='{"base_table":"karyawan","select":["karyawan.id","karyawan.nama"],"filters":[{"name":"status","column":"karyawan.status","operators":["eq","in"]}],"sortable_columns":[{"name":"nama","column":"karyawan.nama"}]}'></textarea>
      </div>
    </div>

    <div class="form-section">
      <label style="display:flex;justify-content:space-between;align-items:center">
        Parameters <button class="btn sm ghost" type="button" @click="addParam">+ Add</button>
      </label>
      <div v-for="(p, i) in form.parameters" :key="i" class="param-row">
        <div class="field"><input v-model="p.name" placeholder="name" /></div>
        <div class="field"><select v-model="p.source"><option>path</option><option>query</option><option>body</option></select></div>
        <div class="field"><select v-model="p.param_type"><option>string</option><option>int</option><option>number</option><option>bool</option><option>uuid</option><option>date</option></select></div>
        <div class="field" style="flex:0 0 auto"><label class="checkbox" style="margin:0; min-height:auto; padding:6px 10px"><input v-model="p.required" type="checkbox" />req</label></div>
        <div class="field"><input v-model.number="p.max_length" type="number" placeholder="maxlen" /></div>
        <button class="btn sm danger btn-remove" type="button" @click="removeParam(i)">×</button>
      </div>
    </div>

    <div class="card stacked-card">
      <div class="card-head">
        <h3>SQL Preview</h3>
        <div class="actions-cell">
          <button class="btn sm ghost" @click="previewSQL" :disabled="previewing"><span v-if="previewing" class="spin" /> Preview</button>
          <button class="btn sm" @click="explainSQL" :disabled="explaining"><span v-if="explaining" class="spin" /> Explain</button>
        </div>
      </div>
      <div class="card-body">
        <div class="field"><label>Sample Query</label><input v-model="previewQueryString" class="mono" placeholder="status=eq:active&sort=-created_at" /></div>
        <p v-if="previewError" class="error-text">{{ previewError }}</p>
        <pre v-if="previewResult" class="code-block">{{ JSON.stringify(previewResult, null, 2) }}</pre>
        <pre v-if="explainResult" class="code-block">{{ JSON.stringify(explainResult, null, 2) }}</pre>
      </div>
    </div>

    <div class="card stacked-card">
      <div class="card-head"><h3>Test Query</h3><button class="btn sm" @click="runTest" :disabled="testing"><span v-if="testing" class="spin" /> Run</button></div>
      <div class="card-body">
        <div v-if="form.parameters.length" class="param-grid">
          <div v-for="(p, i) in form.parameters" :key="i" class="field">
            <label>{{ p.name || 'param' }}</label><input v-model="sampleParams[p.name]" :placeholder="p.param_type" />
          </div>
        </div>
        <p v-if="testError" class="error-text">{{ testError }}</p>
        <pre v-if="testRows" class="code-block">{{ JSON.stringify(testRows, null, 2) }}</pre>
      </div>
    </div>

    <template #footer>
      <button class="btn primary" @click="save" :disabled="saving"><span v-if="saving" class="spin" /> Save Draft</button>
    </template>
  </UiModal>

  <!-- Cache rule -->
  <UiModal :open="cacheModal" :title="`Cache — ${cacheApi?.name}`" @close="cacheModal = false">
    <div class="field"><label class="checkbox"><input v-model="cacheForm.enabled" type="checkbox" /> Enable caching</label></div>
    <div class="row">
      <div class="field"><label>TTL (seconds)</label><input v-model.number="cacheForm.ttl_seconds" type="number" /></div>
      <div class="field"><label class="checkbox" style="margin-top:24px"><input v-model="cacheForm.vary_by_client" type="checkbox" /> Vary by client</label></div>
    </div>
    <div class="field"><label>Cache Key Strategy</label><input v-model="cacheForm.cache_key_strategy" class="mono" /></div>
    <template #footer><button class="btn primary" @click="saveCache">Save Cache Rule</button></template>
  </UiModal>

  <!-- API consumption docs -->
  <UiModal :open="docsModal" :title="`API Docs — ${docsApi?.name}`" wide @close="docsModal = false">
    <div v-if="docsApi">
      <div class="kv" style="margin-bottom:16px">
        <div class="k">Endpoint</div>
        <div><span class="badge gray">{{ docsApi.method }}</span> <span class="mono">{{ docsApi.path }}</span></div>
        <div class="k">Base URL</div><div class="mono">{{ gatewayBase }}</div>
        <div class="k">Required scope</div>
        <div><span v-if="docsApi.required_scope" class="mono">{{ docsApi.required_scope }}</span><span v-else class="faint">none</span></div>
        <div class="k">Status</div><div><StatusBadge :status="docsApi.status" /></div>
      </div>

      <h4 style="margin:0 0 8px">Parameters</h4>
      <div v-if="docsApi.parameters?.length" class="table-wrap" style="margin-bottom:18px">
        <table class="tbl">
          <thead><tr><th>Name</th><th>In</th><th>Type</th><th>Required</th><th>Max length</th></tr></thead>
          <tbody>
            <tr v-for="p in docsApi.parameters" :key="p.name">
              <td class="mono">{{ p.name }}</td><td>{{ p.source }}</td><td>{{ p.param_type }}</td>
              <td>{{ p.required ? 'yes' : 'no' }}</td><td>{{ p.max_length || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="faint" style="margin-bottom:18px">No parameters.</p>

      <h4 style="margin:0 0 6px">1. Get an access token</h4>
      <div class="code-block" style="position:relative">
        <button class="btn icon" style="position:absolute;top:8px;right:8px" title="Copy" @click="copy(tokenCurl)"><Icon name="copy" /></button>
        <span>{{ tokenCurl }}</span>
      </div>

      <h4 style="margin:16px 0 6px">2. Call the endpoint</h4>
      <div class="code-block" style="position:relative">
        <button class="btn icon" style="position:absolute;top:8px;right:8px" title="Copy" @click="copy(requestCurl)"><Icon name="copy" /></button>
        <span>{{ requestCurl }}</span>
      </div>

      <h4 style="margin:16px 0 6px">Example response</h4>
      <pre class="code-block">{{ responseExample }}</pre>
    </div>
    <template #footer>
      <button class="btn ghost" @click="exportOpenAPI"><Icon name="download" /> Download OpenAPI</button>
      <button class="btn primary" @click="docsModal = false">Close</button>
    </template>
  </UiModal>

  <UiModal :open="diffModal" title="Draft vs Published Diff" wide @close="diffModal = false">
    <div class="field">
      <label>Approval Comment</label>
      <input v-model="approvalComment" placeholder="Reason / reviewer comment" />
    </div>
    <pre class="code-block">{{ JSON.stringify(diffResult, null, 2) }}</pre>
    <template #footer>
      <button class="btn primary" @click="diffModal = false">Close</button>
    </template>
  </UiModal>

  <UiModal :open="promotionModal" title="Promotion Bundle Dry-Run" wide @close="promotionModal = false">
    <div class="field">
      <label>Bundle JSON</label>
      <textarea v-model="promotionPayload" rows="12" class="mono" />
    </div>
    <pre v-if="promotionResult" class="code-block">{{ JSON.stringify(promotionResult, null, 2) }}</pre>
    <template #footer>
      <button class="btn ghost" @click="dryRunPromotion">Dry-Run Import</button>
      <button class="btn primary" @click="promotionModal = false">Close</button>
    </template>
  </UiModal>

  <!-- Live API Playground Modal -->
  <UiModal :open="playgroundModal" :title="`Interactive API Playground — ${playgroundApi?.name}`" wide @close="playgroundModal = false">
    <div v-if="playgroundApi">
      <div style="display: flex; gap: 16px; align-items: flex-end; margin-bottom: 20px;">
        <div class="field" style="width: 120px; margin: 0;">
          <label>HTTP Method</label>
          <select v-model="playgroundMethod">
            <option value="GET">GET</option>
            <option value="QUERY">QUERY — RFC 10008</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="PATCH">PATCH</option>
            <option value="DELETE">DELETE</option>
          </select>
        </div>
        <div class="field" style="flex: 1; margin: 0;">
          <label>Live Target URL</label>
          <input :value="playgroundUrl" readonly class="mono" style="background-color: #f5f5f5;" />
        </div>
        <button class="btn primary" @click="sendPlaygroundRequest" :disabled="playgroundSending" style="height: 42px;">
          <span v-if="playgroundSending" class="spin" /> Send Request
        </button>
      </div>

      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-top: 16px;">
        <!-- Left Column: Request Builder -->
        <div>
          <h4 style="margin: 0 0 8px;">Headers</h4>
          <div v-for="(h, idx) in playgroundHeaders" :key="idx" style="display: flex; gap: 8px; margin-bottom: 8px;">
            <input v-model="h.key" placeholder="Header Name" style="flex: 1;" />
            <input v-model="h.value" placeholder="Value" style="flex: 2;" />
            <button class="btn sm danger" type="button" @click="removePlaygroundRow(playgroundHeaders, idx)">×</button>
          </div>
          <button class="btn sm ghost" style="margin-bottom: 16px;" @click="addPlaygroundHeader">+ Add Header</button>

          <h4 style="margin: 0 0 8px;">Query Parameters</h4>
          <div v-for="(p, idx) in playgroundParams" :key="idx" style="display: flex; gap: 8px; margin-bottom: 8px;">
            <input v-model="p.key" placeholder="Param Name" style="flex: 1;" />
            <input v-model="p.value" placeholder="Value" style="flex: 2;" />
            <button class="btn sm danger" type="button" @click="removePlaygroundRow(playgroundParams, idx)">×</button>
          </div>
          <button class="btn sm ghost" style="margin-bottom: 16px;" @click="addPlaygroundParam">+ Add Parameter</button>

          <div v-if="!['GET', 'HEAD'].includes(playgroundMethod.toUpperCase())">
            <h4 style="margin: 0 0 8px;">JSON Request Body</h4>
            <textarea v-model="playgroundBody" rows="8" class="mono" placeholder="{}" style="width: 100%; font-family: monospace;"></textarea>
          </div>
        </div>

        <!-- Right Column: Response Viewer -->
        <div>
          <h4 style="margin: 0 0 8px;">Response</h4>
          <div v-if="playgroundError" class="error-text" style="padding: 12px; border: 1px solid #ffccd5; background: #fff5f5; border-radius: 4px; color: #d93838; font-size: 14px;">
            {{ playgroundError }}
          </div>
          <div v-else-if="playgroundResponse">
            <div style="display: flex; gap: 12px; margin-bottom: 12px; font-size: 14px;">
              <div>Status: <b :style="{ color: playgroundResponse.ok ? '#2e7d32' : '#d32f2f' }">{{ playgroundResponse.status }} {{ playgroundResponse.statusText }}</b></div>
              <div>Latency: <b>{{ playgroundResponse.latency }} ms</b></div>
            </div>

            <h5 style="margin: 12px 0 6px;">Headers</h5>
            <pre class="code-block" style="max-height: 150px; overflow-y: auto; font-size: 12px; padding: 8px;">{{ JSON.stringify(playgroundResponse.headers, null, 2) }}</pre>

            <h5 style="margin: 12px 0 6px;">Body</h5>
            <pre class="code-block" style="max-height: 350px; overflow-y: auto; font-size: 12px; padding: 8px;">{{ JSON.stringify(playgroundResponse.body, null, 2) }}</pre>
          </div>
          <div v-else style="display: flex; align-items: center; justify-content: center; height: 250px; border: 2px dashed #ddd; border-radius: 4px; color: #888;">
            Click "Send Request" to see output
          </div>
        </div>
      </div>
    </div>
    <template #footer>
      <button class="btn primary" @click="playgroundModal = false">Close</button>
    </template>
  </UiModal>
</template>
