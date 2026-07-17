<script setup lang="ts">
const props = defineProps<{ open: boolean; title: string; wide?: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

// A route transition or an old cached script must never render modal content as
// the page body. Render it only after this component is mounted and explicitly open.
const mounted = ref(false)
const isVisible = computed(() => mounted.value === true && props.open === true)

function close() {
  emit('close')
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && isVisible.value) close()
}

watch(isVisible, (visible) => {
  if (!mounted.value) return
  document.body.classList.toggle('modal-open', visible)
}, { immediate: true })

onMounted(() => {
  mounted.value = true
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  if (typeof document !== 'undefined') document.body.classList.remove('modal-open')
})
</script>

<template>
  <Teleport to="body">
    <Transition name="modal-fade">
      <div v-if="isVisible" class="modal-backdrop" role="presentation" @click.self="close">
        <section
          class="modal flex-modal"
          :class="{ lg: wide }"
          role="dialog"
          aria-modal="true"
          :aria-label="title"
        >
          <div class="modal-head">
            <h3>{{ title }}</h3>
            <button class="x-close" type="button" aria-label="Close modal" @click="close">×</button>
          </div>
          <div class="modal-body flex-scroll">
            <slot />
          </div>
          <div class="modal-foot">
            <slot name="footer" />
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>
