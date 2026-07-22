import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'
import App from './App'

const savedServer = {
  id: 'server-1',
  name: 'Production bastion',
  host: 'bastion.example.com',
  port: 22,
  username: 'deploy',
  authMode: 'agent',
  keyReference: '',
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
} = {}) {
  const state = {
    servers: clone(servers),
    sessions: clone(sessions),
    history: clone(history),
    preferences: {
      theme: 'dark',
      lastSelectedServerId: '',
      browserSwitcherShortcut: 'Alt+X',
      browserSwitcherBackwardShortcut: 'Alt+Z',
      browserAppearances: {},
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
      },
      currentUsername,
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
    previewBrowserLaunchThroughSocks: vi.fn(async () => ({ command: '' })),
    launchBrowserThroughSocks: vi.fn(async () => ({ success: true })),
    listRunningBrowsers: vi.fn(async () => clone(runningBrowsers)),
    activateRunningBrowser: vi.fn(async () => undefined),
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
    hideApplicationWindow: vi.fn(async () => undefined),
    quitApplication: vi.fn(async () => undefined),
    openExternalURL: vi.fn(async () => undefined),
  }

  return { api, state }
}

function renderApp(api, controllerOptions = {}) {
  return render(<App api={api} controllerOptions={{ pollMs: 60_000, copyText: vi.fn(), ...controllerOptions }} />)
}

async function openSavedTunnel(user, api) {
  renderApp(api)
  await user.click(await screen.findByRole('button', { name: /Production bastion/ }))
  const tunnelLabel = await screen.findByText('Admin database')
  await user.click(tunnelLabel.closest('button'))
  await screen.findByRole('button', { name: 'Start tunnel' })
}

describe('React application flows', () => {
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
    })
    expect(screen.getByText('deploy@staging.example.com:2222')).toBeTruthy()
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

    await user.click(await screen.findByRole('button', { name: /Production bastion/ }))
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

    await user.click(await screen.findByRole('button', { name: /Production bastion/ }))
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

    await user.click(await screen.findByRole('button', { name: /Production bastion/ }))
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

    await user.click(await screen.findByRole('button', { name: /Production bastion/ }))
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

    await user.click(await screen.findByRole('button', { name: /Production bastion/ }))
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
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Settings' }))
    const nextRecorder = screen.getByRole('button', { name: 'Next browser shortcut' })
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
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Settings' }))
    await user.click(screen.getByRole('button', { name: 'Customize' }))
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
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Settings' }))
    await user.click(screen.getByRole('button', { name: 'Customize' }))
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
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Settings' }))
    await user.click(screen.getByRole('button', { name: 'Customize' }))
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
    expect(api.hideApplicationWindow).toHaveBeenCalledTimes(1)
  })

  test('switches between labeled proxy and regular browser instances', async () => {
    const user = userEvent.setup()
    const proxy = { id: 'browser:202', pid: 202, browserId: 'google-chrome', browserName: 'Google Chrome', kind: 'proxy', serverId: 'server-prod', serverName: 'Production' }
    const regular = { id: 'browser:101', pid: 101, browserId: 'google-chrome', browserName: 'Google Chrome', kind: 'regular' }
    const { api } = createFakeApi({ runningBrowsers: [proxy, regular] })
    renderApp(api)

    await user.click(await screen.findByRole('button', { name: 'Settings' }))
    await user.click(screen.getByRole('button', { name: 'Customize' }))

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
    expect(api.hideApplicationWindow).toHaveBeenCalled()
  })
})
