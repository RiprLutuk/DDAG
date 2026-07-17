// useApi wraps fetch with the admin-backend base URL, cookie credentials, and
// DDAG envelope unwrapping. Errors are normalized to { code, message, status }.
export interface ApiError { code: string; message: string; status?: number }
export interface ListResult<T> { items: T[]; pagination: { page: number; limit: number; total: number } }

export function useApi() {
  const base = import.meta.env.VITE_API_BASE || ''

  function csrfToken() {
    return document.cookie.split(';').map((v) => v.trim()).find((v) => v.startsWith('ddag_csrf='))?.split('=')[1] || ''
  }

  async function call<T = any>(method: string, path: string, body?: any): Promise<T> {
    const headers: Record<string, string> = {}
    if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
      const token = csrfToken()
      if (token) headers['X-CSRF-Token'] = decodeURIComponent(token)
    }
    let response: Response
    try {
      response = await fetch(base + path, {
        method,
        body: body === undefined ? undefined : JSON.stringify(body),
        headers: body === undefined ? headers : { ...headers, 'Content-Type': 'application/json' },
        credentials: 'include',
      })
    } catch (e: any) {
      throw { code: 'NETWORK_ERROR', message: e?.message || 'Network request failed' } satisfies ApiError
    }

    const env = await response.json().catch(() => null)
    if (!response.ok) {
      throw {
        code: env?.error?.code || 'ERROR',
        message: env?.error?.message || response.statusText || 'Request failed',
        status: response.status,
      } satisfies ApiError
    }
    return env
  }

  return {
    get: async <T = any>(path: string): Promise<T> => (await call<any>('GET', path)).data,
    post: async <T = any>(path: string, body?: any): Promise<T> => (await call<any>('POST', path, body)).data,
    put: async <T = any>(path: string, body?: any): Promise<T> => (await call<any>('PUT', path, body)).data,
    del: async <T = any>(path: string): Promise<T> => (await call<any>('DELETE', path)).data,
    list: async <T = any>(path: string): Promise<ListResult<T>> => {
      const res = await call<any>('GET', path)
      return { items: res.data || [], pagination: res.pagination || { page: 1, limit: res.data?.length || 0, total: res.data?.length || 0 } }
    },
  }
}
