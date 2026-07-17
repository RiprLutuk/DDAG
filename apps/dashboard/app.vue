<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import DefaultLayout from './layouts/default.vue'
import BlankLayout from './layouts/blank.vue'
import ToastHost from './components/ToastHost.vue'

const route = useRoute()
const layout = computed(() => route.meta.layout === 'blank' ? BlankLayout : DefaultLayout)
</script>

<template>
  <component :is="layout">
    <RouterView />
  </component>
  <ToastHost />
</template>
