const ACTIVE_STATUSES = new Set(['starting', 'connected', 'reconnecting', 'needs_attention'])
const STARTABLE_STATUSES = new Set(['stopped', 'failed'])
const STOPPABLE_STATUSES = new Set(['starting', 'connected', 'reconnecting', 'needs_attention'])

function text(value) {
  return value == null ? '' : String(value).trim()
}

function firstNonEmpty(...values) {
  return values.find((value) => text(value) !== '') ?? ''
}

function firstDefined(...values) {
  return values.find((value) => value !== undefined && value !== null)
}

function statusValue(sessionOrStatus) {
  const value = typeof sessionOrStatus === 'string'
    ? sessionOrStatus
    : firstNonEmpty(sessionOrStatus?.status, sessionOrStatus?.Status)

  return text(value).toLowerCase() || 'stopped'
}

function isValidPort(value) {
  const candidate = text(value)
  if (!/^\d+$/.test(candidate)) return false

  const port = Number(candidate)
  return port >= 1 && port <= 65535
}

function isAutomaticSocksPort(value) {
  const candidate = text(value).toLowerCase()
  return candidate === '' || candidate === 'auto' || candidate === '0'
}

function socksPortMode(configuration, options) {
  const explicitMode = typeof options === 'string'
    ? options
    : options?.socksPortMode ?? configuration?.socksPortMode

  if (text(explicitMode) !== '') return text(explicitMode).toLowerCase()
  return isAutomaticSocksPort(configuration?.socksPort) ? 'auto' : 'manual'
}

function timestampValue(value) {
  const timestamp = Date.parse(value || '')
  return Number.isNaN(timestamp) ? 0 : timestamp
}

function positivePort(value) {
  return isValidPort(value) ? Number(text(value)) : 0
}

export function emptyServer(currentUsername = '') {
  return {
    id: '',
    name: '',
    host: 'localhost',
    port: '22',
    username: currentUsername ?? '',
    authMode: 'agent',
    keyReference: '',
  }
}

export function emptyTunnel(serverId = '') {
  return {
    serverId: serverId ?? '',
    label: '',
    connectionType: 'local_forward',
    localPort: '',
    remoteHost: '',
    remotePort: '',
    socksPort: '',
    autoReconnectEnabled: true,
    startOnLaunch: false,
    notes: '',
  }
}

export function validateServer(server = {}) {
  const errors = {}

  if (!text(server.name)) errors.name = 'A server name is required.'
  if (!text(server.host)) errors.host = 'A server host is required.'
  if (!text(server.username)) errors.username = 'An SSH username is required.'
  if (!isValidPort(server.port)) errors.port = 'Port must be between 1 and 65535.'
  if (server.authMode === 'private_key' && !text(server.keyReference)) {
    errors.keyReference = 'A private key path is required.'
  }

  return errors
}

export function validateTunnel(configuration = {}, options = {}) {
  const errors = {}
  const connectionType = text(configuration.connectionType)

  if (!text(configuration.label)) errors.label = 'A label is required.'

  if (connectionType === 'local_forward') {
    if (!text(configuration.localPort)) {
      errors.localPort = 'Local port is required.'
    } else if (!isValidPort(configuration.localPort)) {
      errors.localPort = 'Local port must be between 1 and 65535.'
    }

    if (!text(configuration.remoteHost)) errors.remoteHost = 'Remote host is required.'

    if (!text(configuration.remotePort)) {
      errors.remotePort = 'Remote port is required.'
    } else if (!isValidPort(configuration.remotePort)) {
      errors.remotePort = 'Remote port must be between 1 and 65535.'
    }
  } else if (connectionType === 'socks_proxy') {
    const mode = socksPortMode(configuration, options)

    if (mode !== 'auto' && mode !== 'manual') {
      errors.socksPort = 'SOCKS port mode must be Auto or Manual.'
    } else if (mode === 'manual' && !isValidPort(configuration.socksPort)) {
      errors.socksPort = 'SOCKS port must be Auto or between 1 and 65535.'
    }
  } else {
    errors.connectionType = 'Choose a valid tunnel type.'
  }

  return errors
}

