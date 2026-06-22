<script setup lang="ts">
definePageMeta({ title: 'Token & Scopes' })
const api = useApi()
const toast = useToast()

const rows = ref<any[]>([])
const loading = ref(true)

const columns = [
  { key: 'scope_code', label: 'Scope', mono: true },
  { key: 'description', label: 'Description' },
]

const modalOpen = ref(false)
const saving = ref(false)

const blank = () => ({ scope_code: '', description: '' })
const form = reactive<any>(blank())

async function load() {
  loading.value = true
  try {
    rows.value = (await api.list('/api/scopes')).items
  } finally { loading.value = false }
}
onMounted(load)

function openCreate() { Object.assign(form, blank()); modalOpen.value = true }

async function save() {
  saving.value = true
  try {
    await api.post('/api/scopes', form)
    toast.success('Scope created')
    modalOpen.value = false
    await load()
  } catch (e: any) { toast.error('Save failed', e.message) } finally { saving.value = false }
}

async function remove(row: any) {
  if (!confirm(`Delete scope "${row.scope_code}"? Clients granted this scope will lose the associated access.`)) return
  try {
    await api.del(`/api/scopes/${row.id}`)
    toast.success('Deleted')
    await load()
  } catch (e: any) { toast.error('Delete failed', e.message) }
}
</script>

<template>
  <p class="page-desc">OAuth2 scopes gate API access per client. Each scope is a permission a client can be granted, then checked when issuing and validating tokens.</p>
  <div class="toolbar"><div class="spacer" /><button class="btn primary" @click="openCreate">+ New Scope</button></div>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions empty="No scopes defined yet.">
    <template #col-description="{ value }">
      <span v-if="value">{{ value }}</span>
      <span v-else class="faint">—</span>
    </template>
    <template #actions="{ row }">
      <span class="actions-cell">
        <button class="btn icon danger" title="Delete" @click="remove(row)"><Icon name="trash" /></button>
      </span>
    </template>
  </UiTable>

  <UiModal :open="modalOpen" title="New Scope" @close="modalOpen = false">
    <div class="field"><label>Scope Code</label><input v-model="form.scope_code" class="mono" placeholder="orders:read" /></div>
    <div class="field"><label>Description</label><textarea v-model="form.description" placeholder="Read access to orders." /></div>

    <template #footer>
      <button class="btn primary" :disabled="saving" @click="save"><span v-if="saving" class="spin" /> Create</button>
    </template>
  </UiModal>
</template>
