<script setup lang="ts">
defineProps<{ label: string; value: string | number; sub?: string }>()
</script>

<template>
  <div class="stat">
    <div class="label">{{ label }}</div>
    <div class="value">{{ value }}</div>
    <div v-if="sub" class="sub">{{ sub }}</div>
  </div>
</template>
