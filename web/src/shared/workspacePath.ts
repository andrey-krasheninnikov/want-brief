const prefix = /^\/w\/([\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12})(?:\/|$)/i

export function workspaceFromPath(path: string): string | undefined {
  return prefix.exec(path)?.[1]
}

export function workspaceUrl(id: string, page = '/today'): string {
  return `/w/${encodeURIComponent(id)}${page}`
}
