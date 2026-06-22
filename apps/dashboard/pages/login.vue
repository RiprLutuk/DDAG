<script setup lang="ts">
definePageMeta({ layout: 'blank' })
const { login } = useAuth()
const form = reactive({ login: 'superadmin', password: '' })
const loading = ref(false)
const error = ref('')

async function submit() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    await login(form.login, form.password)
    await navigateTo('/')
  } catch (e: any) {
    error.value = e?.message || 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <form class="login-card" @submit.prevent="submit">
      <div class="logo">
        <b>DDAG</b>
        <span>Dynamic Database API Gateway</span>
      </div>
      <div class="field">
        <label>Username or email</label>
        <input v-model="form.login" autocomplete="username" autofocus />
      </div>
      <div class="field">
        <label>Password</label>
        <input v-model="form.password" type="password" autocomplete="current-password" />
      </div>
      <p v-if="error" class="error-text" style="margin:0 0 12px">{{ error }}</p>
      <button class="btn primary" style="width:100%;justify-content:center" :disabled="loading">
        <span v-if="loading" class="spin" /> Sign in
      </button>
      <p class="faint" style="text-align:center;margin:16px 0 0;font-size:12px">
        Demo: <span class="mono">superadmin</span> / <span class="mono">Admin#12345</span>
      </p>
    </form>
  </div>
</template>
