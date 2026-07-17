<script setup lang="ts">
const api = useApi()
const toast = useToast()

const rows = ref<any[]>([])
const loading = ref(true)
const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'created_at', sortDir: 'desc' })
const rolesCatalog = ref<string[]>([])

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'email', label: 'Email' },
  { key: 'username', label: 'Username', mono: true },
  { key: 'roles', label: 'Roles' },
  { key: 'status', label: 'Status' },
  { key: 'last_login_at', label: 'Last Login' },
]

const modalOpen = ref(false)
const editing = ref<any>(null)
const saving = ref(false)
const pwModal = ref(false)
const pwTarget = ref<any>(null)
const newPw = ref('')

const blank = () => ({ name: '', email: '', username: '', password: '', tenant: '', status: 'active', roles: [] as string[] })
const form = reactive<any>(blank())

const fmt = (v: string) => (v ? new Date(v).toLocaleString() : '—')

async function load() {
  loading.value = true
  try {
    const q = new URLSearchParams({ page: String(table.page), limit: String(table.limit), search: table.search, sort_by: table.sortBy, sort_dir: table.sortDir })
    const result = await api.list(`/api/users?${q}`)
    rows.value = result.items; table.total = result.pagination.total
    rolesCatalog.value = (await api.list('/api/roles')).items.map((r: any) => r.name)
  } finally { loading.value = false }
}
onMounted(load)
function queryTable(q: any) { Object.assign(table, q); load() }

function openCreate() { Object.assign(form, blank()); editing.value = null; modalOpen.value = true }
async function openEdit(row: any) {
  const u = await api.get(`/api/users/${row.id}`)
  Object.assign(form, { ...blank(), ...u, password: '', tenant: u.tenant || '', roles: u.roles || [] })
  editing.value = row; modalOpen.value = true
}
function toggleRole(r: string) {
  const i = form.roles.indexOf(r); i >= 0 ? form.roles.splice(i, 1) : form.roles.push(r)
}

async function save() {
  saving.value = true
  try {
    if (editing.value) {
      await api.put(`/api/users/${editing.value.id}`, { name: form.name, email: form.email, tenant: form.tenant || null, status: form.status })
      await api.post(`/api/users/${editing.value.id}/roles`, { roles: form.roles })
      toast.success('User updated')
    } else {
      const u = await api.post('/api/users', { name: form.name, email: form.email, username: form.username, password: form.password, tenant: form.tenant || null, roles: form.roles })
      toast.success('User created', u.username)
    }
    modalOpen.value = false; await load()
  } catch (e: any) { toast.error('Save failed', e.message) } finally { saving.value = false }
}

function openPw(row: any) { pwTarget.value = row; newPw.value = ''; pwModal.value = true }
async function savePw() {
  try {
    await api.post(`/api/users/${pwTarget.value.id}/password`, { password: newPw.value })
    toast.success('Password reset', pwTarget.value.username); pwModal.value = false
  } catch (e: any) { toast.error('Failed', e.message) }
}

async function disable(row: any) {
  if (!confirm(`Disable user "${row.username}"? They will not be able to sign in.`)) return
  try { await api.post(`/api/users/${row.id}/disable`); toast.info('Disabled', row.username); await load() }
  catch (e: any) { toast.error('Failed', e.message) }
}
</script>

<template>
  <PageHeader eyebrow="ACCESS CONTROL" title="Users" icon="users"
    description="Manage dashboard operators, access status, and role assignments. Permission updates take effect immediately.">
    <template #actions>
      <button class="btn primary" @click="openCreate"><Icon name="plus" /> New User</button>
    </template>
  </PageHeader>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions remote v-bind="table" empty="No users yet." @query="queryTable">
    <template #col-name="{ row }"><div><b>{{ row.name }}</b><div class="faint">{{ row.email }}</div></div></template>
    <template #col-email="{ value }"><span class="mono faint">{{ value }}</span></template>
    <template #col-roles="{ row }">
      <span v-for="r in row.roles" :key="r" class="tag">{{ r }}</span>
      <span v-if="!(row.roles || []).length" class="faint">—</span>
    </template>
    <template #col-status="{ value }"><StatusBadge :status="value" /></template>
    <template #col-last_login_at="{ value }"><span class="faint">{{ fmt(value) }}</span></template>
    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button>
        <button class="btn icon" title="Reset password" @click="openPw(row)"><Icon name="key" /></button>
        <button class="btn icon danger" title="Disable" @click="disable(row)"><Icon name="disable" /></button>
      </span>
    </template>
  </UiTable>

  <UiModal :open="modalOpen" :title="editing ? 'Edit User' : 'New User'" @close="modalOpen = false">
    <div class="row">
      <div class="field"><label>Name</label><input v-model="form.name" /></div>
      <div class="field"><label>Email</label><input v-model="form.email" type="email" /></div>
    </div>
    <div class="row">
      <div class="field"><label>Username</label><input v-model="form.username" class="mono" :disabled="!!editing" /></div>
      <div class="field"><label>Tenant</label><input v-model="form.tenant" placeholder="optional" /></div>
    </div>
    <div v-if="!editing" class="field"><label>Password</label><input v-model="form.password" type="password" autocomplete="new-password" /></div>
    <div v-if="editing" class="field"><label>Status</label><select v-model="form.status"><option value="active">active</option><option value="inactive">inactive</option></select></div>
    <div class="field">
      <label>Roles</label>
      <div class="permission-grid">
        <label v-for="r in rolesCatalog" :key="r" class="checkbox permission-item">
          <input type="checkbox" :checked="form.roles.includes(r)" @change="toggleRole(r)" />
          <div>
            <div>{{ r }}</div>
            <div class="faint">Grant {{ r }} capabilities</div>
          </div>
        </label>
      </div>
    </div>
    <template #footer><button class="btn primary" @click="save" :disabled="saving"><span v-if="saving" class="spin" /> {{ editing ? 'Save' : 'Create' }}</button></template>
  </UiModal>

  <UiModal :open="pwModal" :title="`Reset Password — ${pwTarget?.username}`" @close="pwModal = false">
    <div class="field"><label>New Password</label><input v-model="newPw" type="password" autocomplete="new-password" /></div>
    <template #footer><button class="btn primary" @click="savePw" :disabled="newPw.length < 6">Set Password</button></template>
  </UiModal>
</template>
