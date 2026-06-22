// Global route guard: ensures a session before showing any page except /login.
// Permissions only fetched once per app load (then cached in useAuth state).
export default defineNuxtRouteMiddleware(async (to) => {
  const { state, fetchMe } = useAuth()
  if (!state.value.ready) {
    await fetchMe()
  }
  const authed = !!state.value.user
  if (to.path === '/login') {
    if (authed) return navigateTo('/')
    return
  }
  if (!authed) {
    return navigateTo('/login')
  }
})
