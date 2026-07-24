import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'
import CommandApp from './CommandApp'

const server = {
  id: 'server-1',
  name: 'Production',
  host: 'prod.example.com',
  port: 22,
  username: 'deploy',
}

const savedEntry = {
  id: 'history-1',
  serverId: server.id,
  command: 'pwd',
  output: '/srv/app\n',
  exitCode: 0,
  startedAt: '2026-07-24T12:00:00.000Z',
  endedAt: '2026-07-24T12:00:00.250Z',
  truncated: false,
  error: '',
}

function createApi({ history = [savedEntry], connect = { connected: true, homePath: '/home/deploy' } } = {}) {
  const nextEntry = {
    ...savedEntry,
    id: 'history-2',
    command: 'cat src/server.go',
    output: 'package main\n',
    startedAt: '2026-07-24T12:01:00.000Z',
    endedAt: '2026-07-24T12:01:01.000Z',
  }
  return {
    initialState: vi.fn(async () => ({ server, history })),
    connect: vi.fn(async () => connect),
    completePath: vi.fn(async () => ({
      items: [
        { value: 'src/services/', name: 'services/', kind: 'directory' },
        { value: 'src/server.go', name: 'server.go', kind: 'file' },
      ],
    })),
    runCommand: vi.fn(async () => nextEntry),
    cancelCommand: vi.fn(async () => undefined),
    deleteHistory: vi.fn(async () => undefined),
    close: vi.fn(async () => undefined),
  }
}

describe('quick command window', () => {
  test('loads saved prompt/output history and copies output', async () => {
    const user = userEvent.setup()
    const api = createApi()
    const copyText = vi.fn(async () => undefined)
    render(<CommandApp api={api} copyText={copyText} />)

    expect(await screen.findByRole('heading', { name: 'Production' })).toBeTruthy()
    expect(screen.getByText('/srv/app')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Copy command output' }))
    expect(copyText).toHaveBeenCalledWith('/srv/app\n')
  })

  test('uses remote file completion and saves the finished command in history', async () => {
    const user = userEvent.setup()
    const api = createApi({ history: [] })
    render(<CommandApp api={api} copyText={vi.fn()} />)
    const input = await screen.findByLabelText('Command')

    await user.type(input, 'cat src/se')
    const suggestions = await screen.findByRole('listbox', { name: 'Remote file suggestions' })
    expect(api.completePath).toHaveBeenLastCalledWith('src/se')
    fireEvent.keyDown(input, { key: 'Tab' })
    expect(input.value).toBe('cat src/services/')

    await user.clear(input)
    await user.type(input, 'cat src/server.go{Enter}')
    await waitFor(() => expect(api.runCommand).toHaveBeenCalledWith('cat src/server.go'))
    expect(await screen.findByText('package main')).toBeTruthy()
    expect(within(screen.getByLabelText('Command history')).getByText('cat src/server.go')).toBeTruthy()
    expect(suggestions).toBeTruthy()
  })

  test('deletes a selected prompt and its output from saved history', async () => {
    const user = userEvent.setup()
    const api = createApi()
    render(<CommandApp api={api} copyText={vi.fn()} />)

    await screen.findByText('/srv/app')
    await user.click(screen.getByRole('button', { name: 'Delete selected command history' }))

    expect(api.deleteHistory).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: 'Delete pwd?' })).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(api.deleteHistory).not.toHaveBeenCalled()
    expect(screen.queryByRole('heading', { name: 'Delete pwd?' })).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Delete selected command history' }))
    await user.click(screen.getByRole('button', { name: 'Delete command' }))

    await waitFor(() => expect(api.deleteHistory).toHaveBeenCalledWith(savedEntry.id))
    expect(screen.queryByText('/srv/app')).toBeNull()
    expect(screen.getByText('Ready for a command')).toBeTruthy()
  })

  test('prompts for an encrypted key before enabling commands', async () => {
    const user = userEvent.setup()
    const api = createApi({ connect: { needsPassphrase: true } })
    api.connect
      .mockResolvedValueOnce({ needsPassphrase: true })
      .mockResolvedValueOnce({ connected: true, homePath: '/home/deploy' })
    render(<CommandApp api={api} copyText={vi.fn()} />)

    await user.type(await screen.findByLabelText('Passphrase'), 'secret')
    await user.click(screen.getByRole('button', { name: 'Unlock and connect' }))

    await waitFor(() => expect(api.connect).toHaveBeenLastCalledWith('secret'))
    expect(await screen.findByLabelText('Command')).toBeTruthy()
  })
})
