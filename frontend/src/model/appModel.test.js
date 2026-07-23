import { describe, expect, it } from 'vitest'

import {
  activeSessions,
  browserAppearanceForTarget,
  browserAppearanceForeground,
  browserAppearanceKey,
  browserSelectionIndexForDirections,
  buildRuntimeSessions,
  configurationEndpoint,
  emptyServer,
  emptyTunnel,
  findConfigurationRecord,
  initialBrowserSelectionIndex,
  isManagedSOCKSConfiguration,
  managedSOCKSConfigurationID,
  moveBrowserSelectionIndex,
  normalizeHistoryEntry,
  normalizeBrowserAppearance,
  normalizeSession,
  orderBrowsersByRecentActivation,
  recordBrowserTargetActivation,
  selectInitialServerId,
  sessionCapabilities,
  shortcutFromKeyboardEvent,
  statusLabel,
  statusTone,
  validateServer,
  validateBrowserAppearance,
  validateTunnel,
} from './appModel'

describe('shortcutFromKeyboardEvent', () => {
  it('uses the physical key so shifted punctuation stays configurable', () => {
    expect(shortcutFromKeyboardEvent({ code: 'Semicolon', altKey: true, shiftKey: true })).toBe('Alt+Shift+;')
  })

  it('normalizes command and named keys', () => {
    expect(shortcutFromKeyboardEvent({ code: 'ArrowRight', metaKey: true })).toBe('Meta+ArrowRight')
  })

  it('rejects bare keys and modifier-only events', () => {
    expect(shortcutFromKeyboardEvent({ code: 'KeyB' })).toBe('')
    expect(shortcutFromKeyboardEvent({ code: 'AltLeft', altKey: true })).toBe('')
  })
})

describe('browser switcher selection', () => {
  it('starts at the first item going forward and the last item going backward', () => {
    expect(initialBrowserSelectionIndex(3, 'forward')).toBe(0)
    expect(initialBrowserSelectionIndex(3, 'backward')).toBe(2)
  })

  it('moves in either direction and wraps at both ends', () => {
    expect(moveBrowserSelectionIndex(2, 3, 'forward')).toBe(0)
    expect(moveBrowserSelectionIndex(0, 3, 'backward')).toBe(2)
    expect(moveBrowserSelectionIndex(1, 3, 'forward')).toBe(2)
    expect(moveBrowserSelectionIndex(1, 3, 'backward')).toBe(0)
  })

  it('replays directions received while the browser list is loading', () => {
    expect(browserSelectionIndexForDirections(4, ['forward', 'forward', 'backward'])).toBe(0)
    expect(browserSelectionIndexForDirections(4, ['backward', 'backward'])).toBe(2)
  })

  it('applies the first direction relative to the current browser when known', () => {
    expect(browserSelectionIndexForDirections(3, ['forward'], 0)).toBe(1)
    expect(browserSelectionIndexForDirections(3, ['backward'], 0)).toBe(2)
    expect(browserSelectionIndexForDirections(3, ['forward', 'backward'], 0)).toBe(0)
  })

  it('returns zero when no items are available', () => {
    expect(initialBrowserSelectionIndex(0, 'backward')).toBe(0)
    expect(moveBrowserSelectionIndex(5, 0, 'forward')).toBe(0)
    expect(browserSelectionIndexForDirections(0, ['backward', 'backward'])).toBe(0)
  })
})

