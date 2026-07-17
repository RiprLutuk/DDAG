<script setup lang="ts">
interface Column { key: string; label: string; mono?: boolean; sortable?: boolean }
const props = withDefaults(defineProps<{
  columns: Column[]; rows: any[]; loading?: boolean; empty?: string; error?: string; hasActions?: boolean
  retry?: () => void
  remote?: boolean; page?: number; limit?: number; total?: number; search?: string; sortBy?: string; sortDir?: string
}>(), { page: 1, limit: 10, total: 0, search: '', sortBy: '', sortDir: 'desc' })
const emit = defineEmits<{
  query: [value: { page: number; limit: number; search: string; sortBy: string; sortDir: string }]
}>()
const localSearch = ref(props.search)
let timer: ReturnType<typeof setTimeout> | undefined
watch(() => props.search, v => { localSearch.value = v })
const pages = computed(() => Math.max(1, Math.ceil(props.total / props.limit)))
function send(overrides: Record<string, any> = {}) {
  emit('query', { page: props.page, limit: props.limit, search: localSearch.value, sortBy: props.sortBy, sortDir: props.sortDir, ...overrides })
}
function onSearch() { clearTimeout(timer); timer = setTimeout(() => send({ page: 1, search: localSearch.value }), 300) }
function sort(c: Column) {
  if (!props.remote || c.sortable === false) return
  send({ page: 1, sortBy: c.key, sortDir: props.sortBy === c.key && props.sortDir === 'asc' ? 'desc' : 'asc' })
}
</script>

<template>
  <div class="card table-card">
    <div v-if="remote" class="table-tools">
      <label class="table-length">Show <select :value="limit" class="page-size" @change="send({ page: 1, limit: Number(($event.target as HTMLSelectElement).value) })">
        <option :value="10">10 / page</option><option :value="25">25 / page</option><option :value="50">50 / page</option><option :value="100">100 / page</option>
      </select> entries</label>
      <div class="spacer" />
      <label class="table-search-label">Search: <input v-model="localSearch" class="search" @input="onSearch" /></label>
    </div>
    <div v-if="loading" class="loading table-state"><span class="spin" /> Loading…</div>
    <div v-else-if="error" class="empty table-state table-error"><div class="state-orb danger">!</div><b>Unable to load data.</b><span>{{ error }}</span><button v-if="retry" class="btn sm" @click="retry">Retry</button></div>
    <div v-else-if="!rows.length" class="empty table-state"><div class="state-orb">⌁</div><b>{{ empty || 'No records found.' }}</b><span>No matching records are available for this view.</span></div>
    <div v-else>
      <div class="table-wrap table-desktop">
        <table class="tbl"><thead><tr>
          <th v-for="c in columns" :key="c.key" :class="{ sortable: remote && c.sortable !== false }" @click="sort(c)">
            {{ c.label }} <span v-if="sortBy === c.key" class="sort-mark">{{ sortDir === 'asc' ? '↑' : '↓' }}</span>
          </th>
          <th v-if="hasActions" style="text-align:right">Actions</th>
        </tr></thead><tbody>
          <tr v-for="(row, i) in rows" :key="row.id || i">
            <td v-for="c in columns" :key="c.key" :class="{ mono: c.mono }"><slot :name="`col-${c.key}`" :row="row" :value="row[c.key]">{{ row[c.key] }}</slot></td>
            <td v-if="hasActions" style="text-align:right;white-space:nowrap"><slot name="actions" :row="row" /></td>
          </tr>
        </tbody></table>
      </div>
      <div class="mobile-cards table-mobile">
        <div v-for="(row, i) in rows" :key="row.id || i" class="mobile-card"><div class="mobile-card-body">
          <div v-for="c in columns" :key="c.key" class="mobile-card-row"><span class="mobile-card-label">{{ c.label }}</span><div class="mobile-card-val" :class="{ mono: c.mono }"><slot :name="`col-${c.key}`" :row="row" :value="row[c.key]">{{ row[c.key] }}</slot></div></div>
        </div><div v-if="hasActions" class="mobile-card-actions"><slot name="actions" :row="row" /></div></div>
      </div>
    </div>
    <div v-if="remote && total > 0" class="table-pagination">
      <button class="btn sm" :disabled="page <= 1 || loading" @click="send({ page: page - 1 })">Previous</button>
      <span>Showing {{ total ? ((page - 1) * limit) + 1 : 0 }} to {{ Math.min(page * limit, total) }} of {{ total }} entries</span>
      <button class="btn sm" :disabled="page >= pages || loading" @click="send({ page: page + 1 })">Next</button>
    </div>
  </div>
</template>