export function normalizeSession(session) {
  if (!session) return null

  const configurationId = firstNonEmpty(session.configurationId, session.ConfigurationID)
  if (!configurationId) return null

  return {
    configurationId,
    status: firstNonEmpty(session.status, session.Status) || 'stopped',
    boundPort: firstDefined(session.boundPort, session.BoundPort, 0),
    statusDetail: firstNonEmpty(session.statusDetail, session.StatusDetail),
    startedAt: firstNonEmpty(session.startedAt, session.StartedAt),
    lastStateChangeAt: firstNonEmpty(session.lastStateChangeAt, session.LastStateChangeAt),
    reconnectAttemptCount: firstDefined(session.reconnectAttemptCount, session.ReconnectAttemptCount, 0),
    lastError: firstNonEmpty(session.lastError, session.LastError),
    needsUserInput: firstDefined(session.needsUserInput, session.NeedsUserInput, false),
  }
}

export function buildRuntimeSessions(sourceSessions = []) {
  if (!Array.isArray(sourceSessions)) return []

  const latestByConfiguration = new Map()

  for (const item of sourceSessions) {
    const session = normalizeSession(item)
    if (!session) continue

    const existing = latestByConfiguration.get(session.configurationId)
    if (!existing || timestampValue(session.lastStateChangeAt) >= timestampValue(existing.lastStateChangeAt)) {
      latestByConfiguration.set(session.configurationId, session)
    }
  }

  return Array.from(latestByConfiguration.values())
}

export function sessionCapabilities(sessionOrStatus) {
  const status = statusValue(sessionOrStatus)

  return {
    canStart: STARTABLE_STATUSES.has(status),
    canStop: STOPPABLE_STATUSES.has(status),
    isActive: ACTIVE_STATUSES.has(status),
  }
}

export function statusLabel(sessionOrStatus) {
  const status = statusValue(sessionOrStatus)
  const labels = {
    starting: 'Starting',
    connected: 'Connected',
    reconnecting: 'Reconnecting',
    needs_attention: 'Needs attention',
    failed: 'Failed',
    stopped: 'Stopped',
  }

  if (labels[status]) return labels[status]

  return status
    .split('_')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ') || 'Stopped'
}

export function statusTone(sessionOrStatus) {
  switch (statusValue(sessionOrStatus)) {
    case 'connected':
      return 'positive'
    case 'reconnecting':
    case 'needs_attention':
      return 'warning'
    case 'failed':
      return 'danger'
    case 'starting':
      return 'info'
    default:
      return 'neutral'
  }
}

export function configurationEndpoint(configuration, session = null) {
  if (!configuration) return ''

  if (configuration.connectionType === 'socks_proxy') {
    const normalizedSession = normalizeSession(session)
    const port = positivePort(normalizedSession?.boundPort) || positivePort(configuration.socksPort)
    return port ? `SOCKS :${port}` : 'SOCKS Auto'
  }

  if (configuration.connectionType === 'local_forward') {
    return `${text(configuration.localPort)} → ${text(configuration.remoteHost)}:${text(configuration.remotePort)}`
  }

  return ''
}

export function selectInitialServerId(servers = [], rememberedServerId = '') {
  if (!Array.isArray(servers) || servers.length === 0) return ''

  const rememberedExists = servers.some((item) => item?.server?.id === rememberedServerId)
  if (rememberedExists) return rememberedServerId

  return servers.find((item) => text(item?.server?.id) !== '')?.server.id || ''
}

export function findConfigurationRecord(servers = [], configurationId = '') {
  if (!Array.isArray(servers) || !configurationId) return null

  for (const item of servers) {
    const configuration = Array.isArray(item?.configurations)
      ? item.configurations.find((entry) => entry?.id === configurationId)
      : null

    if (configuration) {
      return { server: item.server, configuration }
    }
  }

  return null
}

export function activeSessions(sourceSessions = []) {
  return buildRuntimeSessions(sourceSessions).filter((session) => sessionCapabilities(session).isActive)
}

export function normalizeHistoryEntry(entry) {
  if (!entry) return null

  const id = firstNonEmpty(entry.id, entry.ID)
  const configurationId = firstNonEmpty(entry.configurationId, entry.ConfigurationID)
  if (!id || !configurationId) return null

  return {
    id,
    configurationId,
    startedAt: firstNonEmpty(entry.startedAt, entry.StartedAt),
    endedAt: firstNonEmpty(entry.endedAt, entry.EndedAt),
    outcome: firstNonEmpty(entry.outcome, entry.Outcome),
    message: firstNonEmpty(entry.message, entry.Message),
  }
}