describe('browser switcher recent activation order', () => {
  const browsers = [
    { id: 'a', browserName: 'A' },
    { id: 'b', browserName: 'B' },
    { id: 'c', browserName: 'C' },
    { id: 'd', browserName: 'D' },
  ]

  it('places recent targets first while retaining backend order for untracked targets', () => {
    expect(orderBrowsersByRecentActivation(browsers, ['c', 'a']).map((item) => item.id)).toEqual(['c', 'a', 'b', 'd'])
    expect(browsers.map((item) => item.id)).toEqual(['a', 'b', 'c', 'd'])
  })

  it('moves a successful activation to the front without duplicates', () => {
    expect(recordBrowserTargetActivation(['b', 'a', 'c'], 'a')).toEqual(['a', 'b', 'c'])
    expect(recordBrowserTargetActivation(['a', 'b'], 'c')).toEqual(['c', 'a', 'b'])
  })

  it('supports forward toggling and reverse selection around the most recent target', () => {
    const history = recordBrowserTargetActivation(['b', 'a'], 'a')
    const ordered = orderBrowsersByRecentActivation(browsers, history)
    const currentIndex = ordered.findIndex((item) => item.id === history[0])

    expect(ordered.map((item) => item.id)).toEqual(['a', 'b', 'c', 'd'])
    expect(ordered[browserSelectionIndexForDirections(ordered.length, ['forward'], currentIndex)].id).toBe('b')
    expect(ordered[browserSelectionIndexForDirections(ordered.length, ['backward'], currentIndex)].id).toBe('d')
  })
})

describe('browser switcher appearance', () => {
  const proxy = { kind: 'proxy', serverId: 'server-a', browserId: 'google-chrome' }
  const regular = { kind: 'regular', browserId: 'google-chrome' }

  it('uses stable semantic keys instead of process ids', () => {
    expect(browserAppearanceKey({ ...proxy, id: 'browser:101' })).toBe('proxy:server-a:google-chrome')
    expect(browserAppearanceKey({ ...proxy, id: 'browser:999' })).toBe('proxy:server-a:google-chrome')
    expect(browserAppearanceKey(regular)).toBe('regular:google-chrome')
    expect(browserAppearanceKey({ kind: 'proxy', browserId: 'google-chrome' })).toBe('')
  })

  it('normalizes and resolves saved colors and marks', () => {
    const appearances = {
      'proxy:server-a:google-chrome': { icon: '  X ', primaryColor: '#22c55e' },
    }
    expect(browserAppearanceForTarget(proxy, appearances)).toEqual({ icon: 'X', primaryColor: '#22C55E' })
    expect(browserAppearanceForTarget(regular, appearances)).toEqual({ icon: '', primaryColor: '' })
    expect(normalizeBrowserAppearance({ icon: ' 🟢 ', primaryColor: '#abcdef' })).toEqual({ icon: '🟢', primaryColor: '#ABCDEF' })
  })

  it('accepts curated icons, short marks, and joined emoji', () => {
    expect(validateBrowserAppearance({ icon: 'icon:x', primaryColor: '#22C55E' })).toEqual({})
    expect(validateBrowserAppearance({ icon: 'X' })).toEqual({})
    expect(validateBrowserAppearance({ icon: '👩🏽‍💻' })).toEqual({})
  })

  it('rejects malformed colors, unknown icons, markup, and long marks', () => {
    expect(validateBrowserAppearance({ primaryColor: 'green' })).toHaveProperty('primaryColor')
    expect(validateBrowserAppearance({ icon: 'icon:unknown' })).toHaveProperty('icon')
    expect(validateBrowserAppearance({ icon: '<b>' })).toHaveProperty('icon')
    expect(validateBrowserAppearance({ icon: 'ABC' })).toHaveProperty('icon')
  })

  it('chooses a readable foreground for light and dark primary colors', () => {
    expect(browserAppearanceForeground('#22C55E')).toBe('#07110B')
    expect(browserAppearanceForeground('#1E3A8A')).toBe('#FFFFFF')
  })
})

describe('emptyServer', () => {
  it('returns the existing server editor shape with the current username', () => {
    expect(emptyServer('eric')).toEqual({
      id: '',
      name: '',
      host: 'localhost',
      port: '22',
      socksPort: '',
      username: 'eric',
      authMode: 'agent',
      keyReference: '',
    })
  })

  it('returns a fresh object each time', () => {
    const first = emptyServer('eric')
    const second = emptyServer('eric')

    first.name = 'Changed'

    expect(second.name).toBe('')
  })
})

