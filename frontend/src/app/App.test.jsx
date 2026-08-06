import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'
import App from './App'

const savedServer = {
  id: 'server-1',
  name: 'Production bastion',
  host: 'bastion.example.com',
  port: 22,
  socksPort: 55123,
  username: 'deploy',
  authMode: 'agent',
  keyReference: '',
}

const managedBrowserProxy = {
  id: `server-socks:${savedServer.id}`,
  serverId: savedServer.id,
  label: 'Browser proxy',
  connectionType: 'socks_proxy',
  localPort: 0,
  remoteHost: '',
  remotePort: 0,
  socksPort: savedServer.socksPort,
  autoReconnectEnabled: true,
  startOnLaunch: false,
  notes: '',
}

const savedTunnel = {
  id: 'tunnel-1',
  serverId: savedServer.id,
  label: 'Admin database',
  connectionType: 'local_forward',
  localPort: 15432,
  remoteHost: '127.0.0.1',
  remotePort: 5432,
  socksPort: 0,
  autoReconnectEnabled: true,
  startOnLaunch: false,
  notes: 'Use for production maintenance.',
}

const secondTunnel = {
  ...savedTunnel,
  id: 'tunnel-2',
  label: 'Internal dashboard',
  localPort: 18080,
  remotePort: 8080,
  notes: '',
}

const stoppedSession = {
  configurationId: savedTunnel.id,
  status: 'stopped',
  boundPort: 0,
  statusDetail: 'Stopped by user',
  lastStateChangeAt: '2026-07-13T15:00:00.000Z',
}

const historyEntry = {
  id: 'history-1',
  configurationId: savedTunnel.id,
  startedAt: '2026-07-13T14:58:00.000Z',
  endedAt: '2026-07-13T15:00:00.000Z',
  outcome: 'stopped',
  message: 'Tunnel stopped cleanly.',
}

function clone(value) {
  return JSON.parse(JSON.stringify(value))
}

function createFakeApi({
  servers = [],
  sessions = [],
  history = [],
  sshKeys = [],
  currentUsername = 'eric',
  runningBrowsers = [],
  version = 'Dev build',
  automaticUpdatesSupported = true,
  updateStatus = { state: 'idle', channel: 'stable' },
} = {}) {
  const state = {
    servers: clone(servers),
    sessions: clone(sessions),
    history: clone(history),
    preferences: {
      theme: 'dark',
      lastSelectedServerId: '',
      automaticUpdates: true,
      useExperimentalChannel: false,
      browserSwitcherShortcut: 'Alt+X',
      browserSwitcherBackwardShortcut: 'Alt+Z',
      browserAppearances: {},
      defaultBrowserId: '',
      proxyBrowserId: '',
      disabledBrowserIds: [],
      customBrowsers: [],
      urlRules: [],
      urlPortAssignments: [],
    },
    nextServerId: 2,
    nextTunnelId: 2,
  }

  const api = {
    loadInitialState: vi.fn(async () => ({
      servers: clone(state.servers),
      sessions: clone(state.sessions),
      sshKeys: clone(sshKeys),
      preferences: clone(state.preferences),
      diagnostics: {
        appDataPath: '/tmp/ssh-man',
        databasePath: '/tmp/ssh-man/ssh-man.db',
        version,
        automaticUpdatesSupported,
      },
      currentUsername,
      updateStatus: clone(updateStatus),
      recoverable: false,
      message: '',
    })),
    saveServer: vi.fn(async (server) => {
      const next = { ...server, id: server.id || `server-${state.nextServerId++}` }
      const existing = state.servers.find((item) => item.server.id === next.id)
      state.servers = existing
        ? state.servers.map((item) => item.server.id === next.id ? { ...item, server: next } : item)
        : state.servers.concat({ server: next, configurations: [] })
      return clone(next)
    }),
    deleteServer: vi.fn(async (serverId) => {
      state.servers = state.servers.filter((item) => item.server.id !== serverId)
    }),
    saveConnectionConfiguration: vi.fn(async (configuration) => {
      const next = { ...configuration, id: configuration.id || `tunnel-${state.nextTunnelId++}` }
      state.servers = state.servers.map((item) => item.server.id !== next.serverId
        ? item
        : {
            ...item,
            configurations: item.configurations.filter((entry) => entry.id !== next.id).concat(next),
          })
      return clone(next)
    }),
    deleteConnectionConfiguration: vi.fn(async (configurationId) => {
      state.servers = state.servers.map((item) => ({
        ...item,
        configurations: item.configurations.filter((entry) => entry.id !== configurationId),
      }))
      state.sessions = state.sessions.filter((session) => session.configurationId !== configurationId)
      state.history = state.history.filter((entry) => entry.configurationId !== configurationId)
    }),
    savePreferences: vi.fn(async (preferences) => {
      state.preferences = { ...state.preferences, ...preferences }
      return clone(state.preferences)
    }),
    saveBrowserAppearance: vi.fn(async (appearanceKey, appearance) => {
      const browserAppearances = { ...(state.preferences.browserAppearances || {}) }
      if (!appearance.icon && !appearance.primaryColor) delete browserAppearances[appearanceKey]
      else browserAppearances[appearanceKey] = appearance
      state.preferences = { ...state.preferences, browserAppearances }
      return clone(state.preferences)
    }),
    listRuntimeSessions: vi.fn(async () => clone(state.sessions)),
    listSessionHistory: vi.fn(async (configurationId) => clone(
      state.history.filter((entry) => entry.configurationId === configurationId),
    )),
    startConfiguration: vi.fn(async (configurationId) => {
      const current = state.sessions.find((session) => session.configurationId === configurationId)
      if (current && !['stopped', 'failed'].includes(current.status)) return clone(current)
      const session = {
        configurationId,
        status: 'connected',
        boundPort: savedTunnel.localPort,
        statusDetail: 'Tunnel connected.',
        lastStateChangeAt: '2026-07-13T15:01:00.000Z',
      }
      state.sessions = state.sessions.filter((item) => item.configurationId !== configurationId).concat(session)
      return clone(session)
    }),
    stopConfiguration: vi.fn(async (configurationId) => {
      const session = { ...stoppedSession, configurationId }
      state.sessions = state.sessions.filter((item) => item.configurationId !== configurationId).concat(session)
      return clone(session)
    }),
    retryConfiguration: vi.fn(async (configurationId) => api.startConfiguration(configurationId)),
    startServerConfigurations: vi.fn(async (serverId) => {
      const record = state.servers.find((item) => item.server.id === serverId)
      const inactive = (record?.configurations || []).filter((configuration) => {
        const current = state.sessions.find((session) => session.configurationId === configuration.id)
        return !current || ['stopped', 'failed'].includes(current.status)
      })
      return Promise.all(inactive.map((item) => api.startConfiguration(item.id)))
    }),
    submitKeyUnlock: vi.fn(async (configurationId) => api.startConfiguration(configurationId)),
    discoverBrowsers: vi.fn(async () => []),
    discoverBrowserCatalog: vi.fn(async () => api.discoverBrowsers()),
    chooseBrowserApplication: vi.fn(async () => '/Applications/Kagi Browser.app'),
    previewBrowserLaunchThroughSocks: vi.fn(async () => ({ command: '' })),
    launchBrowserThroughSocks: vi.fn(async () => ({ success: true })),
    listRunningBrowsers: vi.fn(async () => clone(runningBrowsers)),
    activateRunningBrowser: vi.fn(async () => undefined),
    defaultBrowserStatus: vi.fn(async () => ({ supported: true, isDefault: false })),
    setAsDefaultBrowser: vi.fn(async () => ({ supported: true, isDefault: true })),
    validateURLRulePattern: vi.fn(async () => ({ valid: true, message: '' })),
    pendingURLRoute: vi.fn(async () => null),
    resolveURLRoute: vi.fn(async () => undefined),
    dismissURLRoute: vi.fn(async () => undefined),
    setURLRouteWindowMode: vi.fn(),
    onURLRouteChoiceRequested: vi.fn((callback) => {
      api.urlRouteChoiceListener = callback
      return () => { api.urlRouteChoiceListener = null }
    }),
    onPreferencesChanged: vi.fn((callback) => {
      api.preferencesChangedListener = callback
      return () => { api.preferencesChangedListener = null }
    }),
    onUpdateStatusChanged: vi.fn((callback) => {
      api.updateStatusListener = callback
      return () => { api.updateStatusListener = null }
    }),
    onBrowserSwitcherRequested: vi.fn((callback) => {
      api.browserSwitcherListener = callback
      return () => { api.browserSwitcherListener = null }
    }),
    onBrowserSwitcherCommitRequested: vi.fn((callback) => {
      api.browserSwitcherCommitListener = callback
      return () => { api.browserSwitcherCommitListener = null }
    }),
    onBrowserSwitcherCancelRequested: vi.fn((callback) => {
      api.browserSwitcherCancelListener = callback
      return () => { api.browserSwitcherCancelListener = null }
    }),
    showBrowserSwitcherWindow: vi.fn(async () => undefined),
    openDevTools: vi.fn(async () => undefined),
    openHostWindow: vi.fn(async () => undefined),
    openServerExplorer: vi.fn(async () => undefined),
    openSettingsWindow: vi.fn(async () => undefined),
    allowSettingsWindowClose: vi.fn(async () => undefined),
    onSettingsCloseRequested: vi.fn((callback) => {
      api.settingsCloseListener = callback
      return () => { api.settingsCloseListener = null }
    }),
    openServerCommand: vi.fn(async () => undefined),
    hideApplicationWindow: vi.fn(async () => undefined),
    quitApplication: vi.fn(async () => undefined),
    installApplicationUpdate: vi.fn(async () => undefined),
    openExternalURL: vi.fn(async () => undefined),
  }

  return { api, state }
}

function renderApp(api, controllerOptions = {}) {
  return render(<App api={api} controllerOptions={{ pollMs: 60_000, copyText: vi.fn(), ...controllerOptions }} />)
}

function renderSettingsApp(api, controllerOptions = {}) {
  return render(<App api={api} settingsWindow controllerOptions={{ pollMs: 60_000, copyText: vi.fn(), ...controllerOptions }} />)
}

async function openSavedTunnel(user, api) {
  renderApp(api)
  await user.click(await screen.findByRole('button', { name: 'Show Production bastion details' }))
  const tunnelLabel = await screen.findByText('Admin database')
  await user.click(tunnelLabel.closest('button'))
  await screen.findByRole('button', { name: 'Start tunnel' })
}

