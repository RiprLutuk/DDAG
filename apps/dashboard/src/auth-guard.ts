import type { NavigationGuard } from 'vue-router'
import { useAuth } from '../composables/useAuth'

export const authGuard: NavigationGuard = async (to) => {
  const { state, fetchMe } = useAuth()
  if (!state.value.ready) await fetchMe()

  const authenticated = Boolean(state.value.user)
  if (to.path === '/login') return authenticated ? '/' : true
  return authenticated ? true : '/login'
}
