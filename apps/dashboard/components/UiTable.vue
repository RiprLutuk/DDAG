<script setup lang="ts">
// Generic table. Columns: [{ key, label, mono? }]. Override a cell with a
// scoped slot named `col-<key>` ({ row, value }); add an actions column with the
// `actions` scoped slot ({ row }).
interface Column { key: string; label: string; mono?: boolean }
defineProps<{ columns: Column[]; rows: any[]; loading?: boolean; empty?: string; hasActions?: boolean }>()
</script>

<template>
  <div class="card">
    <div v-if="loading" class="loading"><span class="spin" /> Loading…</div>
    <div v-else-if="!rows.length" class="empty">{{ empty || 'No records found.' }}</div>
    <div v-else class="table-wrap">
      <table class="tbl">
        <thead>
          <tr>
            <th v-for="c in columns" :key="c.key">{{ c.label }}</th>
            <th v-if="hasActions" style="text-align:right">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, i) in rows" :key="row.id || i">
            <td v-for="c in columns" :key="c.key" :class="{ mono: c.mono }">
              <slot :name="`col-${c.key}`" :row="row" :value="row[c.key]">{{ row[c.key] }}</slot>
            </td>
            <td v-if="hasActions" style="text-align:right;white-space:nowrap">
              <slot name="actions" :row="row" />
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
