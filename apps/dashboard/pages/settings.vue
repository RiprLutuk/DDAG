<script setup lang="ts">
definePageMeta({ title: 'Settings' })
const api = useApi()
const toast = useToast()

const rows = ref<any[]>([])
const loading = ref(true)
const columns = [
  { key: 'key', label: 'Key', mono: true },
  { key: 'value', label: 'Value', mono: true },
  { key: 'updated_at', label: 'Updated' },
]
const fmt = (v: string) => (v ? new Date(v).toLocaleString() : '—')
const preview = (v: any) => { const s = JSON.stringify(v); return s && s.length > 80 ? s.slice(0, 80) + '…' : s }

const modalOpen = ref(false)
const editing = ref(false)
const saving = ref(false)
const form = reactive({ key: '', value: '{\n  \n}' })

async function load() {
  loading.value = true
  try { rows.value = (await api.list('/api/settings')).items } finally { loading.value = false }
}
onMounted(load)

function openCreate() { form.key = ''; form.value = '{\n  \n}'; editing.value = false; modalOpen.value = true }
function openEdit(row: any) { form.key = row.key; form.value = JSON.stringify(row.value, null, 2); editing.value = true; modalOpen.value = true }

async function save() {
  if (!form.key) { toast.error('Key is required'); return }
  let parsed: any
  try { parsed = JSON.parse(form.value) } catch { toast.error('Invalid JSON', 'Value must be valid JSON'); return }
  saving.value = true
  try {
    await api.put(`/api/settings/${encodeURIComponent(form.key)}`, parsed)
    toast.success('Setting saved', form.key); modalOpen.value = false; await load()
  } catch (e: any) { toast.error('Save failed', e.message) } finally { saving.value = false }
}
</script>

<template>
  <p class="page-desc">Platform key/value settings. Values are stored as JSON and tune platform behavior.</p>
  <div class="toolbar"><div class="spacer" /><button class="btn primary" @click="openCreate">+ New Setting</button></div>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions empty="No settings defined yet.">
    <template #col-value="{ value }"><span class="faint">{{ preview(value) }}</span></template>
    <template #col-updated_at="{ value }"><span class="faint">{{ fmt(value) }}</span></template>
    <template #actions="{ row }"><span class="actions-cell"><button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button></span></template>
  </UiTable>

  <UiModal :open="modalOpen" :title="editing ? 'Edit Setting' : 'New Setting'" @close="modalOpen = false">
    <div class="field"><label>Key</label><input v-model="form.key" class="mono" :disabled="editing" placeholder="cache.default_ttl" /></div>
    <div class="field"><label>Value (JSON)</label><textarea v-model="form.value" rows="8" class="mono"></textarea></div>
    <template #footer><button class="btn primary" @click="save" :disabled="saving"><span v-if="saving" class="spin" /> Save</button></template>
  </UiModal>
</template>
