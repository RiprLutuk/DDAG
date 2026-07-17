<script setup lang="ts">
const api = useApi()
const toast = useToast()
const rows = ref<any[]>([])
const loading = ref(true)
const running = ref('')
const columns = [
  { key: 'name', label: 'Job' },
  { key: 'category', label: 'Category' },
  { key: 'last_status', label: 'Last Status' },
  { key: 'last_run_at', label: 'Last Run' },
  { key: 'last_duration_ms', label: 'Duration ms' },
]
const fmt = (v: string) => (v ? new Date(v).toLocaleString() : 'Never')
async function load() { loading.value = true; try { const res = await api.list('/api/jobs'); rows.value = res.items || res } finally { loading.value = false } }
async function run(row: any) { running.value = row.key; try { await api.post(`/api/jobs/${encodeURIComponent(row.key)}/run`, {}); toast.success('Job completed', row.name); await load() } catch (e: any) { toast.error('Job failed', e.message) } finally { running.value = '' } }
onMounted(load)
</script>
<template>
  <p class="page-desc">Allowlisted safe maintenance jobs for DDAG operations.</p>
  <div class="toolbar compact-mobile"><div class="spacer" /><button class="btn" @click="load">Refresh</button></div>
  <UiTable :columns="columns" :rows="rows" :loading="loading" has-actions empty="No jobs registered.">
    <template #col-name="{ row }"><div><b>{{ row.name }}</b><div class="faint">{{ row.description }}</div></div></template>
    <template #col-last_run_at="{ value }"><span class="faint">{{ fmt(value) }}</span></template>
    <template #actions="{ row }"><button class="btn small primary" :disabled="running === row.key || !row.safe" @click="run(row)"><span v-if="running === row.key" class="spin" /> Run</button></template>
  </UiTable>
</template>
