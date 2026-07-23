export { openExternalURL } from './externalURL'

function bindings() {
  if (typeof window === 'undefined') return null
  return window.go?.bindings?.ExplorerBindings || null
}

function requireBindings() {
  const value = bindings()
  if (!value) throw new Error('The remote explorer runtime is unavailable.')
  return value
}

export function hasExplorerRuntime() {
  return bindings() !== null
}

export async function initialState() {
  return requireBindings().InitialState()
}

export async function connect(passphrase = '') {
  return requireBindings().Connect(passphrase)
}

export async function listDirectory(path) {
  return requireBindings().ListDirectory(path)
}

export async function previewFile(path) {
  return requireBindings().PreviewFile(path)
}

export async function saveFile(path, content, expectedRevision) {
  return requireBindings().SaveFile(path, content, expectedRevision)
}

export async function download(paths) {
  return requireBindings().Download(paths)
}

export async function close() {
  return requireBindings().Close()
}

export const remoteContentPrefix = '/__ssh_man_remote__/raw'

export function remoteContentURL(remotePath) {
  const encoded = String(remotePath || '')
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/')
  return `${remoteContentPrefix}${encoded.startsWith('/') ? encoded : `/${encoded}`}`
}