describe('emptyTunnel', () => {
  it('returns the existing tunnel editor shape for a server', () => {
    expect(emptyTunnel('server-1')).toEqual({
      serverId: 'server-1',
      label: '',
      connectionType: 'local_forward',
      localPort: '',
      remoteHost: '',
      remotePort: '',
      socksPort: '',
      autoReconnectEnabled: true,
      startOnLaunch: false,
      notes: '',
    })
  })
})

describe('validateServer', () => {
  const validServer = {
    name: 'Staging',
    host: 'staging.example.com',
    port: '22',
    socksPort: '',
    username: 'deploy',
    authMode: 'agent',
    keyReference: '',
  }

  it('accepts a valid SSH-agent server without mutating it', () => {
    const input = { ...validServer }

    expect(validateServer(input)).toEqual({})
    expect(input).toEqual(validServer)
  })

  it('requires identity and endpoint fields', () => {
    expect(validateServer({ port: '22', authMode: 'agent' })).toEqual({
      name: 'A server name is required.',
      host: 'A server host is required.',
      username: 'An SSH username is required.',
    })
  })

  it.each(['', '0', 0, '65536', 65536, '-1', '22.5', 'abc'])('rejects invalid SSH port %j', (port) => {
    expect(validateServer({ ...validServer, port }).port).toBe('Port must be between 1 and 65535.')
  })

  it.each(['1', 1, '65535', 65535, ' 22 '])('accepts valid SSH port %j', (port) => {
    expect(validateServer({ ...validServer, port }).port).toBeUndefined()
  })

  it('allows automatic browser SOCKS assignment or an editable fixed port', () => {
    expect(validateServer({ ...validServer, socksPort: '' }).socksPort).toBeUndefined()
    expect(validateServer({ ...validServer, socksPort: '55123' }).socksPort).toBeUndefined()
    expect(validateServer({ ...validServer, socksPort: '70000' }).socksPort).toBe('SOCKS port must be between 1 and 65535.')
  })

  it('requires a key path only for private-key authentication', () => {
    expect(validateServer({ ...validServer, authMode: 'private_key' })).toEqual({
      keyReference: 'A private key path is required.',
    })
    expect(validateServer({ ...validServer, authMode: 'private_key', keyReference: '~/.ssh/id_ed25519' })).toEqual({})
  })
})

describe('managed server SOCKS configuration', () => {
  it('uses a stable server-derived identity', () => {
    expect(managedSOCKSConfigurationID('server-1')).toBe('server-socks:server-1')
    expect(isManagedSOCKSConfiguration({ id: 'server-socks:server-1' })).toBe(true)
    expect(isManagedSOCKSConfiguration({ id: 'tunnel-1' })).toBe(false)
  })
})

