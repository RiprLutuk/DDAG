<script setup lang="ts">
defineProps<{ label: string; value: string | number; sub?: string; tone?: 'blue' | 'green' | 'amber' | 'red' | 'violet' }>()
</script>

<template>
  <div class="stat" :class="tone ? `tone-${tone}` : ''">
    <div class="stat-top">
      <div class="label">{{ label }}</div>
      <div class="stat-pulse" />
    </div>
    <div class="value">{{ value }}</div>
    <div class="sub">{{ sub || 'Live metric' }}</div>
  </div>
</template>
