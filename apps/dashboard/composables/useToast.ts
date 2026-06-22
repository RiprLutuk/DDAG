// useToast provides transient notifications shown by ToastHost.
interface Toast { id: number; type: 'success' | 'error' | 'info'; title: string; msg?: string }

export function useToast() {
  const toasts = useState<Toast[]>('toasts', () => [])
  function push(type: Toast['type'], title: string, msg?: string) {
    const id = Date.now() + Math.floor(Math.random() * 1000)
    toasts.value = [...toasts.value, { id, type, title, msg }]
    setTimeout(() => { toasts.value = toasts.value.filter((t) => t.id !== id) }, 4500)
  }
  return {
    toasts,
    success: (title: string, msg?: string) => push('success', title, msg),
    error: (title: string, msg?: string) => push('error', title, msg),
    info: (title: string, msg?: string) => push('info', title, msg),
  }
}
