function getRuntimeBindings() {
  if (typeof window === 'undefined') return null
  return window.go?.bindings?.AppBindings || window.go?.main?.AppBindings || null
}

const hasWailsRuntime = () => getRuntimeBindings() !== null

const memoryState = {
  servers: [],
  preferences: { theme: 'dark', lastSelectedServerId: '' },
  sessions: [],
}

function id() {
  return Math.random().toString(16).slice(2, 14)
}

function syncSession(session) {
  const next = memoryState.sessions.filter((item) => item.configurationId !== session.configurationId)
  next.push(session)
  memoryState.sessions = next
  return session
}

function cloneState(value) {
  if (typeof structuredClone === 'function') {
    return structuredClone(value)
  }
  return JSON.parse(JSON.stringify(value))
}

function appBindings() {
  return getRuntimeBindings()
}

export async function loadInitialState() {
  if (hasWailsRuntime()) {
    return appBindings().LoadInitialState()
  }
  return {
    ...cloneState(memoryState),
    diagnostics: {
      appDataPath: '~/.config/ssh-man',
      databasePath: '~/.config/ssh-man/ssh-man.db',
    },
    recoverable: false,
    message: '',
  }
}

export async function saveServer(server) {
  if (hasWailsRuntime()) {
    return appBindings().SaveServer(server)
  }
  const next = { ...server, id: server.id || id() }
  memoryState.servers = memoryState.servers.filter((item) => item.server.id !== next.id)
  memoryState.servers.push({ server: next, configurations: [] })
  return next
}

export async function deleteServer(serverId) {
  if (hasWailsRuntime()) {
    return appBindings().DeleteServer(serverId)
  }
  memoryState.servers = memoryState.servers.filter((item) => item.server.id !== serverId)
}

export async function saveConnectionConfiguration(configuration) {
  if (hasWailsRuntime()) {
    return appBindings().SaveConnectionConfiguration(configuration)
  }
  const next = { ...configuration, id: configuration.id || id() }
  memoryState.servers = memoryState.servers.map((item) => {
    if (item.server.id !== next.serverId) return item
    const configurations = item.configurations.filter((entry) => entry.id !== next.id)
    configurations.push(next)
    return { ...item, configurations }
  })
  return next
}

export async function deleteConnectionConfiguration(configurationId) {
  if (hasWailsRuntime()) {
    return appBindings().DeleteConnectionConfiguration(configurationId)
  }
  memoryState.servers = memoryState.servers.map((item) => ({
    ...item,
    configurations: item.configurations.filter((entry) => entry.id !== configurationId),
  }))
}

export async function savePreferences(preferences) {
  if (hasWailsRuntime()) {
    return appBindings().SavePreferences(preferences)
  }
  memoryState.preferences = { ...memoryState.preferences, ...preferences }
  return memoryState.preferences
}

export async function startConfiguration(configurationId) {
  if (hasWailsRuntime()) {
    return appBindings().StartConfiguration(configurationId)
  }
  const configuration = memoryState.servers.flatMap((item) => item.configurations).find((item) => item.id === configurationId)
  const boundPort = configuration?.connectionType === 'socks_proxy' ? (configuration.socksPort || 43123) : (configuration?.localPort || 0)
  return syncSession({ configurationId, status: 'connected', boundPort, statusDetail: configuration?.connectionType === 'socks_proxy' ? `Listening on localhost:${boundPort}` : 'Mock tunnel connected' })
}

export async function startServerConfigurations(serverId) {
  if (hasWailsRuntime()) {
    return appBindings().StartServerConfigurations(serverId)
  }
  const server = memoryState.servers.find((item) => item.server.id === serverId)
  if (!server) return []
  return server.configurations.map((configuration, index) => {
    const boundPort = configuration.connectionType === 'socks_proxy' ? (configuration.socksPort || 43123 + index) : (configuration.localPort || 0)
    return syncSession({ configurationId: configuration.id, status: 'connected', boundPort, statusDetail: configuration.connectionType === 'socks_proxy' ? `Listening on localhost:${boundPort}` : 'Mock tunnel connected' })
  })
}

export async function stopConfiguration(configurationId) {
  if (hasWailsRuntime()) {
    return appBindings().StopConfiguration(configurationId)
  }
  return syncSession({ configurationId, status: 'stopped', statusDetail: 'Mock tunnel stopped' })
}

export async function retryConfiguration(configurationId) {
  if (hasWailsRuntime()) {
    return appBindings().RetryConfiguration(configurationId)
  }
  const configuration = memoryState.servers.flatMap((item) => item.configurations).find((item) => item.id === configurationId)
  const boundPort = configuration?.connectionType === 'socks_proxy' ? (configuration.socksPort || 43123) : (configuration?.localPort || 0)
  return syncSession({ configurationId, status: 'connected', boundPort, statusDetail: configuration?.connectionType === 'socks_proxy' ? `Listening on localhost:${boundPort}` : 'Mock tunnel reconnected' })
}

export async function submitKeyUnlock(configurationId, secret) {
  if (hasWailsRuntime()) {
    return appBindings().SubmitKeyUnlock(configurationId, secret)
  }
  const configuration = memoryState.servers.flatMap((item) => item.configurations).find((item) => item.id === configurationId)
  const boundPort = configuration?.connectionType === 'socks_proxy' ? (configuration.socksPort || 43123) : (configuration?.localPort || 0)
  return syncSession({ configurationId, status: secret ? 'connected' : 'needs_attention', boundPort: secret ? boundPort : 0, statusDetail: secret ? (configuration?.connectionType === 'socks_proxy' ? `Listening on localhost:${boundPort}` : 'Mock key unlocked') : 'Unlock required' })
}

export async function discoverBrowsers() {
  if (hasWailsRuntime()) {
    return appBindings().DiscoverBrowsers()
  }
  return [
    { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    { id: 'chromium', displayName: 'Chromium', supportsProxyLaunch: true },
    { id: 'firefox', displayName: 'Firefox', supportsProxyLaunch: true },
  ]
}

export async function previewBrowserLaunchThroughSocks(configurationId, browserId) {
  if (hasWailsRuntime()) {
    return appBindings().PreviewBrowserLaunchThroughSocks(configurationId, browserId)
  }

  const configuration = memoryState.servers.flatMap((item) => item.configurations).find((item) => item.id === configurationId)
  const session = memoryState.sessions.find((item) => item.configurationId === configurationId)
  const boundPort = session?.boundPort || configuration?.socksPort || 43123
  return {
    configurationId,
    browserId,
    browserName: browserId,
    supported: true,
    command: browserId === 'firefox'
      ? `firefox -new-instance -profile /tmp/ssh-man-browser-profiles/firefox-${boundPort}`
      : `${browserId} --proxy-server=socks5://127.0.0.1:${boundPort} --proxy-bypass-list=<-loopback> --host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1`,
  }
}

export async function launchBrowserThroughSocks(configurationId, browserId) {
  if (hasWailsRuntime()) {
    return appBindings().LaunchBrowserThroughSocks(configurationId, browserId)
  }
  return { configurationId, browserId, success: true }
}

export async function openDevTools() {
  if (hasWailsRuntime()) {
    return appBindings().OpenDevTools()
  }
}

export async function listRuntimeSessions() {
  if (hasWailsRuntime()) {
    return appBindings().ListRuntimeSessions()
  }
  return cloneState(memoryState.sessions)
}
