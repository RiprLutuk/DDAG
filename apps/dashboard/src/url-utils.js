export function joinGatewayPath(base, endpointPath) {
  const normalizedBase = String(base || '').replace(/\/+$/, '')
  const normalizedPath = `/${String(endpointPath || '').replace(/^\/+/, '')}`
  if (!normalizedBase) return normalizedPath

  const absolute = /^[a-z][a-z\d+.-]*:\/\//i.test(normalizedBase)
  const parsedBase = new URL(normalizedBase, 'http://ddag.local')
  const basePath = parsedBase.pathname.replace(/\/+$/, '')
  const joinedPath = basePath && (normalizedPath === basePath || normalizedPath.startsWith(`${basePath}/`))
    ? normalizedPath
    : `${basePath}${normalizedPath}`

  return absolute ? `${parsedBase.origin}${joinedPath}` : joinedPath
}
