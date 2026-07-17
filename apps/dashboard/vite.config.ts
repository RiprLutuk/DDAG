import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'

const dashboardAutoImports = (): Plugin => ({
  name: 'ddag-dashboard-auto-imports',
  enforce: 'pre',
  transform(code, id) {
    if (!id.endsWith('.vue') || id.endsWith('/app.vue')) return null
    const match = code.match(/<script\s+setup(?:\s+lang="ts")?\s*>/)
    if (!match || match.index === undefined) return null
    const imports = `\nimport { computed, nextTick, onBeforeUnmount, onMounted, onUnmounted, reactive, readonly, ref, watch, useRoute, useRouter, useApi, useAuth, useTheme, useToast } from '../src/dashboard-globals'\n`
    const index = match.index + match[0].length
    return `${code.slice(0, index)}${imports}${code.slice(index)}`
  },
})

export default defineConfig({
  plugins: [vue(), dashboardAutoImports()],
  build: { outDir: 'dist', emptyOutDir: true },
})
