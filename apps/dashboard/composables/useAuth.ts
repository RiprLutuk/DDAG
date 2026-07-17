import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useApi } from './useApi'

// useAuth holds the dashboard session state (current user + effective
// permissions) and exposes login/logout/permission helpers.
interface AuthState {
  user: any | null
  permissions: string[]
  roles: string[]
  ready: boolean
}

const state = ref<AuthState>({ user: null, permissions: [], roles: [], ready: false })

export function useAuth() {
  const router = useRouter()
  const api = useApi()

  async function fetchMe(): Promise<boolean> {
    try {
      const data = await api.get('/auth/me')
      state.value = { user: data.user, permissions: data.permissions || [], roles: data.roles || data.user?.roles || [], ready: true }
      return true
    } catch {
      state.value = { user: null, permissions: [], roles: [], ready: true }
      return false
    }
  }

  async function login(login: string, password: string) {
    const data = await api.post('/auth/login', { login, password })
    state.value = { user: data.user, permissions: data.permissions || [], roles: data.user?.roles || [], ready: true }
  }

  async function logout() {
    try { await api.post('/auth/logout') } catch { /* ignore */ }
    state.value = { user: null, permissions: [], roles: [], ready: true }
    await router.push('/login')
  }

  const has = (permission: string) => state.value.permissions.includes(permission)
  const hasAny = (...permissions: string[]) => permissions.some((permission) => state.value.permissions.includes(permission))

  return { state, fetchMe, login, logout, has, hasAny }
}
