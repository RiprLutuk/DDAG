<script setup lang="ts">
const api = useApi()
const toast = useToast()

const rows = ref<any[]>([])
const loading = ref(true)
const catalog = ref<any[]>([])
const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'name', sortDir: 'asc' })

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'description', label: 'Description' },
  { key: 'permissions', label: 'Permissions' },
  { key: 'type', label: 'Type' },
]

const createOpen = ref(false)
const creating = ref(false)
const blankCreate = () => ({ name: '', description: '' })
const createForm = reactive<any>(blankCreate())

const editOpen = ref(false)
const saving = ref(false)
const editing = ref<any>(null)
const editDescription = ref('')
const selected = ref<string[]>([])

async function load() {
  loading.value = true
  try {
    const q = new URLSearchParams({ page: String(table.page), limit: String(table.limit), search: table.search, sort_by: table.sortBy, sort_dir: table.sortDir })
    const result = await api.list(`/api/roles?${q}`)
    rows.value = result.items; table.total = result.pagination.total
    catalog.value = (await api.list('/api/permissions')).items
  } finally { loading.value = false }
}
onMounted(load)
function queryTable(q: any) { Object.assign(table, q); load() }

function openCreate() { Object.assign(createForm, blankCreate()); createOpen.value = true }

async function createRole() {
  creating.value = true
  try {
    await api.post('/api/roles', { name: createForm.name, description: createForm.description })
    toast.success('Role created', createForm.name)
    createOpen.value = false
    await load()
  } catch (e: any) { toast.error('Create failed', e.message) } finally { creating.value = false }
}

async function openEdit(row: any) {
  const r = await api.get(`/api/roles/${row.id}`)
  editing.value = r
  editDescription.value = r.description || ''
  selected.value = [...(r.permissions || [])]
  editOpen.value = true
}

function isChecked(code: string) { return selected.value.includes(code) }
function toggle(code: string) {
  const i = selected.value.indexOf(code)
  i >= 0 ? selected.value.splice(i, 1) : selected.value.push(code)
}

async function savePermissions() {
  saving.value = true
  try {
    if (!editing.value.is_system) {
      await api.put(`/api/roles/${editing.value.id}`, { description: editDescription.value })
    }
    await api.post(`/api/roles/${editing.value.id}/permissions`, { permissions: selected.value })
    toast.success('Permissions updated', editing.value.name)
    editOpen.value = false
    await load()
  } catch (e: any) { toast.error('Save failed', e.message) } finally { saving.value = false }
}

async function remove(row: any) {
  if (row.is_system) return
  if (!confirm(`Delete role "${row.name}"? This cannot be undone.`)) return
  try {
    await api.del(`/api/roles/${row.id}`)
    toast.success('Deleted', row.name)
    await load()
  } catch (e: any) { toast.error('Delete failed', e.message) }
}
</script>

<template>
  <p class="page-desc">Define roles and grant them permissions from the catalog. System roles are built-in and cannot be deleted.</p>
  <div class="toolbar compact-mobile"><div class="spacer" /><button class="btn primary" @click="openCreate">+ New Role</button></div>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions remote v-bind="table" empty="No roles defined yet." @query="queryTable">
    <template #col-name="{ value }"><span class="mono">{{ value }}</span></template>
    <template #col-description="{ value }"><span :class="{ faint: !value }">{{ value || '—' }}</span></template>
    <template #col-permissions="{ row }"><span class="badge blue">{{ (row.permissions || []).length }}</span></template>
    <template #col-type="{ row }">
      <span class="badge" :class="row.is_system ? 'gray' : 'green'">{{ row.is_system ? 'system' : 'custom' }}</span>
    </template>
    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon" title="Edit permissions" @click="openEdit(row)"><Icon name="edit" /></button>
        <button v-if="!row.is_system" class="btn icon danger" title="Delete" @click="remove(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>

  <!-- Create role -->
  <UiModal :open="createOpen" title="New Role" @close="createOpen = false">
    <div class="field"><label>Name</label><input v-model="createForm.name" class="mono" placeholder="content_editor" /></div>
    <div class="field"><label>Description</label><textarea v-model="createForm.description" rows="3" placeholder="What this role is for"></textarea></div>
    <template #footer>
      <button class="btn primary" @click="createRole" :disabled="creating"><span v-if="creating" class="spin" /> Create</button>
    </template>
  </UiModal>

  <!-- Edit permissions -->
  <UiModal :open="editOpen" :title="`Permissions — ${editing?.name}`" wide @close="editOpen = false">
    <div class="field">
      <label>Description</label>
      <textarea v-model="editDescription" rows="2" :disabled="editing?.is_system" placeholder="Role description"></textarea>
      <span v-if="editing?.is_system" class="faint">System roles cannot be renamed or re-described.</span>
    </div>

    <div class="field">
      <label style="display:flex;justify-content:space-between;align-items:center">
        Permission Catalog <span class="faint">{{ selected.length }} of {{ catalog.length }} granted</span>
      </label>
      <div v-if="!catalog.length" class="empty">No permissions defined.</div>
      <div class="permission-grid">
        <label v-for="p in catalog" :key="p.id" class="checkbox permission-item">
          <input type="checkbox" :checked="isChecked(p.code)" @change="toggle(p.code)" />
          <span>
            <span class="mono">{{ p.code }}</span>
            <span v-if="p.description" class="faint" style="display:block">{{ p.description }}</span>
          </span>
        </label>
      </div>
    </div>

    <template #footer>
      <button class="btn primary" @click="savePermissions" :disabled="saving"><span v-if="saving" class="spin" /> Save</button>
    </template>
  </UiModal>
</template>