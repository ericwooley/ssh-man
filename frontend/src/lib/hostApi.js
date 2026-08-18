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

export async function loadInitialState() {
  return requireBindings().LoadAppState()
}

export async function listRuntimeSessions() {
  return requireBindings().ListRuntimeSessions()
}

export async function listSessionHistory(configurationId) {
  return requireBindings().ListSessionHistory(configurationId)
}

export async function savePreferences(preferences) {
  return preferences
}

export async function saveServer(server) {
  return requireBindings().SaveServer(server)
}

export async function deleteServer(serverId) {
  return requireBindings().DeleteServer(serverId)
}

export async function saveConnectionConfiguration(configuration) {
  return requireBindings().SaveConnectionConfiguration(configuration)
}

export async function deleteConnectionConfiguration(configurationId) {
  return requireBindings().DeleteConnectionConfiguration(configurationId)
}

export async function startConfiguration(configurationId) {
  return requireBindings().StartConfiguration(configurationId)
}

export async function startServerConfigurations(serverId) {
  return requireBindings().StartServerConfigurations(serverId)
}

export async function stopConfiguration(configurationId) {
  return requireBindings().StopConfiguration(configurationId)
}

export async function retryConfiguration(configurationId) {
  return requireBindings().RetryConfiguration(configurationId)
}

export async function submitKeyUnlock(configurationId, secret) {
  return requireBindings().SubmitKeyUnlock(configurationId, secret)
}

export async function discoverBrowsers() {
  return requireBindings().DiscoverBrowsers()
}

export async function previewBrowserLaunchThroughSocks(configurationId, browserId) {
  return requireBindings().PreviewBrowserLaunchThroughSocks(configurationId, browserId)
}

export async function launchBrowserThroughSocks(configurationId, browserId) {
  return requireBindings().LaunchBrowserThroughSocks(configurationId, browserId)
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