describe('validateTunnel', () => {
  const localForward = {
    label: 'Web app',
    connectionType: 'local_forward',
    localPort: '3000',
    remoteHost: '127.0.0.1',
    remotePort: '3000',
    socksPort: '',
  }

  it('accepts a valid local forward', () => {
    expect(validateTunnel(localForward)).toEqual({})
  })

  it('returns field-specific errors for an empty local forward', () => {
    expect(validateTunnel({ connectionType: 'local_forward' })).toEqual({
      label: 'A label is required.',
      localPort: 'Local port is required.',
      remoteHost: 'Remote host is required.',
      remotePort: 'Remote port is required.',
    })
  })

  it.each(['0', 0, '65536', '-1', '12.5', 'not-a-port'])('rejects invalid local and remote port %j', (port) => {
    expect(validateTunnel({ ...localForward, localPort: port }).localPort).toBe('Local port must be between 1 and 65535.')
    expect(validateTunnel({ ...localForward, remotePort: port }).remotePort).toBe('Remote port must be between 1 and 65535.')
  })

  it.each(['', 'auto', 'AUTO', 0, '0'])('accepts SOCKS automatic port marker %j', (socksPort) => {
    expect(validateTunnel({
      label: 'Proxy',
      connectionType: 'socks_proxy',
      socksPort,
    })).toEqual({})
  })

  it.each(['1', 1, '1080', 1080, '65535'])('accepts valid manual SOCKS port %j', (socksPort) => {
    expect(validateTunnel({
      label: 'Proxy',
      connectionType: 'socks_proxy',
      socksPort,
    })).toEqual({})
  })

  it.each(['-1', '65536', '12.5', 'abc'])('rejects invalid manual SOCKS port %j', (socksPort) => {
    expect(validateTunnel({
      label: 'Proxy',
      connectionType: 'socks_proxy',
      socksPort,
    })).toEqual({
      socksPort: 'SOCKS port must be Auto or between 1 and 65535.',
    })
  })

  it('requires a value when the UI explicitly selects manual SOCKS mode', () => {
    expect(validateTunnel({
      label: 'Proxy',
      connectionType: 'socks_proxy',
      socksPort: '',
    }, { socksPortMode: 'manual' })).toEqual({
      socksPort: 'SOCKS port must be Auto or between 1 and 65535.',
    })
  })

  it('ignores a stale port when the UI explicitly selects automatic SOCKS mode', () => {
    expect(validateTunnel({
      label: 'Proxy',
      connectionType: 'socks_proxy',
      socksPort: 'invalid-stale-value',
    }, { socksPortMode: 'auto' })).toEqual({})
  })

  it('rejects unsupported tunnel types', () => {
    expect(validateTunnel({ label: 'Unknown', connectionType: 'reverse_forward' })).toEqual({
      connectionType: 'Choose a valid tunnel type.',
    })
  })
})

describe('normalizeSession', () => {
  it('normalizes camel-case session data and preserves false and zero values', () => {
    expect(normalizeSession({
      configurationId: 'config-1',
      status: 'connected',
      boundPort: 0,
      statusDetail: 'Connected',
      startedAt: '2026-07-13T10:00:00Z',
      lastStateChangeAt: '2026-07-13T10:01:00Z',
      reconnectAttemptCount: 0,
      lastError: '',
      needsUserInput: false,
    })).toEqual({
      configurationId: 'config-1',
      status: 'connected',
      boundPort: 0,
      statusDetail: 'Connected',
      startedAt: '2026-07-13T10:00:00Z',
      lastStateChangeAt: '2026-07-13T10:01:00Z',
      reconnectAttemptCount: 0,
      lastError: '',
      needsUserInput: false,
    })
  })

  it('normalizes Pascal-case Wails session data', () => {
    expect(normalizeSession({
      ConfigurationID: 'config-2',
      Status: 'needs_attention',
      BoundPort: 1080,
      StatusDetail: 'Unlock required',
      StartedAt: '2026-07-13T10:00:00Z',
      LastStateChangeAt: '2026-07-13T10:02:00Z',
      ReconnectAttemptCount: 2,
      LastError: 'locked',
      NeedsUserInput: true,
    })).toEqual({
      configurationId: 'config-2',
      status: 'needs_attention',
      boundPort: 1080,
      statusDetail: 'Unlock required',
      startedAt: '2026-07-13T10:00:00Z',
      lastStateChangeAt: '2026-07-13T10:02:00Z',
      reconnectAttemptCount: 2,
      lastError: 'locked',
      needsUserInput: true,
    })
  })

  it('rejects missing configuration IDs and defaults omitted state', () => {
    expect(normalizeSession(null)).toBeNull()
    expect(normalizeSession({ status: 'connected' })).toBeNull()
    expect(normalizeSession({ configurationId: 'config-1' })).toEqual({
      configurationId: 'config-1',
      status: 'stopped',
      boundPort: 0,
      statusDetail: '',
      startedAt: '',
      lastStateChangeAt: '',
      reconnectAttemptCount: 0,
      lastError: '',
      needsUserInput: false,
    })
  })
})

