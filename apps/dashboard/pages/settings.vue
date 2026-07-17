<script setup lang="ts">
const api = useApi()
const toast = useToast()
const rows = ref<any[]>([])
const loading = ref(true)
const modalOpen = ref(false)
const saving = ref(false)
const form = reactive({ key: '', value: '', type: 'json' })

const table = reactive({ page: 1, limit: 10, total: 0, search: '', sortBy: 'key', sortDir: 'asc' })
function openNotifications() {
  table.page = 1
  table.search = 'notifications'
  load()
}

const columns = [
  { key: 'key', label: 'Key', mono: true },
  { key: 'category', label: 'Category' },
  { key: 'value', label: 'Value', mono: true },
  { key: 'type', label: 'Type' },
  { key: 'restart_required', label: 'Restart' },
  { key: 'updated_at', label: 'Updated' },
]

const fmt = (v: string) => (v ? new Date(v).toLocaleString() : '—')
const preview = (v: any) => {
  const s = JSON.stringify(v)
  return s && s.length > 80 ? s.slice(0, 80) + '…' : s
}

async function load() {
  loading.value = true
  try {
    const result = await api.list(`/api/settings?${new URLSearchParams({
      page: String(table.page),
      limit: String(table.limit),
      search: table.search,
      sort_by: table.sortBy,
      sort_dir: table.sortDir
    })}`)
    rows.value = result.items || []
    table.total = result.pagination.total
  } catch (e: any) {
    toast.error('Cannot load settings', e.message)
  } finally {
    loading.value = false
  }
}

onMounted(load)
function queryTable(q: any) {
  Object.assign(table, q)
  load()
}

function openEdit(row: any) {
  form.key = row.key
  form.type = row.value_type || 'json'
  form.value = JSON.stringify(row.value, null, 2)
  modalOpen.value = true
}

async function save() {
  let parsed: any
  try {
    parsed = JSON.parse(form.value)
  } catch {
    toast.error('Invalid JSON', 'Value must match the setting type')
    return
  }
  saving.value = true
  try {
    await api.put(`/api/settings/${encodeURIComponent(form.key)}`, parsed)
    toast.success('Setting saved', form.key)
    modalOpen.value = false
    await load()
  } catch (e: any) {
    toast.error('Save failed', e.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <PageHeader eyebrow="PLATFORM" title="Settings" icon="settings"
    description="Manage system variables, engine thresholds, and worker configurations dynamically across all control planes.">
    <template #actions><button class="btn" @click="openNotifications">Notification delivery</button></template>
  </PageHeader>

  <section class="notification-card card" aria-labelledby="notification-delivery-title">
    <div class="notification-card-copy">
      <div class="eyebrow">ALERT DELIVERY</div>
      <h3 id="notification-delivery-title">Notification delivery</h3>
      <p>Configure the Alertmanager endpoint and receiver channels. Sensitive credentials such as Telegram webhooks or SMTP stay outside the settings table.</p>
    </div>
    <div class="notification-card-actions">
      <span class="badge gray">Optional</span>
      <button class="btn primary" @click="openNotifications">Configure notifications</button>
    </div>
  </section>

  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions remote v-bind="table" empty="No settings found matching search." @query="queryTable">
    <template #col-key="{ value }"><span class="mono">{{ value }}</span></template>
    <template #col-category="{ value }"><span class="badge blue">{{ value }}</span></template>
    <template #col-value="{ value }"><span class="faint">{{ preview(value) }}</span></template>
    <template #col-restart_required="{ value }"><span :class="value ? 'badge warn' : 'faint'">{{ value ? 'Required' : 'No' }}</span></template>
    <template #col-updated_at="{ value }"><span class="faint">{{ fmt(value) }}</span></template>
    <template #actions="{ row }">
      <button class="btn icon" title="Edit" @click="openEdit(row)"><Icon name="edit" /></button>
    </template>
  </UiTable>

  <UiModal :open="modalOpen" title="Edit Setting" @close="modalOpen = false">
    <div class="field"><label>Key</label><input v-model="form.key" class="mono" disabled /></div>
    <div class="field"><label>Value ({{ form.type }}, JSON encoded)</label><textarea v-model="form.value" rows="8" class="mono"></textarea></div>
    <template #footer>
      <button class="btn primary" @click="save" :disabled="saving">
        <span v-if="saving" class="spin" /> Save
      </button>
    </template>
  </UiModal>
</template>
