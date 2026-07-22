const ACTIVE_STATUSES = new Set(['starting', 'connected', 'reconnecting', 'needs_attention'])
const STARTABLE_STATUSES = new Set(['stopped', 'failed'])
const STOPPABLE_STATUSES = new Set(['starting', 'connected', 'reconnecting', 'needs_attention'])
const BROWSER_APPEARANCE_ICON_TOKENS = new Set([
  'icon:x',
  'icon:shield',
  'icon:terminal',
  'icon:globe',
  'icon:network',
  'icon:star',
  'icon:briefcase',
  'icon:code',
])
const BROWSER_APPEARANCE_COLOR_PATTERN = /^#[0-9A-F]{6}$/

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

const shortcutCodeKeys = {
  Semicolon: ';',
  Comma: ',',
  Period: '.',
  Slash: '/',
  Backslash: '\\',
  Quote: "'",
  BracketLeft: '[',
  BracketRight: ']',
  Minus: '-',
  Equal: '=',
  Backquote: '`',
  Space: 'Space',
  Tab: 'Tab',
  Enter: 'Enter',
  Escape: 'Escape',
  Backspace: 'Backspace',
  Delete: 'Delete',
  ArrowUp: 'ArrowUp',
  ArrowDown: 'ArrowDown',
  ArrowLeft: 'ArrowLeft',
  ArrowRight: 'ArrowRight',
}

export function shortcutFromKeyboardEvent(event = {}) {
  const code = String(event.code || '')
  let key = shortcutCodeKeys[code] || ''
  if (/^Key[A-Z]$/.test(code)) key = code.slice(3)
  if (/^Digit[0-9]$/.test(code)) key = code.slice(5)
  if (/^F(?:[1-9]|1[0-9]|20)$/.test(code)) key = code
  if (!key || (!event.ctrlKey && !event.altKey && !event.metaKey)) return ''

  return [
    event.ctrlKey ? 'Ctrl' : '',
    event.altKey ? 'Alt' : '',
    event.shiftKey ? 'Shift' : '',
    event.metaKey ? 'Meta' : '',
    key,
  ].filter(Boolean).join('+')
}

export function normalizeBrowserSwitchDirection(direction) {
  return direction === 'backward' ? 'backward' : 'forward'
}

export function browserAppearanceKey(target = {}) {
  const browserId = text(target.browserId)
  if (!browserId) return ''
  if (target.kind === 'proxy') {
    const serverId = text(target.serverId)
    return serverId ? `proxy:${serverId}:${browserId}` : ''
  }
  return `regular:${browserId}`
}

function browserIconGraphemes(value) {
  if (typeof Intl !== 'undefined' && typeof Intl.Segmenter === 'function') {
    const segmenter = new Intl.Segmenter(undefined, { granularity: 'grapheme' })
    return Array.from(segmenter.segment(value), ({ segment }) => segment)
  }
  return Array.from(value)
}

export function normalizeBrowserAppearance(appearance = {}) {
  const icon = text(appearance.icon)
  const primaryColor = text(appearance.primaryColor).toUpperCase()
  return { icon, primaryColor }
}

export function validateBrowserAppearance(appearance = {}) {
  const normalized = normalizeBrowserAppearance(appearance)
  const errors = {}

  if (normalized.primaryColor && !BROWSER_APPEARANCE_COLOR_PATTERN.test(normalized.primaryColor)) {
    errors.primaryColor = 'Choose a valid six-digit color.'
  }
  if (normalized.icon) {
    if (normalized.icon.startsWith('icon:') && !BROWSER_APPEARANCE_ICON_TOKENS.has(normalized.icon)) {
      errors.icon = 'Choose one of the available icons.'
    } else if (!BROWSER_APPEARANCE_ICON_TOKENS.has(normalized.icon)) {
      if (/[<>\u0000-\u001F\u007F]/u.test(normalized.icon)) {
        errors.icon = 'Use a visible emoji or short mark.'
      } else if (browserIconGraphemes(normalized.icon).length > 2) {
        errors.icon = 'Use one emoji or up to two characters.'
      }
    }
  }

  return errors
}

export function browserAppearanceForTarget(target = {}, appearances = {}) {
  const key = browserAppearanceKey(target)
  if (!key || !appearances || typeof appearances !== 'object') return { icon: '', primaryColor: '' }
  return normalizeBrowserAppearance(appearances[key])
}

export function browserAppearanceForeground(primaryColor = '') {
  const color = text(primaryColor).toUpperCase()
  if (!BROWSER_APPEARANCE_COLOR_PATTERN.test(color)) return '#FFFFFF'
  const red = Number.parseInt(color.slice(1, 3), 16)
  const green = Number.parseInt(color.slice(3, 5), 16)
  const blue = Number.parseInt(color.slice(5, 7), 16)
  const luminance = (0.2126 * red + 0.7152 * green + 0.0722 * blue) / 255
  return luminance > 0.56 ? '#07110B' : '#FFFFFF'
}

export function initialBrowserSelectionIndex(itemCount, direction = 'forward') {
  const count = Number.isInteger(itemCount) && itemCount > 0 ? itemCount : 0
  if (!count) return 0
  return normalizeBrowserSwitchDirection(direction) === 'backward' ? count - 1 : 0
}

export function moveBrowserSelectionIndex(selectedIndex, itemCount, direction = 'forward') {
  const count = Number.isInteger(itemCount) && itemCount > 0 ? itemCount : 0
  if (!count) return 0

  const index = Number.isInteger(selectedIndex)
    ? ((selectedIndex % count) + count) % count
    : 0
  const step = normalizeBrowserSwitchDirection(direction) === 'backward' ? -1 : 1
  return (index + step + count) % count
}

export function browserSelectionIndexForDirections(itemCount, directions = [], currentIndex = -1) {
  if (!directions.length) return initialBrowserSelectionIndex(itemCount)

  let index = Number.isInteger(currentIndex) && currentIndex >= 0 && currentIndex < itemCount
    ? moveBrowserSelectionIndex(currentIndex, itemCount, directions[0])
    : initialBrowserSelectionIndex(itemCount, directions[0])
  for (const direction of directions.slice(1)) {
    index = moveBrowserSelectionIndex(index, itemCount, direction)
  }
  return index
}

export function recordBrowserTargetActivation(history = [], targetId = '') {
  const id = String(targetId || '')
  if (!id) return [...history]
  return [id, ...history.filter((item) => item !== id)]
}

export function orderBrowsersByRecentActivation(items = [], history = []) {
  const rank = new Map(history.map((id, index) => [id, index]))
  return items
    .map((item, index) => ({ item, index, rank: rank.get(item.id) }))
    .sort((left, right) => {
      const leftTracked = left.rank !== undefined
      const rightTracked = right.rank !== undefined
      if (leftTracked && rightTracked) return left.rank - right.rank
      if (leftTracked) return -1
      if (rightTracked) return 1
      return left.index - right.index
    })
    .map(({ item }) => item)
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
