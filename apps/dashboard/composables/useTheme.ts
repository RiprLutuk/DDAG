import { readonly, ref } from 'vue'

type ThemeMode = 'dark' | 'light'

const STORAGE_KEY = 'ddag.theme'
const theme = ref<ThemeMode>('dark')

function applyTheme(mode: ThemeMode) {
  theme.value = mode
  document.documentElement.dataset.theme = mode
  document.documentElement.style.colorScheme = mode
  localStorage.setItem(STORAGE_KEY, mode)
}

export function useTheme() {
  const initTheme = () => {
      const saved = localStorage.getItem(STORAGE_KEY) as ThemeMode | null
    const mode: ThemeMode = saved === 'light' || saved === 'dark' ? saved : 'dark'
    applyTheme(mode)
  }
  const toggleTheme = () => applyTheme(theme.value === 'dark' ? 'light' : 'dark')
  return { theme: readonly(theme), initTheme, setTheme: applyTheme, toggleTheme }
}
