<script setup lang="ts">
defineProps<{ eyebrow?: string; title: string; description?: string; icon?: string }>()
</script>

<template>
  <div class="page-header">
    <div class="page-header-main">
      <div v-if="icon" class="page-header-icon"><Icon :name="icon" :size="18" /></div>
      <div>
        <div v-if="eyebrow" class="eyebrow">{{ eyebrow }}</div>
        <h2>{{ title }}</h2>
        <p v-if="description" class="page-desc">{{ description }}</p>
      </div>
    </div>
    <div v-if="$slots.actions" class="page-header-actions"><slot name="actions" /></div>
  </div>
</template>
