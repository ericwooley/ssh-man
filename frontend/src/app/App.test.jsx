import { render, screen, waitFor, within } from '@testing-library/react'
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
} = {}) {
  const state = {
    servers: clone(servers),
    sessions: clone(sessions),
    history: clone(history),
    preferences: { theme: 'dark', lastSelectedServerId: '' },
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
})
