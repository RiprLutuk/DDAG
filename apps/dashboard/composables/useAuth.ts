// useAuth holds the dashboard session state (current user + effective
// permissions) and exposes login/logout/permission helpers.
interface AuthState {
  user: any | null
  permissions: string[]
  roles: string[]
  ready: boolean
}

export function useAuth() {
  const state = useState<AuthState>('auth', () => ({ user: null, permissions: [], roles: [], ready: false }))
  const api = useApi()

  async function fetchMe(): Promise<boolean> {
    try {
      const d = await api.get('/auth/me')
      state.value = { user: d.user, permissions: d.permissions || [], roles: d.roles || d.user?.roles || [], ready: true }
      return true
    } catch {
      state.value = { user: null, permissions: [], roles: [], ready: true }
      return false
    }
  }

  async function login(login: string, password: string) {
    const d = await api.post('/auth/login', { login, password })
    state.value = { user: d.user, permissions: d.permissions || [], roles: d.user?.roles || [], ready: true }
  }

  async function logout() {
    try { await api.post('/auth/logout') } catch { /* ignore */ }
    state.value = { user: null, permissions: [], roles: [], ready: true }
    await navigateTo('/login')
  }

  const has = (perm: string) => state.value.permissions.includes(perm)
  const hasAny = (...perms: string[]) => perms.some((p) => state.value.permissions.includes(p))

  return { state, fetchMe, login, logout, has, hasAny }
}