describe('buildRuntimeSessions', () => {
  it('keeps the latest normalized session for each configuration', () => {
    const result = buildRuntimeSessions([
      { configurationId: 'config-1', status: 'starting', lastStateChangeAt: '2026-07-13T10:00:00Z' },
      { ConfigurationID: 'config-2', Status: 'connected', LastStateChangeAt: '2026-07-13T10:01:00Z' },
      { configurationId: 'config-1', status: 'connected', lastStateChangeAt: '2026-07-13T10:02:00Z' },
      { configurationId: '', status: 'failed' },
      null,
    ])

    expect(result).toHaveLength(2)
    expect(result.map((session) => [session.configurationId, session.status])).toEqual([
      ['config-1', 'connected'],
      ['config-2', 'connected'],
    ])
  })

  it('lets the later item win when timestamps are equal or unavailable', () => {
    expect(buildRuntimeSessions([
      { configurationId: 'config-1', status: 'starting' },
      { configurationId: 'config-1', status: 'failed' },
    ])[0].status).toBe('failed')
  })

  it('does not let an older session replace a newer session', () => {
    expect(buildRuntimeSessions([
      { configurationId: 'config-1', status: 'connected', lastStateChangeAt: '2026-07-13T10:02:00Z' },
      { configurationId: 'config-1', status: 'starting', lastStateChangeAt: '2026-07-13T10:01:00Z' },
    ])[0].status).toBe('connected')
  })

  it('returns an empty array for non-array input', () => {
    expect(buildRuntimeSessions(null)).toEqual([])
    expect(buildRuntimeSessions({})).toEqual([])
  })
})

describe('session presentation and capabilities', () => {
  it.each([
    ['stopped', 'Stopped', 'neutral', true, false, false],
    ['starting', 'Starting', 'info', false, true, true],
    ['connected', 'Connected', 'positive', false, true, true],
    ['reconnecting', 'Reconnecting', 'warning', false, true, true],
    ['needs_attention', 'Needs attention', 'warning', false, true, true],
    ['failed', 'Failed', 'danger', true, false, false],
  ])('maps %s to its label, tone, and capabilities', (status, label, tone, canStart, canStop, isActive) => {
    expect(statusLabel(status)).toBe(label)
    expect(statusTone({ status })).toBe(tone)
    expect(sessionCapabilities(status)).toEqual({ canStart, canStop, isActive })
  })

  it('treats a missing session as stopped', () => {
    expect(statusLabel(null)).toBe('Stopped')
    expect(statusTone(null)).toBe('neutral')
    expect(sessionCapabilities(null)).toEqual({ canStart: true, canStop: false, isActive: false })
  })

  it('renders an unknown snake-case status readably but does not enable actions', () => {
    expect(statusLabel('waiting_for_network')).toBe('Waiting For Network')
    expect(statusTone('waiting_for_network')).toBe('neutral')
    expect(sessionCapabilities('waiting_for_network')).toEqual({ canStart: false, canStop: false, isActive: false })
  })
})

describe('configurationEndpoint', () => {
  it('formats local forwards', () => {
    expect(configurationEndpoint({
      connectionType: 'local_forward',
      localPort: 3000,
      remoteHost: '127.0.0.1',
      remotePort: 5173,
    })).toBe('3000 → 127.0.0.1:5173')
  })

  it('formats automatic, fixed, and runtime-assigned SOCKS ports', () => {
    const automatic = { connectionType: 'socks_proxy', socksPort: 0 }
    const fixed = { connectionType: 'socks_proxy', socksPort: 1080 }

    expect(configurationEndpoint(automatic)).toBe('SOCKS Auto')
    expect(configurationEndpoint(fixed)).toBe('SOCKS :1080')
    expect(configurationEndpoint(automatic, { ConfigurationID: 'config-1', BoundPort: 43123 })).toBe('SOCKS :43123')
  })

  it('returns an empty label for missing or unknown configurations', () => {
    expect(configurationEndpoint(null)).toBe('')
    expect(configurationEndpoint({ connectionType: 'unknown' })).toBe('')
  })
})

