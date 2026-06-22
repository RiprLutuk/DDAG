// useApi wraps $fetch with the admin-backend base URL, cookie credentials, and
// DDAG envelope unwrapping. Errors are normalized to { code, message, status }.
export interface ApiError { code: string; message: string; status?: number }
export interface ListResult<T> { items: T[]; pagination: { page: number; limit: number; total: number } }

export function useApi() {
  const base = useRuntimeConfig().public.apiBase as string

  async function call<T = any>(method: string, path: string, body?: any): Promise<any> {
    try {
      return await $fetch<any>(base + path, {
        method: method as any,
        body,
        credentials: 'include',
      })
    } catch (e: any) {
      const env = e?.data
      const err: ApiError = {
        code: env?.error?.code || 'ERROR',
        message: env?.error?.message || e?.message || 'Request failed',
        status: e?.status || e?.statusCode,
      }
      throw err
    }
  }

  return {
    // Single-object endpoints: returns the `data` field.
    get: async <T = any>(path: string): Promise<T> => (await call('GET', path)).data,
    post: async <T = any>(path: string, body?: any): Promise<T> => (await call('POST', path, body)).data,
    put: async <T = any>(path: string, body?: any): Promise<T> => (await call('PUT', path, body)).data,
    del: async <T = any>(path: string): Promise<T> => (await call('DELETE', path)).data,
    // List endpoints: returns { items, pagination }.
    list: async <T = any>(path: string): Promise<ListResult<T>> => {
      const res = await call('GET', path)
      return { items: res.data || [], pagination: res.pagination || { page: 1, limit: res.data?.length || 0, total: res.data?.length || 0 } }
    },
  }
}
