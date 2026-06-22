<script setup lang="ts">
const { toasts } = useToast()
</script>

<template>
  <div class="toasts">
    <div v-for="t in toasts" :key="t.id" class="toast" :class="t.type">
      <div class="t-title">{{ t.title }}</div>
      <div v-if="t.msg" class="t-msg">{{ t.msg }}</div>
    </div>
  </div>
</template>