describe('selectInitialServerId', () => {
  const servers = [
    { server: { id: 'server-1', name: 'First' }, configurations: [] },
    { server: { id: 'server-2', name: 'Second' }, configurations: [] },
  ]

  it('uses the remembered server when it still exists', () => {
    expect(selectInitialServerId(servers, 'server-2')).toBe('server-2')
  })

  it('falls back to the first valid server', () => {
    expect(selectInitialServerId(servers, 'deleted-server')).toBe('server-1')
    expect(selectInitialServerId([{ server: null }, ...servers], '')).toBe('server-1')
  })

  it('returns an empty ID when no server is available', () => {
    expect(selectInitialServerId([], 'server-1')).toBe('')
    expect(selectInitialServerId(null, 'server-1')).toBe('')
  })
})

describe('findConfigurationRecord', () => {
  const configuration = { id: 'config-2', label: 'Database' }
  const server = { id: 'server-2', name: 'Production' }
  const servers = [
    { server: { id: 'server-1' }, configurations: [] },
    { server, configurations: [{ id: 'config-1' }, configuration] },
  ]

  it('returns the owning server and configuration', () => {
    expect(findConfigurationRecord(servers, 'config-2')).toEqual({ server, configuration })
  })

  it('returns null for missing or malformed data', () => {
    expect(findConfigurationRecord(servers, 'missing')).toBeNull()
    expect(findConfigurationRecord([{ server, configurations: null }], 'config-2')).toBeNull()
    expect(findConfigurationRecord(null, 'config-2')).toBeNull()
  })
})

describe('activeSessions', () => {
  it('returns only the latest active and attention-requiring sessions', () => {
    const result = activeSessions([
      { configurationId: 'starting', status: 'starting' },
      { configurationId: 'connected', status: 'connected' },
      { configurationId: 'reconnecting', status: 'reconnecting' },
      { configurationId: 'attention', status: 'needs_attention' },
      { configurationId: 'stopped', status: 'stopped' },
      { configurationId: 'failed', status: 'failed' },
      { configurationId: 'changed', status: 'connected', lastStateChangeAt: '2026-07-13T10:00:00Z' },
      { configurationId: 'changed', status: 'stopped', lastStateChangeAt: '2026-07-13T10:01:00Z' },
    ])

    expect(result.map((session) => session.configurationId)).toEqual([
      'starting',
      'connected',
      'reconnecting',
      'attention',
    ])
  })
})

describe('normalizeHistoryEntry', () => {
  it('normalizes camel-case history data', () => {
    expect(normalizeHistoryEntry({
      id: 'history-1',
      configurationId: 'config-1',
      startedAt: '2026-07-13T10:00:00Z',
      endedAt: '2026-07-13T10:01:00Z',
      outcome: 'connected',
      message: 'Tunnel connected',
    })).toEqual({
      id: 'history-1',
      configurationId: 'config-1',
      startedAt: '2026-07-13T10:00:00Z',
      endedAt: '2026-07-13T10:01:00Z',
      outcome: 'connected',
      message: 'Tunnel connected',
    })
  })

  it('normalizes Pascal-case Wails history data and defaults optional fields', () => {
    expect(normalizeHistoryEntry({
      ID: 'history-2',
      ConfigurationID: 'config-2',
      Outcome: 'failed_runtime',
      Message: 'Connection failed',
    })).toEqual({
      id: 'history-2',
      configurationId: 'config-2',
      startedAt: '',
      endedAt: '',
      outcome: 'failed_runtime',
      message: 'Connection failed',
    })
  })

  it('rejects entries without both IDs', () => {
    expect(normalizeHistoryEntry(null)).toBeNull()
    expect(normalizeHistoryEntry({ id: 'history-1' })).toBeNull()
    expect(normalizeHistoryEntry({ configurationId: 'config-1' })).toBeNull()
  })
})
