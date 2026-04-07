import { fireEvent, render, screen, within } from '@testing-library/svelte'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../lib/api', () => ({
  loadInitialState: vi.fn(async () => ({
    servers: [],
    preferences: { theme: 'dark', lastSelectedServerId: '' },
    sessions: [],
    diagnostics: { appDataPath: '/tmp/ssh-man', databasePath: '/tmp/ssh-man/ssh-man.db' },
    message: '',
    recoverable: false,
  })),
  saveServer: vi.fn(async (server) => ({ ...server, id: server.id || 'server-1' })),
  deleteServer: vi.fn(async () => {}),
  saveConnectionConfiguration: vi.fn(async (config) => ({ ...config, id: config.id || 'config-1' })),
  deleteConnectionConfiguration: vi.fn(async () => {}),
  savePreferences: vi.fn(async (preferences) => preferences),
  startConfiguration: vi.fn(),
  startServerConfigurations: vi.fn(async () => []),
  stopConfiguration: vi.fn(),
  retryConfiguration: vi.fn(),
  submitKeyUnlock: vi.fn(),
  discoverBrowsers: vi.fn(async () => []),
  launchBrowserThroughSocks: vi.fn(),
  openDevTools: vi.fn(),
}))

import App from './App.svelte'

describe('App', () => {
  beforeEach(() => {
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
})