describe('React application flows', () => {
  test('opens host details in a separate window from the server row', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [] }],
    })
    renderApp(api)

    await user.click(await screen.findByRole('button', {
      name: 'Open Production bastion host details',
    }))

    await waitFor(() => expect(api.openHostWindow).toHaveBeenCalledWith(savedServer.id))
    expect(await screen.findByText('Host details opened in a new window.')).toBeTruthy()
  })

  test('validates and saves the first server through the full-screen flow', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({ currentUsername: '' })
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Add server' }))

    const name = screen.getByLabelText('Name')
    const host = screen.getByLabelText('Host')
    const username = screen.getByLabelText('Username')
    const port = screen.getByLabelText('Port')

    await user.clear(host)
    await user.clear(port)
    await user.type(port, '70000')
    await user.click(screen.getByRole('button', { name: 'Save server' }))

    expect(screen.getByText('A server name is required.')).toBeTruthy()
    expect(screen.getByText('A server host is required.')).toBeTruthy()
    expect(screen.getByText('An SSH username is required.')).toBeTruthy()
    expect(screen.getByText('Port must be between 1 and 65535.')).toBeTruthy()
    expect(name.getAttribute('aria-invalid')).toBe('true')
    expect(name.getAttribute('aria-describedby')).toBe('server-name-error')
    await waitFor(() => expect(document.activeElement).toBe(name))
    expect(api.saveServer).toHaveBeenCalledTimes(0)

    await user.type(name, 'Staging bastion')
    await user.type(host, 'staging.example.com')
    await user.type(username, 'deploy')
    await user.clear(port)
    await user.type(port, '2222')
    await user.click(screen.getByRole('button', { name: 'Save server' }))

    await screen.findByRole('heading', { name: 'Staging bastion', level: 1 })
    expect(api.saveServer).toHaveBeenCalledTimes(1)
    expect(api.saveServer.mock.calls[0][0]).toMatchObject({
      name: 'Staging bastion',
      host: 'staging.example.com',
      username: 'deploy',
      port: 2222,
      socksPort: 0,
    })
    expect(screen.getByText('deploy@staging.example.com:2222')).toBeTruthy()
  })

  test('edits the saved browser SOCKS port as part of the server', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [managedBrowserProxy] }],
    })
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Show Production bastion details' }))
    await user.click(screen.getByRole('button', { name: 'Edit Production bastion' }))
    const socksPort = screen.getByLabelText(/^Browser SOCKS port/)
    expect(socksPort.value).toBe('55123')
    await user.clear(socksPort)
    await user.type(socksPort, '61234')
    await user.click(screen.getByRole('button', { name: 'Save server' }))

    await waitFor(() => expect(api.saveServer).toHaveBeenCalledWith(expect.objectContaining({
      id: savedServer.id,
      socksPort: 61234,
    })))
  })

  test('offers discovered SSH keys and a custom path option', async () => {
    const user = userEvent.setup()
    const discoveredKey = { name: 'id_ed25519', path: '/Users/eric/.ssh/id_ed25519' }
    const { api } = createFakeApi({ sshKeys: [discoveredKey] })
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Add server' }))
    await user.click(screen.getByRole('radio', { name: /Private key/ }))

    const keySelect = screen.getByRole('combobox')
    expect(screen.getByRole('option', { name: 'id_ed25519' })).toBeTruthy()
    await user.selectOptions(keySelect, discoveredKey.path)
    expect(keySelect.value).toBe(discoveredKey.path)

    await user.selectOptions(keySelect, '__custom__')
    expect(screen.getByPlaceholderText('/Users/you/.ssh/id_ed25519')).toBeTruthy()
  })

  test('validates and saves a tunnel beneath its selected server', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({ servers: [{ server: savedServer, configurations: [] }] })
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Show Production bastion details' }))
    await user.click(screen.getAllByRole('button', { name: 'Add tunnel' })[0])
    const label = screen.getByLabelText('Label')
    await user.click(screen.getByRole('button', { name: 'Save tunnel' }))

    expect(screen.getByText('A label is required.')).toBeTruthy()
    expect(screen.getByText('Local port is required.')).toBeTruthy()
    expect(screen.getByText('Remote host is required.')).toBeTruthy()
    expect(screen.getByText('Remote port is required.')).toBeTruthy()
    await waitFor(() => expect(document.activeElement).toBe(label))
    expect(api.saveConnectionConfiguration).toHaveBeenCalledTimes(0)

    await user.type(label, 'Redis admin')
    await user.type(screen.getByLabelText(/^Local port/), '16379')
    await user.type(screen.getByLabelText(/^Remote host/), '127.0.0.1')
    await user.type(screen.getByLabelText(/^Remote port/), '6379')
    await user.type(screen.getByLabelText(/^Notes/), 'Temporary production access')
    await user.click(screen.getByRole('checkbox', { name: /Connect when SSH Man starts/ }))
    await user.click(screen.getByRole('button', { name: 'Save tunnel' }))

    await screen.findByRole('heading', { name: 'Redis admin', level: 1 })
    expect(api.saveConnectionConfiguration).toHaveBeenCalledTimes(1)
    expect(api.saveConnectionConfiguration.mock.calls[0][0]).toMatchObject({
      serverId: savedServer.id,
      label: 'Redis admin',
      localPort: 16379,
      remoteHost: '127.0.0.1',
      remotePort: 6379,
      startOnLaunch: true,
      notes: 'Temporary production access',
    })
    expect(screen.getByRole('button', { name: 'Start tunnel' })).toBeTruthy()
    expect(screen.getByText('16379 → 127.0.0.1:6379')).toBeTruthy()
  })

  test('offers independent browser, explorer, and command launchers on each server card', async () => {
    const user = userEvent.setup()
    const managedStopped = {
      ...stoppedSession,
      configurationId: managedBrowserProxy.id,
    }
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [managedBrowserProxy] }],
      sessions: [managedStopped],
    })
    renderApp(api)

    const browserLauncher = await screen.findByRole('button', { name: 'Open browser through Production bastion' })
    const explorerLauncher = screen.getByRole('button', { name: 'Open Production bastion explorer' })
    const commandLauncher = screen.getByRole('button', { name: 'Open Production bastion quick command' })
    await user.click(browserLauncher)

    await waitFor(() => expect(api.startConfiguration).toHaveBeenCalledWith(managedBrowserProxy.id))
    await waitFor(() => expect(api.launchBrowserThroughSocks).toHaveBeenCalledWith(managedBrowserProxy.id, ''))
    expect(api.openServerExplorer).not.toHaveBeenCalled()
    expect(await screen.findByText('Browser is open through Production bastion.')).toBeTruthy()

    await user.click(explorerLauncher)
    await waitFor(() => expect(api.openServerExplorer).toHaveBeenCalledWith(savedServer.id))
    expect(api.launchBrowserThroughSocks).toHaveBeenCalledTimes(1)

    await user.click(commandLauncher)
    await waitFor(() => expect(api.openServerCommand).toHaveBeenCalledWith(savedServer.id))
    expect(api.openServerExplorer).toHaveBeenCalledTimes(1)
  })

  test('resumes the row browser launcher after unlocking an encrypted key', async () => {
    const user = userEvent.setup()
    const managedStopped = {
      ...stoppedSession,
      configurationId: managedBrowserProxy.id,
    }
    const needsAttention = {
      ...managedStopped,
      status: 'needs_attention',
      statusDetail: 'Unlock the SSH key to continue.',
      needsUserInput: true,
    }
    const connected = {
      ...managedStopped,
      status: 'connected',
      boundPort: managedBrowserProxy.socksPort,
      statusDetail: `Listening on localhost:${managedBrowserProxy.socksPort}`,
    }
    const { api, state } = createFakeApi({
      servers: [{ server: savedServer, configurations: [managedBrowserProxy] }],
      sessions: [managedStopped],
    })
    api.startConfiguration.mockImplementationOnce(async () => {
      state.sessions = [needsAttention]
      return clone(needsAttention)
    })
    api.submitKeyUnlock.mockImplementationOnce(async () => {
      state.sessions = [connected]
      return clone(connected)
    })
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Open browser through Production bastion' }))
    expect(await screen.findByRole('dialog', { name: 'Unlock SSH key' })).toBeTruthy()
    expect(api.launchBrowserThroughSocks).not.toHaveBeenCalled()
    expect(api.openServerExplorer).not.toHaveBeenCalled()

    await user.type(screen.getByLabelText('Passphrase'), 'secret')
    await user.click(screen.getByRole('button', { name: 'Unlock and connect' }))

    await waitFor(() => expect(api.submitKeyUnlock).toHaveBeenCalledWith(managedBrowserProxy.id, 'secret'))
    await waitFor(() => expect(api.launchBrowserThroughSocks).toHaveBeenCalledWith(managedBrowserProxy.id, ''))
    expect(api.openServerExplorer).not.toHaveBeenCalled()
  })

  test('uses the configured Zen browser from a server quick launcher', async () => {
    const user = userEvent.setup()
    const connected = {
      ...stoppedSession,
      configurationId: managedBrowserProxy.id,
      status: 'connected',
      boundPort: managedBrowserProxy.socksPort,
    }
    const { api, state } = createFakeApi({
      servers: [{ server: savedServer, configurations: [managedBrowserProxy] }],
      sessions: [connected],
    })
    state.preferences.proxyBrowserId = 'zen'
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Open browser through Production bastion' }))
    await waitFor(() => expect(api.launchBrowserThroughSocks).toHaveBeenCalledWith(managedBrowserProxy.id, 'zen'))
  })

  test('keeps the MoonPixels link in the persistent footer instead of Settings content', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    renderApp(api)

    const moonPixelsLink = await screen.findByRole('button', { name: 'Visit MoonPixels' })
    await user.click(moonPixelsLink)

    expect(api.openExternalURL).toHaveBeenCalledWith('https://moonpixels.tech')

    await user.click(screen.getByRole('button', { name: 'Settings' }))
    await waitFor(() => expect(api.openSettingsWindow).toHaveBeenCalledTimes(1))
    expect(screen.getByRole('button', { name: 'Visit MoonPixels' })).toBeTruthy()
    expect(screen.queryByRole('heading', { name: 'Gifted with love by MoonPixels' })).toBeNull()
    expect(screen.queryByLabelText('Settings')).toBeNull()
  })

  test('renders settings as a dedicated window without compact navigation', async () => {
    const { api } = createFakeApi()
    render(<App api={api} settingsWindow controllerOptions={{ pollMs: 60_000, copyText: vi.fn() }} />)

    expect(await screen.findByRole('heading', { name: 'Settings', level: 1 })).toBeTruthy()
    expect(screen.getByLabelText('Settings')).toBeTruthy()
    expect(screen.queryByRole('navigation', { name: 'Main navigation' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Close Settings' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Quit SSH Man' })).toBeNull()
  })

  test('shows the backend-provided app version', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({ version: '1.2.3' })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'App health' }))
    expect(await screen.findByText('App version')).toBeTruthy()
    expect(screen.getAllByText('1.2.3')).not.toHaveLength(0)
  })

  test('lets users turn off automatic updates', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    renderSettingsApp(api)

    const toggle = await screen.findByRole('checkbox', { name: 'Install updates automatically' })
    expect(toggle.checked).toBe(true)

    await user.click(toggle)

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      automaticUpdates: false,
    })))
    expect(toggle.checked).toBe(false)
  })

  test('lets users select the experimental update channel', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    renderSettingsApp(api)

    const toggle = await screen.findByRole('checkbox', { name: 'Use experimental channel' })
    expect(toggle.checked).toBe(false)

    await user.click(toggle)

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      useExperimentalChannel: true,
    })))
    expect(toggle.checked).toBe(true)
  })

  test('shows a ready update banner and installs with one click', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      updateStatus: { state: 'ready', version: '1.15.0', channel: 'experimental' },
    })
    renderApp(api)

    expect(await screen.findByText('SSH Man 1.15.0 is ready')).toBeTruthy()
    expect(screen.getByText('Experimental update')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Update now' }))

    expect(api.installApplicationUpdate).toHaveBeenCalledTimes(1)
  })

  test('updates the banner when the backend publishes update status', async () => {
    const { api } = createFakeApi()
    renderApp(api)

    await waitFor(() => expect(api.updateStatusListener).toBeTypeOf('function'))
    expect(screen.queryByText('SSH Man 1.15.0 is ready')).toBeNull()

    act(() => api.updateStatusListener({
      state: 'ready',
      version: '1.15.0',
      channel: 'stable',
    }))

    expect(await screen.findByText('SSH Man 1.15.0 is ready')).toBeTruthy()
  })

  test('hides automatic updates when the platform does not support them', async () => {
    const { api } = createFakeApi({ automaticUpdatesSupported: false })
    renderSettingsApp(api)

    await screen.findByRole('heading', { name: 'General' })
    expect(screen.queryByRole('checkbox', { name: 'Install updates automatically' })).toBeNull()
  })

  test('keeps a stopped tunnel actionable with settings and history, then exposes it in Active after starting', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [savedTunnel] }],
      sessions: [stoppedSession],
      history: [historyEntry],
    })
    await openSavedTunnel(user, api)

    expect(screen.getByLabelText('Tunnel status: Stopped')).toBeTruthy()
    expect(screen.getByText('15432 → 127.0.0.1:5432')).toBeTruthy()
    expect(screen.getByText('Automatic')).toBeTruthy()
    expect(screen.getByText('Use for production maintenance.')).toBeTruthy()
    expect(await screen.findByText('Tunnel stopped cleanly.')).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Recent history' })).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Start tunnel' }))
    await screen.findByRole('button', { name: 'Stop tunnel' })
    expect(screen.getByLabelText('Tunnel status: Connected')).toBeTruthy()
    expect(api.startConfiguration).toHaveBeenCalledWith(savedTunnel.id)

    await user.click(screen.getByRole('button', { name: 'Go back' }))
    expect(screen.queryByRole('button', { name: /Start .*inactive tunnel/ })).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Go back' }))
    await user.click(screen.getByRole('button', { name: /Active/ }))

    expect(await screen.findByRole('heading', { name: '1 active tunnel' })).toBeTruthy()
    expect(screen.getByText('Admin database').closest('button')).toBeTruthy()
    expect(screen.getByLabelText('Tunnel status: Connected')).toBeTruthy()
  })

  test('bulk start only starts inactive tunnels and leaves connected tunnels alone', async () => {
    const user = userEvent.setup()
    const connected = {
      ...stoppedSession,
      configurationId: savedTunnel.id,
      status: 'connected',
      statusDetail: 'Tunnel connected.',
    }
    const inactive = { ...stoppedSession, configurationId: secondTunnel.id }
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [savedTunnel, secondTunnel] }],
      sessions: [connected, inactive],
    })
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Show Production bastion details' }))
    await user.click(screen.getByRole('button', { name: 'Start inactive tunnel' }))

    await waitFor(() => expect(api.startServerConfigurations).toHaveBeenCalledWith(savedServer.id))
    expect(api.startConfiguration).toHaveBeenCalledTimes(1)
    expect(api.startConfiguration).toHaveBeenCalledWith(secondTunnel.id)
    expect(screen.queryByRole('button', { name: /Start .*inactive tunnel/ })).toBeNull()
    expect(screen.getAllByLabelText('Tunnel status: Connected')).toHaveLength(2)
  })

  test('refreshes status after a partial bulk-start error so successful tunnels stay visible', async () => {
    const user = userEvent.setup()
    const firstStopped = { ...stoppedSession, configurationId: savedTunnel.id }
    const secondStopped = { ...stoppedSession, configurationId: secondTunnel.id }
    const { api, state } = createFakeApi({
      servers: [{ server: savedServer, configurations: [savedTunnel, secondTunnel] }],
      sessions: [firstStopped, secondStopped],
    })
    api.startServerConfigurations.mockImplementationOnce(async () => {
      state.sessions = [{
        ...firstStopped,
        status: 'connected',
        statusDetail: 'Tunnel connected.',
      }, secondStopped]
      throw new Error('Internal dashboard could not be started.')
    })
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Show Production bastion details' }))
    await user.click(screen.getByRole('button', { name: 'Start all 2 inactive tunnels' }))

    expect(await screen.findByText('Starting inactive tunnels did not complete.')).toBeTruthy()
    expect(api.listRuntimeSessions).toHaveBeenCalled()
    expect(screen.getByLabelText('Tunnel status: Connected')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Start inactive tunnel' })).toBeTruthy()
  })

  test('replaces stale bulk controls with an explicit status refresh', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [savedTunnel] }],
      sessions: [stoppedSession],
    })
    api.listRuntimeSessions.mockRejectedValue(new Error('runtime unavailable'))
    renderApp(api, { pollMs: 5 })

    await user.click(await screen.findByRole('button', { name: 'Show Production bastion details' }))
    expect(await screen.findByRole('button', { name: 'Refresh live status' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Start inactive tunnel' })).toBeNull()
    expect(screen.getByRole('button', { name: /Refresh live status before controlling Admin database/ }).disabled).toBe(true)
  })

  test('lets a dismissed key prompt be reopened from the tunnel detail', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi({
      servers: [{ server: savedServer, configurations: [savedTunnel] }],
      sessions: [stoppedSession],
    })
    const needsAttention = {
      ...stoppedSession,
      status: 'needs_attention',
      statusDetail: 'Unlock the SSH key to continue.',
      needsUserInput: true,
    }
    api.startConfiguration.mockImplementationOnce(async () => {
      state.sessions = [needsAttention]
      return clone(needsAttention)
    })
    await openSavedTunnel(user, api)

    await user.click(screen.getByRole('button', { name: 'Start tunnel' }))
    expect(await screen.findByRole('dialog', { name: 'Unlock SSH key' })).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Not now' }))

    expect(screen.queryByRole('dialog', { name: 'Unlock SSH key' })).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Unlock SSH key' }))
    expect(await screen.findByRole('dialog', { name: 'Unlock SSH key' })).toBeTruthy()
  })

  test('requires confirmation before deleting a tunnel and honors cancellation', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [savedTunnel] }],
      sessions: [stoppedSession],
      history: [historyEntry],
    })
    await openSavedTunnel(user, api)

    await user.click(screen.getByRole('button', { name: 'Delete tunnel' }))
    let dialog = screen.getByRole('dialog', { name: 'Delete Admin database?' })
    expect(within(dialog).getByText(/connection history/)).toBeTruthy()
    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(api.deleteConnectionConfiguration).toHaveBeenCalledTimes(0)
    expect(screen.getByRole('button', { name: 'Start tunnel' })).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Delete tunnel' }))
    dialog = screen.getByRole('dialog', { name: 'Delete Admin database?' })
    await user.click(within(dialog).getByRole('button', { name: 'Delete tunnel' }))

    await screen.findByRole('heading', { name: 'No tunnels yet' })
    expect(api.deleteConnectionConfiguration).toHaveBeenCalledTimes(1)
    expect(api.deleteConnectionConfiguration).toHaveBeenCalledWith(savedTunnel.id)
  })

  test('surfaces the same operation error again after the first message is dismissed', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [savedTunnel] }],
      sessions: [stoppedSession],
    })
    api.startConfiguration = vi.fn(async () => {
      throw new Error('SSH handshake timed out.')
    })
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Show Production bastion details' }))
    await user.click(screen.getByRole('button', { name: 'Start Admin database' }))

    let alert = await screen.findByRole('alert')
    expect(within(alert).getByText('The tunnel could not be started.')).toBeTruthy()
    expect(within(alert).getByText('SSH handshake timed out.')).toBeTruthy()
    await user.click(within(alert).getByRole('button', { name: 'Dismiss message' }))
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull())

    await user.click(screen.getByRole('button', { name: 'Start Admin database' }))
    alert = await screen.findByRole('alert')
    expect(within(alert).getByText('The tunnel could not be started.')).toBeTruthy()
    expect(within(alert).getByText('SSH handshake timed out.')).toBeTruthy()
    expect(api.startConfiguration).toHaveBeenCalledTimes(2)
  })

  test('records and persists forward and backward browser switcher shortcuts', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    renderSettingsApp(api)

    const nextRecorder = await screen.findByRole('button', { name: 'Next browser shortcut' })
    const previousRecorder = screen.getByRole('button', { name: 'Previous browser shortcut' })
    expect(nextRecorder.textContent).toContain('Alt+X')
    expect(previousRecorder.textContent).toContain('Alt+Z')

    await user.click(nextRecorder)
    fireEvent.keyDown(nextRecorder, { key: 'b', code: 'KeyB', altKey: true })

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      browserSwitcherShortcut: 'Alt+B',
      browserSwitcherBackwardShortcut: 'Alt+Z',
    })))
    expect(nextRecorder.textContent).toContain('Alt+B')

    await user.click(previousRecorder)
    fireEvent.keyDown(previousRecorder, { key: 'c', code: 'KeyC', altKey: true })

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      browserSwitcherShortcut: 'Alt+B',
      browserSwitcherBackwardShortcut: 'Alt+C',
    })))
    expect(previousRecorder.textContent).toContain('Alt+C')
  })

  test('serializes overlapping preference saves onto the latest persisted revision', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi()
    state.preferences.updatedAt = '2026-07-29T20:00:00Z'
    let resolveFirst
    let resolveSecond
    api.savePreferences
      .mockImplementationOnce((input) => new Promise((resolve) => {
        resolveFirst = () => resolve({ ...input, updatedAt: '2026-07-29T20:00:01Z' })
      }))
      .mockImplementationOnce((input) => new Promise((resolve) => {
        resolveSecond = () => resolve({ ...input, updatedAt: '2026-07-29T20:00:02Z' })
      }))
    renderSettingsApp(api)

    const nextRecorder = await screen.findByRole('button', { name: 'Next browser shortcut' })
    const previousRecorder = screen.getByRole('button', { name: 'Previous browser shortcut' })
    await user.click(nextRecorder)
    fireEvent.keyDown(nextRecorder, { key: 'b', code: 'KeyB', altKey: true })
    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledTimes(1))

    await user.click(previousRecorder)
    fireEvent.keyDown(previousRecorder, { key: 'c', code: 'KeyC', altKey: true })
    expect(api.savePreferences).toHaveBeenCalledTimes(1)

    await act(async () => resolveFirst())
    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledTimes(2))
    expect(api.savePreferences.mock.calls[1][0]).toEqual(expect.objectContaining({
      browserSwitcherShortcut: 'Alt+B',
      browserSwitcherBackwardShortcut: 'Alt+C',
      updatedAt: '2026-07-29T20:00:01Z',
    }))

    await act(async () => resolveSecond())
    await waitFor(() => expect(previousRecorder.textContent).toContain('Alt+C'))
  })

  test('applies a delayed server selection to the latest persisted preferences', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi()
    state.preferences.updatedAt = '2026-07-29T20:00:00Z'
    let resolveServer
    api.saveServer.mockImplementationOnce((input) => new Promise((resolve) => {
      resolveServer = () => resolve({ ...input, id: 'server-2' })
    }))
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Add server' }))
    await user.type(screen.getByLabelText('Name'), 'Work server')
    await user.type(screen.getByLabelText('Host'), 'work.example.com')
    await user.click(screen.getByRole('button', { name: 'Save server' }))
    await waitFor(() => expect(api.saveServer).toHaveBeenCalledTimes(1))

    state.preferences = {
      ...state.preferences,
      theme: 'light',
      updatedAt: '2026-07-29T20:00:01Z',
    }
    await act(async () => api.preferencesChangedListener(clone(state.preferences)))
    await waitFor(() => expect(api.loadInitialState).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe('light'))

    await act(async () => resolveServer())
    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledTimes(1))
    expect(api.savePreferences.mock.calls[0][0]).toEqual(expect.objectContaining({
      lastSelectedServerId: 'server-2',
      theme: 'light',
      updatedAt: '2026-07-29T20:00:01Z',
    }))
  })

  test.each(['resolves', 'rejects'])(
    'keeps a newer hydrated preference revision when an older save %s',
    async (outcome) => {
      const user = userEvent.setup()
      const { api, state } = createFakeApi()
      state.preferences.updatedAt = '2026-07-29T20:00:00.100000000Z'
      let settleSave
      api.savePreferences.mockImplementationOnce((input) => new Promise((resolve, reject) => {
        settleSave = () => {
          if (outcome === 'resolves') {
            resolve({ ...input, theme: 'dark', updatedAt: '2026-07-29T20:00:00.200000000Z' })
          } else {
            reject(new Error('stale preference save failed'))
          }
        }
      }))
      renderSettingsApp(api)

      const nextRecorder = await screen.findByRole('button', { name: 'Next browser shortcut' })
      const previousRecorder = screen.getByRole('button', { name: 'Previous browser shortcut' })
      await user.click(nextRecorder)
      fireEvent.keyDown(nextRecorder, { key: 'b', code: 'KeyB', altKey: true })
      await waitFor(() => expect(api.savePreferences).toHaveBeenCalledTimes(1))

      state.preferences = {
        ...state.preferences,
        theme: 'light',
        browserSwitcherShortcut: 'Alt+B',
        updatedAt: '2026-07-29T20:00:00.300000000Z',
      }
      await act(async () => api.preferencesChangedListener(clone(state.preferences)))
      await waitFor(() => expect(api.loadInitialState).toHaveBeenCalledTimes(2))
      await waitFor(() => expect(document.documentElement.dataset.theme).toBe('light'))

      await act(async () => settleSave())
      await waitFor(() => expect(document.documentElement.dataset.theme).toBe('light'))

      await user.click(previousRecorder)
      fireEvent.keyDown(previousRecorder, { key: 'c', code: 'KeyC', altKey: true })
      await waitFor(() => expect(api.savePreferences).toHaveBeenCalledTimes(2))
      expect(api.savePreferences.mock.calls[1][0]).toEqual(expect.objectContaining({
        theme: 'light',
        browserSwitcherShortcut: 'Alt+B',
        browserSwitcherBackwardShortcut: 'Alt+C',
        updatedAt: '2026-07-29T20:00:00.300000000Z',
      }))
    },
  )

  test('uses an accepted preference event while its follow-up hydration is pending', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi()
    state.preferences.updatedAt = '2026-07-29T20:00:00.100000000Z'
    const loadCurrentState = api.loadInitialState.getMockImplementation()
    let resolveHydration
    renderSettingsApp(api)

    const previousRecorder = await screen.findByRole('button', { name: 'Previous browser shortcut' })
    api.loadInitialState.mockImplementationOnce(() => new Promise((resolve) => {
      resolveHydration = async () => resolve(await loadCurrentState())
    }))
    state.preferences = {
      ...state.preferences,
      theme: 'light',
      browserSwitcherShortcut: 'Alt+B',
      updatedAt: '2026-07-29T20:00:00.200000000Z',
    }
    act(() => api.preferencesChangedListener(clone(state.preferences)))
    await waitFor(() => expect(api.loadInitialState).toHaveBeenCalledTimes(2))

    await user.click(previousRecorder)
    fireEvent.keyDown(previousRecorder, { key: 'c', code: 'KeyC', altKey: true })
    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledTimes(1))
    expect(api.savePreferences.mock.calls[0][0]).toEqual(expect.objectContaining({
      theme: 'light',
      browserSwitcherShortcut: 'Alt+B',
      browserSwitcherBackwardShortcut: 'Alt+C',
      updatedAt: '2026-07-29T20:00:00.200000000Z',
    }))

    await act(async () => resolveHydration())
  })

  test('refreshes persisted preferences before continuing after a stale-save conflict', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi()
    state.preferences.updatedAt = '2026-07-29T20:00:00.100000000Z'
    let rejectFirstSave
    api.savePreferences.mockImplementationOnce(() => new Promise((resolve, reject) => {
      rejectFirstSave = () => reject(
        new Error('preference update conflict'),
      )
    }))
    renderSettingsApp(api)

    const nextRecorder = await screen.findByRole('button', { name: 'Next browser shortcut' })
    const previousRecorder = screen.getByRole('button', { name: 'Previous browser shortcut' })
    await user.click(nextRecorder)
    fireEvent.keyDown(nextRecorder, { key: 'b', code: 'KeyB', altKey: true })
    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledTimes(1))
    await user.click(previousRecorder)
    fireEvent.keyDown(previousRecorder, { key: 'c', code: 'KeyC', altKey: true })
    expect(api.savePreferences).toHaveBeenCalledTimes(1)

    state.preferences = {
      ...state.preferences,
      theme: 'light',
      updatedAt: '2026-07-29T20:00:00.200000000Z',
    }
    await act(async () => rejectFirstSave())

    await waitFor(() => expect(api.loadInitialState).toHaveBeenCalledTimes(2))
    expect(await screen.findByText('Settings changed before this update was saved. Please try again.')).toBeTruthy()
    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledTimes(2))
    expect(api.savePreferences.mock.calls[1][0]).toEqual(expect.objectContaining({
      theme: 'light',
      browserSwitcherShortcut: 'Alt+X',
      browserSwitcherBackwardShortcut: 'Alt+C',
      updatedAt: '2026-07-29T20:00:00.200000000Z',
    }))
  })

  test('uses the settings sidebar and removes disabled browsers from routing controls', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
      { id: 'firefox', displayName: 'Firefox', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    expect(await screen.findByRole('heading', { name: 'General' })).toBeTruthy()
    expect(screen.getByRole('navigation', { name: 'Settings pages' })).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Browsers' }))
    expect(await screen.findByRole('heading', { name: 'Browsers' })).toBeTruthy()
    await waitFor(() => expect(api.defaultBrowserStatus).toHaveBeenCalled())

    await user.click(screen.getByRole('checkbox', { name: 'Enable Safari' }))
    await user.click(screen.getByRole('button', { name: 'Save all settings' }))

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      disabledBrowserIds: ['safari'],
    })))

    await user.click(screen.getByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    expect(within(screen.getByRole('combobox', { name: 'Rule 1 browser' })).queryByRole('option', { name: 'Safari' })).toBeNull()
  })

  test('shows one empty state when no URL rules exist', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))

    expect(screen.getByText('No URL rules')).toBeTruthy()
    expect(screen.queryByText('Select a rule')).toBeNull()
    expect(screen.getByRole('heading', { name: 'Port defaults' })).toBeTruthy()
  })

  test('explains when default-browser integration is unavailable', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.defaultBrowserStatus.mockResolvedValue({ supported: false, isDefault: false })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))

    expect(screen.getByText('Default browser integration is unavailable')).toBeTruthy()
    expect(screen.getByText('You can still launch any detected browser through an SSH tunnel.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Unavailable' }).disabled).toBe(true)
  })

  test('offers custom command browsers only where command launching is supported', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.defaultBrowserStatus.mockResolvedValue({ supported: false, isDefault: false })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))

    expect(screen.queryByRole('button', { name: 'Add custom browser' })).toBeNull()
    expect(screen.getByText('Choose which installed browsers SSH Man can offer.')).toBeTruthy()
  })

  test('does not offer new command rules where command launching is unsupported', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.defaultBrowserStatus.mockResolvedValue({ supported: false, isDefault: false })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))

    const action = screen.getByRole('combobox', { name: 'Rule 1 action' })
    expect(within(action).queryByRole('option', { name: 'Run command' })).toBeNull()
  })

  test('keeps saved command rules readable and repairable on unsupported platforms', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi()
    state.preferences.urlRules = [{
      id: 'saved-command',
      matchMode: 'contains',
      pattern: 'example.com',
      action: 'command',
      browserId: '',
      command: 'open "<URL>"',
      openDirect: false,
    }]
    api.defaultBrowserStatus.mockResolvedValue({ supported: false, isDefault: false })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))

    const action = screen.getByRole('combobox', { name: 'Rule 1 action' })
    expect(within(action).getByRole('option', { name: 'Run command (saved)' }).disabled).toBe(true)
    expect(screen.getByRole('textbox', { name: 'Rule 1 command' }).readOnly).toBe(true)
    expect(screen.getByText('This saved command remains unchanged. Switch the action to Open in browser, or use SSH Man on macOS to edit it.')).toBeTruthy()

    await user.selectOptions(action, 'browser')
    expect(screen.queryByRole('textbox', { name: 'Rule 1 command' })).toBeNull()
    expect(screen.getByRole('combobox', { name: 'Rule 1 browser' })).toBeTruthy()
  })

  test('saves unrelated edits while unchanged legacy commands remain repairable', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi()
    state.preferences.customBrowsers = [{
      id: 'legacy-browser',
      displayName: 'Legacy browser',
      command: '/bin/zsh -lc "open <URL>"',
      icon: '',
    }]
    state.preferences.urlRules = [{
      id: 'legacy-rule',
      matchMode: 'contains',
      pattern: 'example.com',
      action: 'command',
      browserId: '',
      command: '/bin/zsh -lc "open <URL>"',
      openDirect: false,
    }]
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    const name = screen.getByRole('textbox', { name: 'Custom browser name' })
    await user.clear(name)
    await user.type(name, 'Updated legacy browser')

    const save = screen.getByRole('button', { name: 'Save all settings' })
    expect(save.disabled).toBe(false)
    await user.click(save)

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      customBrowsers: [expect.objectContaining({
        id: 'legacy-browser',
        displayName: 'Updated legacy browser',
        command: '/bin/zsh -lc "open <URL>"',
      })],
      urlRules: [expect.objectContaining({
        id: 'legacy-rule',
        command: '/bin/zsh -lc "open <URL>"',
      })],
    })))
  })

  test('keeps saved custom browser commands read-only on unsupported platforms', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi()
    state.preferences.customBrowsers = [{
      id: 'saved-browser',
      displayName: 'Work browser',
      command: 'open -a "Zen" "<URL>"',
      icon: 'icon:briefcase',
    }]
    api.defaultBrowserStatus.mockResolvedValue({ supported: false, isDefault: false })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))

    expect(screen.getByRole('textbox', { name: 'Custom browser name' }).readOnly).toBe(false)
    expect(screen.getByRole('textbox', { name: 'Custom browser command' }).readOnly).toBe(true)
    expect(screen.getByText('This saved command remains unchanged. Use SSH Man on macOS to edit it.')).toBeTruthy()
    expect(screen.getByRole('radio', { name: 'Briefcase icon' }).disabled).toBe(false)
  })

  test('gives an available recovery path when no browsers are detected', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.defaultBrowserStatus.mockResolvedValue({ supported: false, isDefault: false })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))

    expect(screen.getByText('No browsers found')).toBeTruthy()
    expect(screen.getByText('Install a browser on this computer, then reopen Settings to detect it.')).toBeTruthy()
  })

  test('adds a command-based custom browser with an icon and uses it in a rule', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('button', { name: 'Add custom browser' }))
    await user.type(screen.getByRole('textbox', { name: 'Custom browser name' }), 'Work browser')
    await user.type(screen.getByRole('textbox', { name: 'Custom browser command' }), 'open -a "Zen" "<URL>"')
    await user.click(screen.getByRole('radio', { name: 'Briefcase icon' }))

    expect(within(screen.getByRole('combobox', { name: 'Fallback browser' })).getByRole('option', { name: 'Work browser' })).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Save all settings' }))

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      customBrowsers: [
        expect.objectContaining({
          displayName: 'Work browser',
          command: 'open -a "Zen" "<URL>"',
          icon: 'icon:briefcase',
        }),
      ],
    })))

    await user.click(screen.getByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.selectOptions(screen.getByRole('combobox', { name: 'Rule 1 browser' }), screen.getByRole('option', { name: 'Work browser' }))
    expect(screen.getByRole('combobox', { name: 'Rule 1 browser' }).value).toMatch(/^custom-browser-/)
  })

  test('focuses the editor for each newly created settings item', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('button', { name: 'Add custom browser' }))
    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('textbox', { name: 'Custom browser name' }),
    ))

    await user.click(screen.getByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('textbox', { name: 'Rule 1 match value' }),
    ))
  })

  test('announces the active browser and rule summary in master-detail lists', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'firefox', displayName: 'Firefox', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    const chromeRow = screen.getByRole('button', { name: 'Edit Google Chrome' })
    const firefoxRow = screen.getByRole('button', { name: 'Edit Firefox' })
    expect(chromeRow.getAttribute('aria-current')).toBe('true')
    expect(firefoxRow.hasAttribute('aria-current')).toBe(false)

    await user.click(firefoxRow)
    expect(chromeRow.hasAttribute('aria-current')).toBe(false)
    expect(firefoxRow.getAttribute('aria-current')).toBe('true')

    await user.click(screen.getByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.click(screen.getByRole('button', { name: 'Starts with' }))
    await user.type(screen.getByRole('textbox', { name: 'Rule 1 match value' }), 'https://work.example/')
    await user.click(screen.getByRole('checkbox', { name: 'Skip selection for rule 1' }))

    expect(screen.getByText('Open immediately when available. Otherwise, SSH Man shows the chooser.')).toBeTruthy()
    expect(screen.getByRole('button', {
      name: 'Edit rule 1: https://work.example/; Starts with; Open directly',
    }).getAttribute('aria-current')).toBe('true')
  })

  test('explains literal rule matching against the complete link', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))

    const matchValue = screen.getByRole('textbox', { name: 'Rule 1 match value' })
    expect(screen.getByText('SSH Man compares this value with the complete link.')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Starts with' }))
    expect(matchValue.placeholder).toBe('https://github.com/')

    await user.click(screen.getByRole('button', { name: 'Ends with' }))
    expect(matchValue.placeholder).toBe('/login')

    await user.click(screen.getByRole('button', { name: 'Contains' }))
    expect(matchValue.placeholder).toBe('github.com')
  })

  test('restores focus to an adjacent browser or the add button after deletion', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('button', { name: 'Add custom browser' }))
    await user.type(screen.getByRole('textbox', { name: 'Custom browser name' }), 'First browser')
    await user.click(screen.getByRole('button', { name: 'Add custom browser' }))
    await user.type(screen.getByRole('textbox', { name: 'Custom browser name' }), 'Second browser')

    await user.click(screen.getByRole('button', { name: 'Delete custom browser' }))
    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('button', { name: 'Edit First browser' }),
    ))

    await user.click(screen.getByRole('button', { name: 'Delete custom browser' }))
    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('button', { name: 'Add custom browser' }),
    ))
  })

  test('restores focus to an adjacent rule or the add button after deletion', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))

    await user.click(screen.getByRole('button', { name: 'Remove rule 2' }))
    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('button', { name: 'Edit rule 1: New rule; Contains; Show chooser' }),
    ))

    await user.click(screen.getByRole('button', { name: 'Remove rule 1' }))
    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('button', { name: 'Add rule' }),
    ))
  })

  test('restores focus to an adjacent port assignment or the add button after deletion', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [managedBrowserProxy] }],
    })
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Assign port' }))
    await user.click(screen.getByRole('button', { name: 'Assign port' }))

    await user.click(screen.getByRole('button', { name: 'Remove port assignment 1' }))
    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('spinbutton', { name: 'Port assignment 1 port' }),
    ))

    await user.click(screen.getByRole('button', { name: 'Remove port assignment 1' }))
    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('button', { name: 'Assign port' }),
    ))
  })

  test('restores focus to Add rule when the empty port-assignment fallback is disabled', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi()
    state.preferences.urlPortAssignments = [{
      id: 'unavailable-port-default',
      port: 3000,
      serverId: 'unavailable-server',
      browserId: 'unavailable-browser',
    }]
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    expect(screen.getByRole('button', { name: 'Assign port' }).disabled).toBe(true)
    await user.click(screen.getByRole('button', { name: 'Remove port assignment 1' }))

    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('button', { name: 'Add rule' }),
    ))
  })

  test('saves literal starts-with rules that open directly', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'firefox', displayName: 'Firefox', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.click(screen.getByRole('button', { name: 'Starts with' }))
    await user.type(screen.getByRole('textbox', { name: 'Rule 1 match value' }), 'https://work.example/')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Rule 1 browser' }), 'firefox')
    await user.click(screen.getByRole('checkbox', { name: 'Skip selection for rule 1' }))
    expect(screen.getByRole('button', { name: 'URL routing' }).querySelector('.settings-nav-badge').title).toBe('Unsaved changes')
    await user.click(screen.getByRole('button', { name: 'Save all settings' }))

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      urlRules: [
        expect.objectContaining({
          matchMode: 'starts_with',
          pattern: 'https://work.example/',
          action: 'browser',
          browserId: 'firefox',
          openDirect: true,
        }),
      ],
    })))
  })

  test('shows backend regex validation on the rule match field', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    api.validateURLRulePattern.mockImplementation(async (matchMode, pattern) => {
      if (matchMode === 'regex' && pattern === '(?=work)') {
        return { valid: false, message: 'Enter a valid regular expression.' }
      }
      return { valid: true, message: '' }
    })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.click(screen.getByRole('button', { name: 'Regex' }))
    const matchValue = screen.getByRole('textbox', { name: 'Rule 1 match value' })
    await user.type(matchValue, '(?=work)')

    expect(await screen.findByText('Enter a valid regular expression.')).toBeTruthy()
    expect(matchValue.getAttribute('aria-invalid')).toBe('true')
    expect(screen.getByRole('button', { name: 'Save all settings' }).disabled).toBe(true)

    await user.clear(matchValue)
    await user.type(matchValue, '(?P<name>work)')
    await waitFor(() => expect(matchValue.getAttribute('aria-invalid')).toBe('false'))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save all settings' }).disabled).toBe(false))
  })

  test('retries regex validation transport failures without changing the pattern', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    let attempts = 0
    api.validateURLRulePattern.mockImplementation(async () => {
      attempts += 1
      if (attempts === 1) throw new Error('bridge unavailable')
      return { valid: true, message: '' }
    })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.click(screen.getByRole('button', { name: 'Regex' }))
    const matchValue = screen.getByRole('textbox', { name: 'Rule 1 match value' })
    await user.type(matchValue, 'work')

    expect(await screen.findByText('Regular expressions could not be checked. Retry validation.')).toBeTruthy()
    expect(matchValue.getAttribute('aria-invalid')).toBe('false')
    expect(screen.getByRole('button', { name: 'Save all settings' }).disabled).toBe(true)
    expect(screen.getByRole('button', { name: 'Make SSH Man default' }).disabled).toBe(true)

    await user.click(screen.getByRole('button', { name: 'Retry validation' }))

    await waitFor(() => expect(api.validateURLRulePattern).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Save all settings' }).disabled).toBe(false))
    expect(screen.getByRole('button', { name: 'Make SSH Man default' }).disabled).toBe(false)
    expect(matchValue.value).toBe('work')
  })

  test('surfaces cross-page validation when saving shared browser and routing settings', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.click(screen.getByRole('button', { name: 'Browsers' }))

    expect(screen.getByRole('button', { name: 'Save all settings' }).disabled).toBe(true)
    expect(screen.getByText('1 URL routing field needs attention.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'URL routing' }).querySelector('.settings-nav-badge').title).toBe('1 field needs attention')
  })

  test('retains unsaved browser changes when persistence fails', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
    ])
    let rejectSave
    api.savePreferences.mockImplementationOnce(() => new Promise((resolve, reject) => {
      rejectSave = reject
    }))
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Safari' }))
    await user.click(screen.getByRole('button', { name: 'Save all settings' }))

    expect(screen.getByRole('status').textContent).toBe('Saving browser and routing settings…')
    await act(async () => rejectSave(new Error('Preference write failed.')))

    expect(await screen.findByText('Your preference could not be saved.')).toBeTruthy()
    expect(screen.getByRole('checkbox', { name: 'Enable Safari' }).checked).toBe(false)
    expect(screen.getByRole('button', { name: 'Save all settings' }).disabled).toBe(false)
    expect(screen.getByText('Unsaved browser and routing changes are ready to save.', {
      selector: '.settings-save-bar [role="status"]',
    })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Browsers' }).querySelector('.settings-nav-badge').title).toBe('Unsaved changes')
  })

  test('retains newer browser changes when an earlier save succeeds', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
    ])
    let resolveSave
    api.savePreferences.mockImplementationOnce(() => new Promise((resolve) => {
      resolveSave = resolve
    }))
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Safari' }))
    await user.click(screen.getByRole('button', { name: 'Save all settings' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Google Chrome' }))
    await act(async () => resolveSave())

    expect(screen.getByRole('checkbox', { name: 'Enable Safari' }).checked).toBe(false)
    expect(screen.getByRole('checkbox', { name: 'Enable Google Chrome' }).checked).toBe(false)
    expect(screen.getByText('Unsaved browser and routing changes are ready to save.', {
      selector: '.settings-save-bar [role="status"]',
    })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Browsers' }).querySelector('.settings-nav-badge').title).toBe('Unsaved changes')
  })

  test('guards explicit and native settings close requests while changes are unsaved', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Safari' }))
    await user.click(screen.getByRole('button', { name: 'Close Settings' }))

    let dialog = screen.getByRole('dialog', { name: 'Unsaved settings' })
    expect(within(dialog).getByRole('button', { name: 'Save all settings' })).toBeTruthy()
    await user.click(within(dialog).getByRole('button', { name: 'Keep editing' }))
    expect(api.quitApplication).not.toHaveBeenCalled()
    await waitFor(() => expect(document.activeElement).toBe(
      screen.getByRole('button', { name: 'Close Settings' }),
    ))

    const safariToggle = screen.getByRole('checkbox', { name: 'Enable Safari' })
    safariToggle.focus()
    act(() => api.settingsCloseListener())
    dialog = screen.getByRole('dialog', { name: 'Unsaved settings' })
    await user.click(within(dialog).getByRole('button', { name: 'Keep editing' }))
    await waitFor(() => expect(document.activeElement).toBe(safariToggle))

    act(() => api.settingsCloseListener())
    dialog = screen.getByRole('dialog', { name: 'Unsaved settings' })
    await user.click(within(dialog).getByRole('button', { name: 'Discard changes' }))

    await waitFor(() => expect(api.allowSettingsWindowClose).toHaveBeenCalledTimes(1))
    expect(api.quitApplication).toHaveBeenCalledTimes(1)
  })

  test('keeps discard available while regular expressions are still being checked', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    api.validateURLRulePattern.mockImplementation(() => new Promise(() => {}))
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.click(screen.getByRole('button', { name: 'Regex' }))
    await user.type(screen.getByRole('textbox', { name: 'Rule 1 match value' }), 'work')
    expect(await screen.findByText('Checking regular expressions…')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Close Settings' }))
    const dialog = screen.getByRole('dialog', { name: 'Unsaved settings' })
    expect(within(dialog).getByRole('button', { name: 'Keep editing' }).disabled).toBe(false)
    expect(within(dialog).getByRole('button', { name: 'Discard changes' }).disabled).toBe(false)
    expect(within(dialog).queryByRole('button', { name: 'Save all settings' })).toBeNull()
    expect(within(dialog).getByText('Regular expressions are still being checked. Keep editing or discard these changes.')).toBeTruthy()

    await user.click(within(dialog).getByRole('button', { name: 'Discard changes' }))
    await waitFor(() => expect(api.allowSettingsWindowClose).toHaveBeenCalledTimes(1))
    expect(api.quitApplication).toHaveBeenCalledTimes(1)
  })

  test('keeps unsaved settings guarded while the browser customizer is open', async () => {
    const user = userEvent.setup()
    const runningBrowser = {
      id: 'browser:101',
      pid: 101,
      browserId: 'google-chrome',
      browserName: 'Google Chrome',
      kind: 'regular',
    }
    const { api } = createFakeApi({ runningBrowsers: [runningBrowser] })
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Safari' }))
    await user.click(screen.getByRole('button', { name: 'General' }))
    await user.click(screen.getByRole('button', { name: 'Customize' }))
    expect(await screen.findByRole('dialog', { name: 'Switch browser' })).toBeTruthy()

    act(() => api.settingsCloseListener())

    const dialog = await screen.findByRole('dialog', { name: 'Unsaved settings' })
    expect(screen.queryByRole('dialog', { name: 'Switch browser' })).toBeNull()
    expect(api.allowSettingsWindowClose).not.toHaveBeenCalled()
    expect(api.quitApplication).not.toHaveBeenCalled()

    await user.click(within(dialog).getByRole('button', { name: 'Keep editing' }))
    await user.click(screen.getByRole('button', { name: 'Browsers' }))
    expect(screen.getByRole('checkbox', { name: 'Enable Safari' }).checked).toBe(false)
  })

  test('keeps settings notifications outside the browser customizer modal', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Safari' }))
    await user.click(screen.getByRole('button', { name: 'Save all settings' }))
    expect(await screen.findByText('Browser and routing settings saved.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Dismiss message' })).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'General' }))
    await user.click(screen.getByRole('button', { name: 'Customize' }))
    expect(await screen.findByRole('dialog', { name: 'Switch browser' })).toBeTruthy()
    expect(screen.queryByText('Browser and routing settings saved.')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Dismiss message' })).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Close browser switcher' }))
    expect(await screen.findByText('Browser and routing settings saved.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Dismiss message' })).toBeTruthy()
  })

  test('allows the frontend to close immediately after a clean native close request', async () => {
    const { api } = createFakeApi()
    renderSettingsApp(api)

    await screen.findByRole('heading', { name: 'General' })
    act(() => api.settingsCloseListener())

    await waitFor(() => expect(api.allowSettingsWindowClose).toHaveBeenCalledTimes(1))
    expect(api.quitApplication).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog', { name: 'Unsaved settings' })).toBeNull()
  })

  test('allows native close while settings are still loading', async () => {
    const { api } = createFakeApi()
    api.loadInitialState.mockImplementationOnce(() => new Promise(() => {}))
    renderSettingsApp(api)

    await waitFor(() => expect(api.onSettingsCloseRequested).toHaveBeenCalledTimes(1))
    act(() => api.settingsCloseListener())

    await waitFor(() => expect(api.allowSettingsWindowClose).toHaveBeenCalledTimes(1))
    expect(api.quitApplication).toHaveBeenCalledTimes(1)
  })

  test('allows native close after settings fail to load', async () => {
    const { api } = createFakeApi()
    api.loadInitialState.mockRejectedValueOnce(new Error('Settings load failed.'))
    renderSettingsApp(api)

    expect(await screen.findByRole('heading', { name: 'Settings did not load' })).toBeTruthy()
    act(() => api.settingsCloseListener())

    await waitFor(() => expect(api.allowSettingsWindowClose).toHaveBeenCalledTimes(1))
    expect(api.quitApplication).toHaveBeenCalledTimes(1)
  })

  test('saves valid settings from the close guard before closing', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Safari' }))
    await user.click(screen.getByRole('button', { name: 'Close Settings' }))
    const dialog = screen.getByRole('dialog', { name: 'Unsaved settings' })
    await user.click(within(dialog).getByRole('button', { name: 'Save all settings' }))

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalled())
    expect(api.allowSettingsWindowClose).toHaveBeenCalledTimes(1)
    expect(api.quitApplication).toHaveBeenCalledTimes(1)
  })

  test('shows save failures inside the unsaved-settings close guard', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
    ])
    api.savePreferences.mockRejectedValueOnce(new Error('Preference write failed.'))
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Safari' }))
    await user.click(screen.getByRole('button', { name: 'Close Settings' }))
    const dialog = screen.getByRole('dialog', { name: 'Unsaved settings' })
    await user.click(within(dialog).getByRole('button', { name: 'Save all settings' }))

    expect((await within(dialog).findByRole('alert')).textContent).toBe(
      'Settings could not be saved. Your changes are still here. Try again or keep editing.',
    )
    expect(within(dialog).getByRole('button', { name: 'Save all settings' }).disabled).toBe(false)
    expect(api.allowSettingsWindowClose).not.toHaveBeenCalled()
    expect(api.quitApplication).not.toHaveBeenCalled()
  })

  test('requires invalid settings to be fixed or discarded before closing', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    act(() => api.settingsCloseListener())

    const dialog = screen.getByRole('dialog', { name: 'Unsaved settings' })
    expect(within(dialog).queryByRole('button', { name: 'Save all settings' })).toBeNull()
    expect(within(dialog).getByText('Fix the fields that need attention, or discard these changes.')).toBeTruthy()
  })

  test('associates custom browser validation with its fields', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('button', { name: 'Add custom browser' }))

    const name = screen.getByRole('textbox', { name: 'Custom browser name' })
    const command = screen.getByRole('textbox', { name: 'Custom browser command' })
    expect(document.getElementById(name.getAttribute('aria-describedby')).textContent).toBe('Add a name for this browser.')
    expect(document.getElementById(command.getAttribute('aria-describedby')).textContent).toContain('Add a command that includes <URL>.')
  })

  test('explains that browser commands use macOS open', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('button', { name: 'Add custom browser' }))

    expect(screen.getByText((_, element) => (
      element.tagName === 'SMALL' &&
      /use a macOS open command/i.test(element.textContent)
    ))).toBeTruthy()
  })

  test('repairs empty rule and port browser references after re-enabling a browser', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [managedBrowserProxy, savedTunnel] }],
    })
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    await user.type(screen.getByRole('textbox', { name: 'Rule 1 match value' }), 'work.example')
    await user.click(screen.getByRole('button', { name: 'Assign port' }))
    await user.type(screen.getByRole('spinbutton', { name: 'Port assignment 1 port' }), '3000')

    await user.click(screen.getByRole('button', { name: 'Browsers' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Google Chrome' }))
    await user.click(screen.getByRole('checkbox', { name: 'Enable Google Chrome' }))
    await user.click(screen.getByRole('button', { name: 'URL routing' }))

    const ruleBrowser = screen.getByRole('combobox', { name: 'Rule 1 browser' })
    const portBrowser = screen.getByRole('combobox', { name: 'Port assignment 1 browser' })
    expect(ruleBrowser.value).toBe('')
    expect(portBrowser.value).toBe('')
    expect(within(ruleBrowser).getByRole('option', { name: 'Choose a browser' }).selected).toBe(true)
    expect(within(portBrowser).getByRole('option', { name: 'Choose a browser' }).selected).toBe(true)

    await user.selectOptions(ruleBrowser, 'google-chrome')
    await user.selectOptions(portBrowser, 'google-chrome')
    expect(ruleBrowser.getAttribute('aria-invalid')).toBe('false')
    expect(portBrowser.getAttribute('aria-invalid')).toBe('false')
  })

  test('assigns an explicit URL port to a browser and saved host', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [managedBrowserProxy, savedTunnel] }],
    })
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
      { id: 'firefox', displayName: 'Firefox', supportsProxyLaunch: true },
      { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    const assignPortButton = screen.getByRole('button', { name: 'Assign port' })
    await waitFor(() => expect(assignPortButton.disabled).toBe(false))
    await user.click(assignPortButton)
    await user.type(screen.getByRole('spinbutton', { name: 'Port assignment 1 port' }), '3000')
    await user.selectOptions(screen.getByRole('combobox', { name: 'Port assignment 1 host' }), savedServer.id)
    await user.selectOptions(screen.getByRole('combobox', { name: 'Port assignment 1 browser' }), 'firefox')
    await user.click(screen.getByRole('button', { name: 'Save all settings' }))

    await waitFor(() => expect(api.savePreferences).toHaveBeenCalledWith(expect.objectContaining({
      urlPortAssignments: [
        expect.objectContaining({
          port: 3000,
          serverId: savedServer.id,
          browserId: 'firefox',
        }),
      ],
    })))
  })

  test('shows and associates port-default validation errors', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi({
      servers: [{ server: savedServer, configurations: [managedBrowserProxy, savedTunnel] }],
    })
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'URL routing' }))
    await user.click(screen.getByRole('button', { name: 'Assign port' }))

    const port = screen.getByRole('spinbutton', { name: 'Port assignment 1 port' })
    expect(port.getAttribute('aria-invalid')).toBe('true')
    expect(document.getElementById(port.getAttribute('aria-describedby')).textContent).toBe('Enter a port from 1 to 65535.')
  })

  test('offers installed Zen for regular and SOCKS URL routing', async () => {
    const user = userEvent.setup()
    const { api } = createFakeApi()
    api.discoverBrowsers.mockResolvedValue([
      { id: 'google-chrome', displayName: 'Google Chrome', engine: 'chromium', supportsProxyLaunch: true },
      { id: 'zen', displayName: 'Zen', engine: 'firefox', supportsProxyLaunch: true },
    ])
    renderSettingsApp(api)

    await waitFor(() => expect(api.discoverBrowsers).toHaveBeenCalled())
    await user.click(screen.getByRole('button', { name: 'Browsers' }))

    expect(within(screen.getByRole('combobox', { name: 'Fallback browser' })).getByRole('option', { name: 'Zen' })).toBeTruthy()
    expect(within(screen.getByRole('combobox', { name: 'SOCKS proxy browser' })).getByRole('option', { name: 'Zen' })).toBeTruthy()
  })

  test('shows a timed destination chooser for opened URLs', async () => {
    const user = userEvent.setup()
    const { api, state } = createFakeApi()
    state.preferences.browserAppearances = {
      'proxy:staging:google-chrome': { icon: '🚀', primaryColor: '#22C55E' },
    }
    renderApp(api)
    await waitFor(() => expect(api.urlRouteChoiceListener).toBeTypeOf('function'))

    act(() => {
      api.urlRouteChoiceListener({
        id: 'route-1',
        url: 'http://localhost:3000/dashboard',
        defaultChoiceId: 'browser:safari',
        timeoutMilliseconds: 5000,
        choices: [
          { id: 'browser:safari', kind: 'browser', label: 'Safari', detail: 'Regular browser', browserId: 'safari' },
          { id: 'browser:work', kind: 'command', label: 'Work browser', detail: 'Custom browser', browserId: 'work', icon: 'icon:briefcase' },
          { id: 'proxy:server-socks:staging:google-chrome', kind: 'proxy', label: 'Google Chrome through Staging', detail: 'SOCKS5 proxy', serverId: 'staging', serverName: 'Staging', configurationId: 'server-socks:staging', browserId: 'google-chrome' },
        ],
      })
    })

    const dialog = await screen.findByRole('dialog', { name: 'Choose where to open this link' })
    expect(within(dialog).getByText('http://localhost:3000/dashboard')).toBeTruthy()
    expect(within(dialog).getByText('Opening Safari in 5s')).toBeTruthy()
    const proxyChoice = within(dialog).getByRole('option', { name: 'Open in Google Chrome through Staging' })
    const customChoice = within(dialog).getByRole('option', { name: 'Open in Work browser' })
    expect(customChoice.querySelector('.lucide-briefcase')).toBeTruthy()
    expect(proxyChoice.textContent).toContain('🚀')
    expect(proxyChoice.style.getPropertyValue('--browser-primary')).toBe('#22C55E')
    await user.click(proxyChoice)

    await waitFor(() => expect(api.resolveURLRoute).toHaveBeenCalledWith('route-1', 'proxy:server-socks:staging:google-chrome'))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Choose where to open this link' })).toBeNull())
    expect(api.setURLRouteWindowMode).toHaveBeenCalledWith(true)
    expect(api.hideApplicationWindow).toHaveBeenCalled()
  })

  test('dismisses an open URL chooser when browser routing preferences change', async () => {
    const { api, state } = createFakeApi()
    renderApp(api)
    await waitFor(() => expect(api.urlRouteChoiceListener).toBeTypeOf('function'))
    await waitFor(() => expect(api.preferencesChangedListener).toBeTypeOf('function'))

    act(() => {
      api.urlRouteChoiceListener({
        id: 'route-stale',
        url: 'https://example.com',
        defaultChoiceId: 'browser:chrome',
        timeoutMilliseconds: 5000,
        choices: [
          { id: 'browser:chrome', kind: 'browser', label: 'Chrome', browserId: 'chrome' },
        ],
      })
    })
    await screen.findByRole('dialog', { name: 'Choose where to open this link' })

    state.preferences.disabledBrowserIds = ['chrome']
    act(() => api.preferencesChangedListener(clone(state.preferences)))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Choose where to open this link' })).toBeNull())
    expect(api.dismissURLRoute).toHaveBeenCalledWith('route-stale')
    expect(api.hideApplicationWindow).toHaveBeenCalled()
  })

  test('dismisses a delayed pending URL route invalidated by browser preferences', async () => {
    let resolvePendingRoute
    let backendPendingRoute = null
    const pendingRoutePromise = new Promise((resolve) => { resolvePendingRoute = resolve })
    const { api, state } = createFakeApi()
    api.pendingURLRoute
      .mockImplementationOnce(() => pendingRoutePromise)
      .mockImplementation(async () => backendPendingRoute)
    api.dismissURLRoute.mockImplementation(async (requestId) => {
      if (backendPendingRoute?.id === requestId) backendPendingRoute = null
    })
    const firstWindow = renderApp(api)
    await waitFor(() => expect(api.preferencesChangedListener).toBeTypeOf('function'))

    state.preferences.disabledBrowserIds = ['chrome']
    act(() => api.preferencesChangedListener(clone(state.preferences)))

    const staleRoute = {
      id: 'route-delayed',
      url: 'https://example.com',
      defaultChoiceId: 'browser:chrome',
      timeoutMilliseconds: 5000,
      choices: [
        { id: 'browser:chrome', kind: 'browser', label: 'Chrome', browserId: 'chrome' },
      ],
    }
    await act(async () => {
      backendPendingRoute = staleRoute
      resolvePendingRoute(staleRoute)
      await pendingRoutePromise
    })

    expect(screen.queryByRole('dialog', { name: 'Choose where to open this link' })).toBeNull()
    expect(api.dismissURLRoute).toHaveBeenCalledWith('route-delayed')
    firstWindow.unmount()

    renderApp(api)
    await waitFor(() => expect(api.pendingURLRoute).toHaveBeenCalledTimes(2))
    expect(screen.queryByRole('dialog', { name: 'Choose where to open this link' })).toBeNull()
  })

  test('customizes a running proxy browser with a persistent icon and color', async () => {
    const user = userEvent.setup()
    const proxyBrowser = {
      id: 'browser:202',
      pid: 202,
      browserId: 'google-chrome',
      browserName: 'Google Chrome',
      kind: 'proxy',
      serverId: savedServer.id,
      serverName: savedServer.name,
    }
    const { api } = createFakeApi({ runningBrowsers: [proxyBrowser] })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Customize' }))
    await screen.findByRole('option', { name: /Google Chrome/ })
    await user.click(screen.getByRole('button', { name: 'Customize selected' }))
    await user.click(screen.getByRole('radio', { name: 'X icon' }))
    await user.click(screen.getByRole('radio', { name: 'Green color' }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.saveBrowserAppearance).toHaveBeenCalledWith(
      `proxy:${savedServer.id}:google-chrome`,
      { icon: 'icon:x', primaryColor: '#22C55E' },
    ))
    const tile = await screen.findByRole('option', { name: /Google Chrome/ })
    expect(tile.classList.contains('has-custom-appearance')).toBe(true)
    expect(tile.style.getPropertyValue('--browser-primary')).toBe('#22C55E')
  })

  test('opens and cycles the browser switcher in either requested direction', async () => {
    const browsers = [
      { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Chrome one', kind: 'regular' },
      { id: 'browser:202', pid: 202, browserId: 'google-chrome', browserName: 'Chrome two', kind: 'regular' },
      { id: 'browser:303', pid: 303, browserId: 'google-chrome', browserName: 'Chrome three', kind: 'regular' },
    ]
    const { api } = createFakeApi({ runningBrowsers: browsers })
    renderApp(api)
    await waitFor(() => expect(api.browserSwitcherListener).toBeTypeOf('function'))
    await waitFor(() => expect(api.browserSwitcherCommitListener).toBeTypeOf('function'))

    await act(async () => {
      api.browserSwitcherListener({ direction: 'backward', sessionId: 'switch-1' })
    })

    const dialog = await screen.findByRole('dialog', { name: 'Switch browser' })
    const options = within(dialog).getAllByRole('option')
    expect(within(dialog).getByRole('listbox').getAttribute('aria-orientation')).toBe('horizontal')
    expect(options[2].getAttribute('aria-selected')).toBe('true')
    expect(within(dialog).getByText('Alt+X')).toBeTruthy()
    expect(within(dialog).getByText('Alt+Z')).toBeTruthy()

    act(() => api.browserSwitcherListener({ direction: 'backward', sessionId: 'switch-1' }))
    expect(options[1].getAttribute('aria-selected')).toBe('true')

    act(() => api.browserSwitcherListener({ direction: 'forward', sessionId: 'switch-1' }))
    expect(options[2].getAttribute('aria-selected')).toBe('true')

    act(() => api.browserSwitcherListener({ direction: 'forward', sessionId: 'switch-1' }))
    expect(options[0].getAttribute('aria-selected')).toBe('true')
    expect(api.listRunningBrowsers).toHaveBeenCalledTimes(1)

    act(() => api.browserSwitcherCommitListener({ sessionId: 'switch-1' }))
    await waitFor(() => expect(api.activateRunningBrowser).toHaveBeenCalledWith('browser:101'))
    await waitFor(() => expect(api.hideApplicationWindow).toHaveBeenCalled())
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Switch browser' })).toBeNull())

    await act(async () => {
      api.browserSwitcherListener({ direction: 'forward', sessionId: 'switch-2' })
    })
    const nextDialog = await screen.findByRole('dialog', { name: 'Switch browser' })
    const nextOptions = within(nextDialog).getAllByRole('option')
    expect(nextOptions[0].textContent).toContain('Chrome one')
    expect(nextOptions[1].getAttribute('aria-selected')).toBe('true')
    expect(nextOptions[1].textContent).toContain('Chrome two')
    expect(api.listRunningBrowsers).toHaveBeenCalledTimes(2)
  })

  test('closes an open browser switcher when browser preferences change', async () => {
    const browser = { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Chrome', kind: 'regular' }
    const { api, state } = createFakeApi({ runningBrowsers: [browser] })
    renderApp(api)
    await waitFor(() => expect(api.browserSwitcherListener).toBeTypeOf('function'))
    await waitFor(() => expect(api.preferencesChangedListener).toBeTypeOf('function'))

    await act(async () => api.browserSwitcherListener({ direction: 'forward', sessionId: 'stale-browser' }))
    await screen.findByRole('option', { name: /Chrome/ })

    state.preferences.disabledBrowserIds = ['google-chrome']
    act(() => api.preferencesChangedListener(clone(state.preferences)))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Switch browser' })).toBeNull())
    act(() => api.browserSwitcherCommitListener({ sessionId: 'stale-browser' }))
    expect(api.activateRunningBrowser).not.toHaveBeenCalled()
    expect(api.hideApplicationWindow).toHaveBeenCalled()
  })

  test('clears and reloads tunnel browser choices when browser preferences change', async () => {
    const user = userEvent.setup()
    const socksTunnel = {
      ...managedBrowserProxy,
      id: 'tunnel-socks-1',
      label: 'Team proxy',
    }
    const connected = {
      ...stoppedSession,
      configurationId: socksTunnel.id,
      status: 'connected',
      boundPort: socksTunnel.socksPort,
    }
    const chrome = { id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true }
    const zen = { id: 'zen', displayName: 'Zen', supportsProxyLaunch: true }
    const { api, state } = createFakeApi({
      servers: [{ server: savedServer, configurations: [socksTunnel] }],
      sessions: [connected],
    })
    api.discoverBrowsers
      .mockResolvedValueOnce([chrome])
      .mockResolvedValueOnce([zen])
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Show Production bastion details' }))
    await user.click((await screen.findByText('Team proxy')).closest('button'))
    expect(await screen.findByRole('option', { name: 'Google Chrome' })).toBeTruthy()
    await waitFor(() => expect(api.preferencesChangedListener).toBeTypeOf('function'))

    state.preferences.disabledBrowserIds = ['google-chrome']
    act(() => api.preferencesChangedListener(clone(state.preferences)))

    expect(screen.queryByRole('option', { name: 'Google Chrome' })).toBeNull()
    expect(screen.getByText('Finding browsers…')).toBeTruthy()
    expect(await screen.findByRole('option', { name: 'Zen' })).toBeTruthy()
    expect(api.discoverBrowsers).toHaveBeenCalledTimes(2)
  })

  test('ignores tunnel browser discovery started before browser preferences changed', async () => {
    const user = userEvent.setup()
    let resolveStaleBrowsers
    const staleBrowsers = new Promise((resolve) => { resolveStaleBrowsers = resolve })
    const socksTunnel = {
      ...managedBrowserProxy,
      id: 'tunnel-socks-race',
      label: 'Race-safe proxy',
    }
    const connected = {
      ...stoppedSession,
      configurationId: socksTunnel.id,
      status: 'connected',
      boundPort: socksTunnel.socksPort,
    }
    const { api, state } = createFakeApi({
      servers: [{ server: savedServer, configurations: [socksTunnel] }],
      sessions: [connected],
    })
    api.discoverBrowsers
      .mockImplementationOnce(() => staleBrowsers)
      .mockResolvedValueOnce([{ id: 'zen', displayName: 'Zen', supportsProxyLaunch: true }])
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Show Production bastion details' }))
    await user.click((await screen.findByText('Race-safe proxy')).closest('button'))
    await waitFor(() => expect(api.discoverBrowsers).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(api.preferencesChangedListener).toBeTypeOf('function'))

    state.preferences.disabledBrowserIds = ['google-chrome']
    act(() => api.preferencesChangedListener(clone(state.preferences)))

    expect(await screen.findByRole('option', { name: 'Zen' })).toBeTruthy()
    await act(async () => {
      resolveStaleBrowsers([{ id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true }])
      await staleBrowsers
    })
    expect(screen.queryByRole('option', { name: 'Google Chrome' })).toBeNull()
  })

  test('queues shortcut directions and commit received while browsers are loading', async () => {
    let resolveBrowsers
    const browserPromise = new Promise((resolve) => { resolveBrowsers = resolve })
    const browsers = [
      { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Chrome one', kind: 'regular' },
      { id: 'browser:202', pid: 202, browserId: 'google-chrome', browserName: 'Chrome two', kind: 'regular' },
      { id: 'browser:303', pid: 303, browserId: 'google-chrome', browserName: 'Chrome three', kind: 'regular' },
    ]
    const { api } = createFakeApi()
    api.listRunningBrowsers.mockImplementation(() => browserPromise)
    renderApp(api)
    await waitFor(() => expect(api.browserSwitcherListener).toBeTypeOf('function'))
    await waitFor(() => expect(api.browserSwitcherCommitListener).toBeTypeOf('function'))

    act(() => {
      api.browserSwitcherListener({ direction: 'backward', sessionId: 'switch-loading' })
      api.browserSwitcherListener({ direction: 'backward', sessionId: 'switch-loading' })
      api.browserSwitcherCommitListener({ sessionId: 'switch-loading' })
    })
    await screen.findByRole('dialog', { name: 'Switch browser' })

    await act(async () => {
      resolveBrowsers(browsers)
      await browserPromise
    })

    await waitFor(() => expect(api.activateRunningBrowser).toHaveBeenCalledWith('browser:202'))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Switch browser' })).toBeNull())
    expect(api.listRunningBrowsers).toHaveBeenCalledTimes(1)
  })

  test('ignores stale native commit and cancel events, and native cancel does not hide again', async () => {
    const browser = { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Chrome', kind: 'regular' }
    const { api } = createFakeApi({ runningBrowsers: [browser] })
    renderApp(api)
    await waitFor(() => expect(api.browserSwitcherCancelListener).toBeTypeOf('function'))
    await act(async () => api.browserSwitcherListener({ direction: 'forward', sessionId: 'current' }))
    await screen.findByRole('dialog', { name: 'Switch browser' })

    act(() => {
      api.browserSwitcherCommitListener({ sessionId: 'stale' })
      api.browserSwitcherCancelListener({ sessionId: 'stale' })
    })
    expect(screen.getByRole('dialog', { name: 'Switch browser' })).toBeTruthy()
    expect(api.activateRunningBrowser).not.toHaveBeenCalled()

    act(() => api.browserSwitcherCancelListener({ sessionId: 'current' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Switch browser' })).toBeNull())
    expect(api.hideApplicationWindow).not.toHaveBeenCalled()
  })

  test('ignores browser discovery that resolves after a newer session', async () => {
    let resolveStaleBrowsers
    const staleBrowserPromise = new Promise((resolve) => { resolveStaleBrowsers = resolve })
    const staleBrowser = { id: 'browser:old', pid: 101, browserId: 'google-chrome', browserName: 'Old Chrome', kind: 'regular' }
    const currentBrowser = { id: 'browser:new', pid: 202, browserId: 'google-chrome', browserName: 'Current Chrome', kind: 'regular' }
    const { api } = createFakeApi()
    api.listRunningBrowsers
      .mockImplementationOnce(() => staleBrowserPromise)
      .mockResolvedValueOnce([currentBrowser])
    renderApp(api)
    await waitFor(() => expect(api.browserSwitcherListener).toBeTypeOf('function'))

    act(() => api.browserSwitcherListener({ direction: 'forward', sessionId: 'stale-load' }))
    await screen.findByRole('dialog', { name: 'Switch browser' })
    await act(async () => api.browserSwitcherListener({ direction: 'forward', sessionId: 'current-load' }))
    await screen.findByText('Current Chrome')

    await act(async () => {
      resolveStaleBrowsers([staleBrowser])
      await staleBrowserPromise
    })
    expect(screen.queryByText('Old Chrome')).toBeNull()
    expect(screen.getByText('Current Chrome')).toBeTruthy()
  })

  test('ignores activation completion after a newer session takes over', async () => {
    let resolveStaleActivation
    const staleActivationPromise = new Promise((resolve) => { resolveStaleActivation = resolve })
    const browsers = [
      { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Chrome one', kind: 'regular' },
      { id: 'browser:202', pid: 202, browserId: 'google-chrome', browserName: 'Chrome two', kind: 'regular' },
    ]
    const { api } = createFakeApi({ runningBrowsers: browsers })
    api.activateRunningBrowser.mockImplementationOnce(() => staleActivationPromise)
    renderApp(api)
    await waitFor(() => expect(api.browserSwitcherCommitListener).toBeTypeOf('function'))

    await act(async () => api.browserSwitcherListener({ direction: 'forward', sessionId: 'stale-activation' }))
    act(() => api.browserSwitcherCommitListener({ sessionId: 'stale-activation' }))
    await waitFor(() => expect(api.activateRunningBrowser).toHaveBeenCalledWith('browser:101'))

    await act(async () => api.browserSwitcherListener({ direction: 'forward', sessionId: 'current-session' }))
    const currentDialog = await screen.findByRole('dialog', { name: 'Switch browser' })
    expect(within(currentDialog).getAllByRole('option')[0].getAttribute('aria-selected')).toBe('true')

    await act(async () => {
      resolveStaleActivation()
      await staleActivationPromise
    })
    expect(screen.getByRole('dialog', { name: 'Switch browser' })).toBeTruthy()
    expect(api.hideApplicationWindow).not.toHaveBeenCalled()
  })

  test('cancels browser switching with Escape without activating a browser', async () => {
    const browser = { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Chrome', kind: 'regular' }
    const { api } = createFakeApi({ runningBrowsers: [browser] })
    renderApp(api)
    await waitFor(() => expect(api.browserSwitcherListener).toBeTypeOf('function'))
    await act(async () => api.browserSwitcherListener('forward'))
    const dialog = await screen.findByRole('dialog', { name: 'Switch browser' })

    expect(within(dialog).getByText('Release the shortcut modifier to activate')).toBeTruthy()
    expect(within(dialog).queryByText('Enter')).toBeNull()
    fireEvent.keyDown(window, { key: 'Escape' })

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Switch browser' })).toBeNull())
    expect(api.activateRunningBrowser).not.toHaveBeenCalled()
    expect(api.hideApplicationWindow).toHaveBeenCalledTimes(1)
  })

  test('clears a manual browser switcher when its native window loses focus', async () => {
    const browser = { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Chrome', kind: 'regular' }
    const { api } = createFakeApi({ runningBrowsers: [browser] })
    const user = userEvent.setup()
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Customize' }))
    await screen.findByRole('dialog', { name: 'Switch browser' })

    fireEvent.blur(window)

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Switch browser' })).toBeNull())
    expect(api.hideApplicationWindow).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Customize' })).toBeTruthy()

    fireEvent.focus(window)
    expect(screen.queryByRole('dialog', { name: 'Switch browser' })).toBeNull()
  })

  test('does not cancel a manual activation when activating the browser blurs the window', async () => {
    let resolveActivation
    const activationPromise = new Promise((resolve) => { resolveActivation = resolve })
    const browser = { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Chrome', kind: 'regular' }
    const { api } = createFakeApi({ runningBrowsers: [browser] })
    api.activateRunningBrowser.mockImplementation(() => activationPromise)
    const user = userEvent.setup()
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Customize' }))
    const dialog = await screen.findByRole('dialog', { name: 'Switch browser' })
    const option = within(dialog).getByRole('option')
    await user.click(option)
    await waitFor(() => expect(api.activateRunningBrowser).toHaveBeenCalledWith('browser:101'))

    fireEvent.blur(window)
    expect(screen.getByRole('dialog', { name: 'Switch browser' })).toBeTruthy()

    await act(async () => {
      resolveActivation()
      await activationPromise
    })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Switch browser' })).toBeNull())
    expect(api.hideApplicationWindow).not.toHaveBeenCalled()
  })

  test('switches between labeled proxy and regular browser instances', async () => {
    const user = userEvent.setup()
    const proxy = { id: 'browser:202', pid: 202, browserId: 'google-chrome', browserName: 'Google Chrome', kind: 'proxy', serverId: 'server-prod', serverName: 'Production' }
    const regular = { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Google Chrome', kind: 'regular' }
    const { api } = createFakeApi({ runningBrowsers: [proxy, regular] })
    renderSettingsApp(api)

    await user.click(await screen.findByRole('button', { name: 'Customize' }))

    const dialog = await screen.findByRole('dialog', { name: 'Switch browser' })
    expect(within(dialog).getByText('Click a browser or select it with the arrow keys and press Enter.')).toBeTruthy()
    expect(within(dialog).getByText('Click a tile or press', { exact: false })).toBeTruthy()
    expect(within(dialog).queryByText('Release the shortcut modifier to activate')).toBeNull()
    expect(within(dialog).getByText('Production proxy')).toBeTruthy()
    expect(within(dialog).getByText('Regular browser')).toBeTruthy()
    const options = within(dialog).getAllByRole('option')
    expect(options[0].getAttribute('aria-selected')).toBe('true')

    await user.click(options[1])
    await waitFor(() => expect(api.activateRunningBrowser).toHaveBeenCalledWith('browser:101'))
    expect(api.hideApplicationWindow).not.toHaveBeenCalled()
  })
})
