export const vimModeStorageKey = 'ssh-man:explorer:vim-enabled'

export function favoriteStorageKey(serverID) {
  return `ssh-man:explorer:favorites:${serverID}`
}

function normalizeRemotePath(value) {
  const path = String(value || '').trim()
  if (!path) return ''
  if (path === '/') return path
  return path.replace(/\/+$/, '')
}

function normalizedFavorites(paths) {
  return [...new Set((Array.isArray(paths) ? paths : [])
    .map(normalizeRemotePath)
    .filter(Boolean))]
    .sort((left, right) => left.localeCompare(right))
}

export function addFavorite(paths, remotePath) {
  return normalizedFavorites([...(Array.isArray(paths) ? paths : []), remotePath])
}

export function removeFavorite(paths, remotePath) {
  const target = normalizeRemotePath(remotePath)
  return normalizedFavorites(paths).filter((path) => path !== target)
}

export function readFavoritePaths(storage, serverID) {
  if (!storage || !serverID) return []
  try {
    return normalizedFavorites(JSON.parse(storage.getItem(favoriteStorageKey(serverID)) || '[]'))
  } catch {
    return []
  }
}

export function writeFavoritePaths(storage, serverID, paths) {
  const next = normalizedFavorites(paths)
  if (storage && serverID) storage.setItem(favoriteStorageKey(serverID), JSON.stringify(next))
  return next
}

export function readVimMode(storage) {
  return storage?.getItem(vimModeStorageKey) === 'true'
}

export function writeVimMode(storage, enabled) {
  storage?.setItem(vimModeStorageKey, enabled ? 'true' : 'false')
  return enabled
}
