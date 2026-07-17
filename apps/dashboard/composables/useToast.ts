import { ref } from 'vue'

// Shared transient notifications for the dashboard SPA.
interface Toast { id: number; type: 'success' | 'error' | 'info'; title: string; msg?: string }

const toasts = ref<Toast[]>([])

export function useToast() {
  function push(type: Toast['type'], title: string, msg?: string) {
    const id = Date.now() + Math.floor(Math.random() * 1000)
    toasts.value = [...toasts.value, { id, type, title, msg }]
    window.setTimeout(() => { toasts.value = toasts.value.filter((toast) => toast.id !== id) }, 4500)
  }
  return {
    toasts,
    success: (title: string, msg?: string) => push('success', title, msg),
    error: (title: string, msg?: string) => push('error', title, msg),
    info: (title: string, msg?: string) => push('info', title, msg),
  }
}
