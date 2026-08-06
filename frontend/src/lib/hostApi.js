function bindings() {
  if (typeof window === 'undefined') return null
  return window.go?.bindings?.HostBindings || null
}

function requireBindings() {
  const value = bindings()
  if (!value) throw new Error('The host window runtime is unavailable.')
  return value
}

export function hasHostRuntime() {
  return bindings() !== null
}

export async function initialState() {
  return requireBindings().InitialState()
}

export async function discoverPorts(passphrase = '') {
  return requireBindings().DiscoverPorts(passphrase)
}

export async function savePortLink(link) {
  return requireBindings().SavePortLink(link)
}

export async function deletePortLink(id) {
  return requireBindings().DeletePortLink(id)
}

export async function findPortFavicon(port, scheme) {
  return requireBindings().FindPortFavicon(port, scheme)
}

export async function openPort(port, scheme) {
  return requireBindings().OpenPort(port, scheme)
}

export async function close() {
  return requireBindings().Close()
}
