<script setup lang="ts">
const props = defineProps<{ status: string }>()
const map: Record<string, string> = {
  active: 'green', healthy: 'green', PUBLISHED: 'green', success: 'green', ready: 'green', closed: 'green',
  inactive: 'gray', DRAFT: 'gray', ARCHIVED: 'gray', unknown: 'gray',
  DISABLED: 'red', failure: 'red', unhealthy: 'red', unreachable: 'red', open: 'red',
  REVIEW: 'amber', pending: 'amber', half_open: 'amber',
}
const cls = computed(() => map[props.status] || 'blue')
</script>

<template>
  <span class="badge" :class="cls">{{ status }}</span>
</template>
