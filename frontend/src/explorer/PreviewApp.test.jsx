import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'
import PreviewApp from './PreviewApp'

vi.mock('./MonacoPreview', () => ({
  default: ({ content, label }) => <textarea aria-label={label} value={content} readOnly />,
}))

function fakeApi() {
  return {
    initialState: vi.fn(async () => ({
      server: { id: 'server-1', name: 'Production', username: 'deploy', host: 'prod.example.com' },
      remotePath: '/home/deploy/README.md',
    })),
    connect: vi.fn(async () => ({ connected: true, homePath: '/home/deploy' })),
    previewFile: vi.fn(async () => ({
      path: '/home/deploy/README.md',
      name: 'README.md',
      kind: 'markdown',
      mimeType: 'text/markdown',
      content: '# Production\n\nRemote notes.',
      size: 17,
      revision: 'revision-1',
    })),
    fullscreenState: vi.fn(async () => false),
    toggleFullscreen: vi.fn(async () => true),
  }
}

describe('preview window', () => {
  test('renders the remote preview and toggles native fullscreen', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<PreviewApp api={api} />)

    expect(await screen.findByRole('heading', { name: 'Production', level: 1 })).toBeTruthy()
    expect(screen.getByText(/Production · \/home\/deploy\/README\.md/)).toBeTruthy()
    expect(screen.getByText('Read only')).toBeTruthy()

    await user.click(screen.getByRole('button', { name: 'Enter fullscreen' }))

    expect(api.toggleFullscreen).toHaveBeenCalledOnce()
    expect(await screen.findByRole('button', { name: 'Exit fullscreen' })).toBeTruthy()

    api.toggleFullscreen.mockResolvedValueOnce(false)
    await user.click(screen.getByRole('button', { name: 'Exit fullscreen' }))
    expect(api.toggleFullscreen).toHaveBeenCalledTimes(2)
    expect(await screen.findByRole('button', { name: 'Enter fullscreen' })).toBeTruthy()
  })

  test('syncs the fullscreen control after a native window change', async () => {
    const api = fakeApi()
    api.fullscreenState
      .mockResolvedValueOnce(false)
      .mockResolvedValueOnce(true)
    render(<PreviewApp api={api} />)

    await screen.findByRole('heading', { name: 'Production', level: 1 })
    fireEvent.resize(window)

    expect(await screen.findByRole('button', { name: 'Exit fullscreen' })).toBeTruthy()
  })

  test('reloads the remote file in place', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<PreviewApp api={api} />)

    await screen.findByRole('heading', { name: 'Production', level: 1 })
    await user.click(screen.getByRole('button', { name: 'Reload preview' }))

    await waitFor(() => expect(api.previewFile).toHaveBeenCalledTimes(2))
  })

  test('unlocks an encrypted key inside the preview process', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    api.connect
      .mockResolvedValueOnce({ needsPassphrase: true })
      .mockResolvedValueOnce({ connected: true, homePath: '/home/deploy' })
    render(<PreviewApp api={api} />)

    expect(await screen.findByRole('heading', { name: 'Unlock Production' })).toBeTruthy()
    await user.type(screen.getByLabelText('Key passphrase'), 'secret')
    await user.click(screen.getByRole('button', { name: 'Connect and preview' }))

    await waitFor(() => expect(api.connect).toHaveBeenLastCalledWith('secret'))
    expect(await screen.findByRole('heading', { name: 'Production', level: 1 })).toBeTruthy()
  })
})
