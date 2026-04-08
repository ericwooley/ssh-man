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
    openDevTools: vi.fn(),
    listRuntimeSessions: vi.fn(),
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
    api.openDevTools.mockResolvedValue(undefined)
    api.listRuntimeSessions.mockResolvedValue([])

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
    await fireEvent.input(within(tunnelDialog).getByLabelText('SOCKS port'), { target: { value: '1080' } })
    await fireEvent.click(within(tunnelDialog).getByRole('button', { name: 'Save tunnel' }))

    expect(screen.getByRole('button', { name: 'Select tunnel Docs SOCKS' })).toBeTruthy()
    expect(screen.queryByText('No saved tunnels')).toBeNull()
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
})
