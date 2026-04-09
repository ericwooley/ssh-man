import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { api } = vi.hoisted(() => ({
  api: {
    loadInitialState: vi.fn(),
    saveServer: vi.fn(),
    deleteServer: vi.fn(),
    saveConnectionConfiguration: vi.fn(),
    deleteConnectionConfiguration: vi.fn(),
    savePreferences: vi.fn(),
    startConfiguration: vi.fn(),
    startServerConfigurations: vi.fn(),
    stopConfiguration: vi.fn(),
    retryConfiguration: vi.fn(),
    submitKeyUnlock: vi.fn(),
    discoverBrowsers: vi.fn(),
    launchBrowserThroughSocks: vi.fn(),
    previewBrowserLaunchThroughSocks: vi.fn(),
    openDevTools: vi.fn(),
    listRuntimeSessions: vi.fn(),
    listSessionHistory: vi.fn(),
  },
}))

vi.mock('../lib/api', () => api)

import App from './App.svelte'

describe('App', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    api.loadInitialState.mockResolvedValue({
      servers: [],
      preferences: { theme: 'dark', lastSelectedServerId: '' },
      sessions: [],
      diagnostics: { appDataPath: '/tmp/ssh-man', databasePath: '/tmp/ssh-man/ssh-man.db' },
      message: '',
      recoverable: false,
    })
    api.saveServer.mockImplementation(async (server) => ({ ...server, id: server.id || 'server-1' }))
    api.deleteServer.mockResolvedValue(undefined)
    api.saveConnectionConfiguration.mockImplementation(async (config) => ({ ...config, id: config.id || 'config-1' }))
    api.deleteConnectionConfiguration.mockResolvedValue(undefined)
    api.savePreferences.mockImplementation(async (preferences) => preferences)
    api.startConfiguration.mockResolvedValue(undefined)
    api.startServerConfigurations.mockResolvedValue([])
    api.stopConfiguration.mockResolvedValue(undefined)
    api.retryConfiguration.mockResolvedValue(undefined)
    api.submitKeyUnlock.mockResolvedValue(undefined)
    api.discoverBrowsers.mockResolvedValue([])
    api.launchBrowserThroughSocks.mockResolvedValue(undefined)
    api.previewBrowserLaunchThroughSocks.mockResolvedValue({ command: 'google-chrome --proxy-server=socks5://127.0.0.1:1080' })
    api.openDevTools.mockResolvedValue(undefined)
    api.listRuntimeSessions.mockResolvedValue([])
    api.listSessionHistory.mockResolvedValue([])

    Object.assign(navigator, {
      clipboard: { writeText: vi.fn(async () => {}) },
    })
  })

  it('opens the server dialog from the add button', async () => {
    render(App)

    await fireEvent.click(await screen.findByRole('button', { name: 'Add server' }))

    expect(screen.getByRole('dialog', { name: 'New server' })).toBeTruthy()
  })

  it('selects the saved server and unlocks the workspace after saving', async () => {
    render(App)

    await fireEvent.click(await screen.findByRole('button', { name: 'Add server' }))

    const dialog = screen.getByRole('dialog', { name: 'New server' })
    await fireEvent.input(within(dialog).getByLabelText('Server name'), { target: { value: 'Test server' } })
    await fireEvent.input(within(dialog).getByLabelText('Server host'), { target: { value: 'example.com' } })
    await fireEvent.input(within(dialog).getByLabelText('SSH username'), { target: { value: 'eric' } })
    await fireEvent.input(within(dialog).getByLabelText('Private key path'), { target: { value: '~/.ssh/id_ed25519' } })
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Save server' }))

    expect(screen.getByRole('heading', { name: 'Test server' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Add tunnel' }).hasAttribute('disabled')).toBe(false)
    expect(screen.queryByRole('heading', { name: 'Select a server to continue' })).toBeNull()
  })

  it('shows a saved tunnel in the tunnel list', async () => {
    render(App)

    await fireEvent.click(await screen.findByRole('button', { name: 'Add server' }))

    const serverDialog = screen.getByRole('dialog', { name: 'New server' })
    await fireEvent.input(within(serverDialog).getByLabelText('Server name'), { target: { value: 'Test server' } })
    await fireEvent.input(within(serverDialog).getByLabelText('Server host'), { target: { value: 'example.com' } })
    await fireEvent.input(within(serverDialog).getByLabelText('SSH username'), { target: { value: 'eric' } })
    await fireEvent.input(within(serverDialog).getByLabelText('Private key path'), { target: { value: '~/.ssh/id_ed25519' } })
    await fireEvent.click(within(serverDialog).getByRole('button', { name: 'Save server' }))

    await fireEvent.click(screen.getByRole('button', { name: 'Add tunnel' }))

    const tunnelDialog = screen.getByRole('dialog', { name: 'Tunnel editor' })
    await fireEvent.input(within(tunnelDialog).getByLabelText('Label'), { target: { value: 'Docs SOCKS' } })
    await fireEvent.change(within(tunnelDialog).getByLabelText('Type'), { target: { value: 'socks_proxy' } })
    await fireEvent.change(within(tunnelDialog).getByLabelText('SOCKS port mode'), { target: { value: 'manual' } })
    await fireEvent.input(within(tunnelDialog).getByLabelText('SOCKS port'), { target: { value: '1080' } })
    await fireEvent.click(within(tunnelDialog).getByRole('button', { name: 'Save tunnel' }))

    expect(screen.getByRole('button', { name: 'Select tunnel Docs SOCKS' })).toBeTruthy()
    expect(screen.queryByText('No saved tunnels')).toBeNull()
  })

  it('saves a SOCKS tunnel with auto port mode', async () => {
    render(App)

    await fireEvent.click(await screen.findByRole('button', { name: 'Add server' }))

    const serverDialog = screen.getByRole('dialog', { name: 'New server' })
    await fireEvent.input(within(serverDialog).getByLabelText('Server name'), { target: { value: 'Test server' } })
    await fireEvent.input(within(serverDialog).getByLabelText('Server host'), { target: { value: 'example.com' } })
    await fireEvent.input(within(serverDialog).getByLabelText('SSH username'), { target: { value: 'eric' } })
    await fireEvent.input(within(serverDialog).getByLabelText('Private key path'), { target: { value: '~/.ssh/id_ed25519' } })
    await fireEvent.click(within(serverDialog).getByRole('button', { name: 'Save server' }))

    await fireEvent.click(screen.getByRole('button', { name: 'Add tunnel' }))

    const tunnelDialog = screen.getByRole('dialog', { name: 'Tunnel editor' })
    await fireEvent.input(within(tunnelDialog).getByLabelText('Label'), { target: { value: 'Auto SOCKS' } })
    await fireEvent.change(within(tunnelDialog).getByLabelText('Type'), { target: { value: 'socks_proxy' } })
    await fireEvent.change(within(tunnelDialog).getByLabelText('SOCKS port mode'), { target: { value: 'auto' } })
    await fireEvent.click(within(tunnelDialog).getByRole('button', { name: 'Save tunnel' }))

    expect(api.saveConnectionConfiguration).toHaveBeenCalledWith(expect.objectContaining({ socksPort: 0 }))
    expect(screen.getByText('SOCKS Auto')).toBeTruthy()
  })

  it('refreshes runtime state after start and stop so all views stay in sync', async () => {
    api.loadInitialState.mockResolvedValue({
      servers: [{
        server: { id: 'server-1', name: 'Primary', host: 'example.com', port: 22, username: 'eric', authMode: 'private_key', keyReference: '~/.ssh/id_ed25519' },
        configurations: [{ id: 'config-1', serverId: 'server-1', label: 'Docs SOCKS', connectionType: 'socks_proxy', socksPort: 1080, autoReconnectEnabled: true }],
      }],
      preferences: { theme: 'dark', lastSelectedServerId: 'server-1' },
      sessions: [],
      diagnostics: { appDataPath: '/tmp/ssh-man', databasePath: '/tmp/ssh-man/ssh-man.db' },
      message: '',
      recoverable: false,
    })

    api.startConfiguration.mockResolvedValue({ configurationId: 'config-1', status: 'starting', statusDetail: 'Starting tunnel' })
    api.stopConfiguration.mockResolvedValue({ configurationId: 'config-1', status: 'stopped', statusDetail: 'Tunnel stopped' })
    api.listRuntimeSessions.mockResolvedValue([{ configurationId: 'config-1', status: 'connected', statusDetail: 'Listening on localhost:1080' }])

    render(App)

    await fireEvent.click(await screen.findByRole('button', { name: 'Start tunnel' }))

    await waitFor(() => {
      expect(screen.getByLabelText('Tunnel status connected')).toBeTruthy()
      expect(screen.getByLabelText('Session status connected')).toBeTruthy()
      expect(screen.getAllByText('Listening on localhost:1080').length).toBeGreaterThan(0)
    })

    api.listRuntimeSessions.mockResolvedValue([{ configurationId: 'config-1', status: 'stopped', statusDetail: 'Tunnel stopped' }])

    await fireEvent.click(screen.getByRole('button', { name: 'Stop tunnel' }))

    await waitFor(() => {
      expect(screen.getByLabelText('Tunnel status stopped')).toBeTruthy()
      expect(screen.getByLabelText('Session status stopped')).toBeTruthy()
      expect(screen.queryByText('Listening on localhost:1080')).toBeNull()
    })
  })

  it('shows the runtime assigned port for auto SOCKS tunnels after start', async () => {
    api.loadInitialState.mockResolvedValue({
      servers: [{
        server: { id: 'server-1', name: 'Primary', host: 'example.com', port: 22, username: 'eric', authMode: 'private_key', keyReference: '~/.ssh/id_ed25519' },
        configurations: [{ id: 'config-1', serverId: 'server-1', label: 'Auto SOCKS', connectionType: 'socks_proxy', socksPort: 0, autoReconnectEnabled: true }],
      }],
      preferences: { theme: 'dark', lastSelectedServerId: 'server-1' },
      sessions: [],
      diagnostics: { appDataPath: '/tmp/ssh-man', databasePath: '/tmp/ssh-man/ssh-man.db' },
      message: '',
      recoverable: false,
    })

    api.startConfiguration.mockResolvedValue({ configurationId: 'config-1', status: 'starting', statusDetail: 'Starting tunnel' })
    api.listRuntimeSessions.mockResolvedValue([{ configurationId: 'config-1', status: 'connected', boundPort: 43123, statusDetail: 'Listening on localhost:43123' }])

    render(App)

    expect(await screen.findByText('SOCKS Auto')).toBeTruthy()

    await fireEvent.click(screen.getByRole('button', { name: 'Start tunnel' }))

    await waitFor(() => {
      expect(screen.getByText('SOCKS proxy on :43123')).toBeTruthy()
      expect(screen.getAllByText('Listening on localhost:43123').length).toBeGreaterThan(0)
    })
  })

  it('discovers browsers automatically for a selected socks tunnel and shows the launch command preview', async () => {
    api.loadInitialState.mockResolvedValue({
      servers: [{
        server: { id: 'server-1', name: 'Primary', host: 'example.com', port: 22, username: 'eric', authMode: 'private_key', keyReference: '~/.ssh/id_ed25519' },
        configurations: [{ id: 'config-1', serverId: 'server-1', label: 'Docs SOCKS', connectionType: 'socks_proxy', socksPort: 1080, autoReconnectEnabled: true }],
      }],
      preferences: { theme: 'dark', lastSelectedServerId: 'server-1' },
      sessions: [{ configurationId: 'config-1', status: 'connected', boundPort: 1080, statusDetail: 'Listening on localhost:1080' }],
      diagnostics: { appDataPath: '/tmp/ssh-man', databasePath: '/tmp/ssh-man/ssh-man.db' },
      message: '',
      recoverable: false,
    })
    api.discoverBrowsers.mockResolvedValue([{ id: 'google-chrome', displayName: 'Google Chrome', supportsProxyLaunch: true }])
    api.previewBrowserLaunchThroughSocks.mockResolvedValue({ command: 'google-chrome --proxy-server=socks5://127.0.0.1:1080' })

    render(App)

    await waitFor(() => {
      expect(api.discoverBrowsers).toHaveBeenCalled()
      expect(api.previewBrowserLaunchThroughSocks).toHaveBeenCalledWith('config-1', 'google-chrome')
      expect(screen.getByText((content) => content.includes('google-chrome --proxy-server=socks5://127.0.0.1:1080'))).toBeTruthy()
    })
  })

  it('unlocks all start-all tunnels that need the same SSH key passphrase', async () => {
    api.saveConnectionConfiguration
      .mockImplementationOnce(async (config) => ({ ...config, id: 'config-1' }))
      .mockImplementationOnce(async (config) => ({ ...config, id: 'config-2' }))
    api.startServerConfigurations.mockResolvedValue([
      { configurationId: 'config-1', status: 'needs_attention', statusDetail: 'Unlock the SSH key to continue' },
      { configurationId: 'config-2', status: 'needs_attention', statusDetail: 'Unlock the SSH key to continue' },
    ])
    api.submitKeyUnlock
      .mockResolvedValueOnce({ configurationId: 'config-1', status: 'connected', boundPort: 1080, statusDetail: 'Listening on localhost:1080' })
      .mockResolvedValueOnce({ configurationId: 'config-2', status: 'connected', statusDetail: 'Listening on localhost:9000' })
    api.listRuntimeSessions.mockResolvedValue([
      { configurationId: 'config-1', status: 'connected', boundPort: 1080, statusDetail: 'Listening on localhost:1080' },
      { configurationId: 'config-2', status: 'connected', statusDetail: 'Listening on localhost:9000' },
    ])

    render(App)

    await fireEvent.click(await screen.findByRole('button', { name: 'Add server' }))

    const serverDialog = screen.getByRole('dialog', { name: 'New server' })
    await fireEvent.input(within(serverDialog).getByLabelText('Server name'), { target: { value: 'Primary' } })
    await fireEvent.input(within(serverDialog).getByLabelText('Server host'), { target: { value: 'example.com' } })
    await fireEvent.input(within(serverDialog).getByLabelText('SSH username'), { target: { value: 'eric' } })
    await fireEvent.input(within(serverDialog).getByLabelText('Private key path'), { target: { value: '~/.ssh/id_ed25519' } })
    await fireEvent.click(within(serverDialog).getByRole('button', { name: 'Save server' }))

    await fireEvent.click(screen.getByRole('button', { name: 'Add tunnel' }))

    const socksDialog = screen.getByRole('dialog', { name: 'Tunnel editor' })
    await fireEvent.input(within(socksDialog).getByLabelText('Label'), { target: { value: 'Docs SOCKS' } })
    await fireEvent.change(within(socksDialog).getByLabelText('Type'), { target: { value: 'socks_proxy' } })
    await fireEvent.change(within(socksDialog).getByLabelText('SOCKS port mode'), { target: { value: 'manual' } })
    await fireEvent.input(within(socksDialog).getByLabelText('SOCKS port'), { target: { value: '1080' } })
    await fireEvent.click(within(socksDialog).getByRole('button', { name: 'Save tunnel' }))

    await fireEvent.click(screen.getByRole('button', { name: 'Add tunnel' }))

    const portDialog = screen.getByRole('dialog', { name: 'Tunnel editor' })
    await fireEvent.input(within(portDialog).getByLabelText('Label'), { target: { value: 'Docs Port' } })
    await fireEvent.input(within(portDialog).getByLabelText('Local port'), { target: { value: '9000' } })
    await fireEvent.input(within(portDialog).getByLabelText('Remote host'), { target: { value: '127.0.0.1' } })
    await fireEvent.input(within(portDialog).getByLabelText('Remote port'), { target: { value: '5432' } })
    await fireEvent.click(within(portDialog).getByRole('button', { name: 'Save tunnel' }))

    await fireEvent.click(screen.getByRole('button', { name: 'Start all' }))

    const unlockDialog = await screen.findByRole('dialog', { name: 'SSH key passphrase required' })
    await fireEvent.input(within(unlockDialog).getByLabelText('SSH key passphrase'), { target: { value: 'hunter2' } })
    await fireEvent.click(within(unlockDialog).getByRole('button', { name: 'Unlock key' }))

    await waitFor(() => {
      expect(api.submitKeyUnlock).toHaveBeenCalledTimes(2)
      expect(api.submitKeyUnlock).toHaveBeenNthCalledWith(1, 'config-1', 'hunter2')
      expect(api.submitKeyUnlock).toHaveBeenNthCalledWith(2, 'config-2', 'hunter2')
      expect(screen.getByText('Unlocked and started 2 tunnels.')).toBeTruthy()
    })
  })

  it('loads and copies connection history for the selected tunnel', async () => {
    api.loadInitialState.mockResolvedValue({
      servers: [{
        server: { id: 'server-1', name: 'Primary', host: 'example.com', port: 22, username: 'eric', authMode: 'private_key', keyReference: '~/.ssh/id_ed25519' },
        configurations: [{ id: 'config-1', serverId: 'server-1', label: 'Docs SOCKS', connectionType: 'socks_proxy', socksPort: 1080, autoReconnectEnabled: true }],
      }],
      preferences: { theme: 'dark', lastSelectedServerId: 'server-1' },
      sessions: [{ configurationId: 'config-1', status: 'connected', boundPort: 1080, statusDetail: 'Listening on localhost:1080' }],
      diagnostics: { appDataPath: '/tmp/ssh-man', databasePath: '/tmp/ssh-man/ssh-man.db' },
      message: '',
      recoverable: false,
    })
    api.listSessionHistory.mockResolvedValue([
      { id: 'entry-1', configurationId: 'config-1', outcome: 'connected', endedAt: '2026-04-09T12:00:00Z', message: 'Listening on localhost:1080' },
    ])

    render(App)

    await waitFor(() => {
      expect(api.listSessionHistory).toHaveBeenCalledWith('config-1')
      expect(screen.getAllByText('Listening on localhost:1080').length).toBeGreaterThan(0)
      expect(screen.getByRole('button', { name: 'Copy history' }).hasAttribute('disabled')).toBe(false)
    })

    const browserHeading = screen.getByRole('heading', { name: 'SOCKS-aware browser' })
    const historyHeading = screen.getByRole('heading', { name: 'Recent connection history' })
    expect(browserHeading.compareDocumentPosition(historyHeading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

    await fireEvent.click(screen.getByRole('button', { name: 'Copy history' }))

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('[2026-04-09T12:00:00Z] connected: Listening on localhost:1080')
  })
})
