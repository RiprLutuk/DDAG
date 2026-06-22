<script setup lang="ts">
definePageMeta({ title: 'API Management' })
const api = useApi()
const toast = useToast()
const cfg = useRuntimeConfig()
const gatewayBase = cfg.public.gatewayBase as string
const authBase = cfg.public.authBase as string

const rows = ref<any[]>([])
const loading = ref(true)
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
const testing = ref(false)
const testRows = ref<any[] | null>(null)
const testError = ref('')
const sampleParams = reactive<Record<string, any>>({})

const blank = () => ({
  name: '', namespace: '', path: '', method: 'GET', description: '', database_connection_id: '',
  query_template: '', required_scope: '', default_limit: 100, max_limit: 1000,
  parameters: [] as any[],
})
const form = reactive<any>(blank())

const cacheModal = ref(false)
const cacheApi = ref<any>(null)
const cacheForm = reactive<any>({ enabled: false, ttl_seconds: 300, vary_by_client: true, cache_key_strategy: 'client_id:path:query_params' })

async function load() {
  loading.value = true
  try {
    rows.value = (await api.list('/api/apis')).items
    connections.value = (await api.list('/api/connections')).items
    scopes.value = (await api.list('/api/scopes')).items
  } finally { loading.value = false }
}
onMounted(load)

function openCreate() { Object.assign(form, blank()); editing.value = null; testRows.value = null; testError.value = ''; modalOpen.value = true }
async function openEdit(row: any) {
  const a = await api.get(`/api/apis/${row.id}`)
  Object.assign(form, { ...blank(), ...a, database_connection_id: a.database_connection_id || '', parameters: a.parameters || [] })
  editing.value = row; testRows.value = null; testError.value = ''; modalOpen.value = true
}
function addParam() { form.parameters.push({ name: '', source: 'path', param_type: 'string', required: true, max_length: null, default_value: null, validation_rule: null }) }
function removeParam(i: number) { form.parameters.splice(i, 1) }

async function runTest() {
  testing.value = true; testRows.value = null; testError.value = ''
  try {
    const res = await api.post('/api/apis/test', {
      connection_id: form.database_connection_id, query_template: form.query_template,
      parameters: { ...sampleParams }, limit: form.default_limit,
    })
    if (res.success) testRows.value = res.rows || []
    else testError.value = res.error?.message || 'Query failed'
  } catch (e: any) { testError.value = e.message } finally { testing.value = false }
}

async function save() {
  saving.value = true
  try {
    if (editing.value) await api.put(`/api/apis/${editing.value.id}`, form)
    else await api.post('/api/apis', form)
    toast.success('API saved as draft')
    modalOpen.value = false; await load()
  } catch (e: any) { toast.error('Save failed', e.message) } finally { saving.value = false }
}

async function publish(row: any) {
  try { await api.post(`/api/apis/${row.id}/publish`); toast.success('Published', row.name); await load() }
  catch (e: any) { toast.error('Publish blocked', e.message) }
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
  return gatewayBase + path + (q ? `?${q}` : '')
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
  <p class="page-desc">Build dynamic, read-only APIs from parameter-bound SQL. Publish to make them live on the gateway — no deployment needed.</p>
  <div class="toolbar">
    <div class="spacer" />
    <button class="btn ghost" title="Download OpenAPI 3.0 spec for all published APIs" @click="exportOpenAPI">
      <Icon name="download" /> OpenAPI
    </button>
    <button class="btn primary" @click="openCreate">+ Create API</button>
  </div>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions empty="No APIs defined yet.">
    <template #col-route="{ row }"><span class="mono"><span class="badge gray">{{ row.method }}</span> {{ row.path }}</span></template>
    <template #col-required_scope="{ value }"><span class="mono faint">{{ value || '—' }}</span></template>
    <template #col-status="{ value }"><StatusBadge :status="value" /></template>
    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon" title="API docs" @click="openDocs(row)"><Icon name="docs" /></button>
        <button v-if="row.status !== 'PUBLISHED'" class="btn icon go" title="Publish" @click="publish(row)"><Icon name="publish" /></button>
        <button v-else class="btn icon" title="Disable" @click="disable(row)"><Icon name="disable" /></button>
        <button class="btn icon" title="Cache rule" @click="openCache(row)"><Icon name="cache" /></button>
        <button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button>
        <button class="btn icon danger" title="Delete" @click="remove(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>

  <!-- Builder -->
  <UiModal :open="modalOpen" :title="editing ? 'Edit API' : 'Create API'" wide @close="modalOpen = false">
    <div class="row">
      <div class="field" style="flex:2"><label>Name</label><input v-model="form.name" placeholder="Get BRIM Site" /></div>
      <div class="field"><label>Namespace</label><input v-model="form.namespace" placeholder="brim" /></div>
      <div class="field"><label>Method</label><select v-model="form.method"><option>GET</option><option>POST</option></select></div>
    </div>
    <div class="field"><label>Path</label><input v-model="form.path" class="mono" placeholder="/api/v1/brim/sites/{site_id}" /></div>
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
    <div class="field"><label>Description</label><input v-model="form.description" /></div>
    <div class="field">
      <label>Query Template <span class="faint">(use :param binding — never concatenate user input)</span></label>
      <textarea v-model="form.query_template" rows="5" placeholder="SELECT * FROM table WHERE id = :id"></textarea>
    </div>

    <div class="field">
      <label style="display:flex;justify-content:space-between;align-items:center">
        Parameters <button class="btn sm ghost" type="button" @click="addParam">+ Add</button>
      </label>
      <div v-for="(p, i) in form.parameters" :key="i" class="row" style="margin-bottom:8px;align-items:flex-end">
        <div class="field" style="margin:0"><input v-model="p.name" placeholder="name" /></div>
        <div class="field" style="margin:0"><select v-model="p.source"><option>path</option><option>query</option><option>body</option></select></div>
        <div class="field" style="margin:0"><select v-model="p.param_type"><option>string</option><option>int</option><option>number</option><option>bool</option><option>uuid</option><option>date</option></select></div>
        <div class="field" style="margin:0;flex:0 0 auto"><label class="checkbox"><input v-model="p.required" type="checkbox" />req</label></div>
        <div class="field" style="margin:0"><input v-model.number="p.max_length" type="number" placeholder="maxlen" /></div>
        <button class="btn sm danger" type="button" @click="removeParam(i)">×</button>
      </div>
    </div>

    <div class="card" style="margin-top:6px">
      <div class="card-head"><h3>Test Query</h3><button class="btn sm" @click="runTest" :disabled="testing"><span v-if="testing" class="spin" /> Run</button></div>
      <div class="card-body">
        <div v-if="form.parameters.length" class="row" style="flex-wrap:wrap">
          <div v-for="(p, i) in form.parameters" :key="i" class="field" style="min-width:140px">
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
</template>
