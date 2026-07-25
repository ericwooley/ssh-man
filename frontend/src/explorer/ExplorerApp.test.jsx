import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'
import ExplorerApp from './ExplorerApp'

vi.mock('./MonacoPreview', () => ({
  default: ({ content, label, onChange, onSave, vimMode }) => (
    <div data-testid="monaco-editor" data-vim={vimMode ? 'enabled' : 'disabled'}>
      <textarea aria-label={label} value={content} onChange={(event) => onChange?.(event.target.value)} />
      <button type="button" onClick={() => onSave?.(content)}>Editor save</button>
    </div>
  ),
}))

function fakeApi() {
  const previewWindowListeners = new Set()
  const directories = {
    '/home/deploy': {
      path: '/home/deploy',
      entries: [
        { name: 'site', path: '/home/deploy/site', kind: 'directory', size: 0, modifiedAt: '2026-07-22T12:00:00Z' },
        { name: 'README.md', path: '/home/deploy/README.md', kind: 'file', size: 17, modifiedAt: '2026-07-22T12:00:00Z' },
      ],
    },
    '/home/deploy/site': { path: '/home/deploy/site', entries: [] },
  }
  const api = {
    initialState: vi.fn(async () => ({ server: { id: 'server-1', name: 'Production', username: 'deploy', host: 'prod.example.com' } })),
    connect: vi.fn(async () => ({ connected: true, homePath: '/home/deploy' })),
    listDirectory: vi.fn(async (path) => directories[path]),
    previewFile: vi.fn(async () => ({
      path: '/home/deploy/README.md',
      name: 'README.md',
      kind: 'markdown',
      mimeType: 'text/markdown',
      content: '# Production\n\nRemote notes.',
      size: 17,
      revision: 'revision-1',
    })),
    saveFile: vi.fn(async (path, content) => ({
      path,
      name: 'README.md',
      kind: 'markdown',
      mimeType: 'text/markdown',
      content,
      size: content.length,
      revision: 'revision-2',
    })),
    download: vi.fn(async () => ['/Users/eric/Downloads/README.md']),
    openPreview: vi.fn(async (remotePath) => ({ remotePath, open: true })),
    focusPreview: vi.fn(async (remotePath) => ({ remotePath, open: true })),
    previewWindowState: vi.fn(async (remotePath) => ({ remotePath, open: false })),
    subscribePreviewWindowState: vi.fn((listener) => {
      previewWindowListeners.add(listener)
      return () => previewWindowListeners.delete(listener)
    }),
    openExternalURL: vi.fn(async () => undefined),
    emitPreviewWindowState(state) {
      previewWindowListeners.forEach((listener) => listener(state))
    },
  }
  return api
}

describe('server explorer window', () => {
  beforeEach(() => window.localStorage.clear())

  test('uses browser-compatible local storage', () => {
    expect(typeof window.localStorage.clear).toBe('function')
    expect(typeof window.localStorage.getItem).toBe('function')
  })

  test('keeps the MoonPixels link visible in the explorer sidebar', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('button', { name: 'Visit MoonPixels' }))

    expect(api.openExternalURL).toHaveBeenCalledWith('https://moonpixels.tech')
  })

  test('browses folders and renders remote markdown', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    expect(await screen.findByDisplayValue('/home/deploy')).toBeTruthy()
    await user.click(screen.getByRole('option', { name: /README\.md/ }))

    expect(await screen.findByRole('heading', { name: 'Production', level: 1 })).toBeTruthy()
    expect(screen.getByText('Remote notes.')).toBeTruthy()

    await user.dblClick(screen.getByRole('option', { name: /site/ }))
    await waitFor(() => expect(api.listDirectory).toHaveBeenLastCalledWith('/home/deploy/site'))
    expect(await screen.findByText('This folder is empty')).toBeTruthy()
  })

  test('renders video previews with a full-size native video player', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    api.listDirectory.mockImplementation(async () => ({
      path: '/home/deploy',
      entries: [
        { name: 'capture.mp4', path: '/home/deploy/capture.mp4', kind: 'file', size: 1400000, modifiedAt: '2026-07-22T12:00:00Z' },
      ],
    }))
    api.previewFile.mockResolvedValue({
      path: '/home/deploy/capture.mp4',
      name: 'capture.mp4',
      kind: 'video',
      mimeType: 'video/mp4',
      size: 1400000,
    })
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /capture\.mp4/ }))

    const player = await screen.findByLabelText('capture.mp4 video preview')
    expect(player.tagName).toBe('VIDEO')
    expect(player.hasAttribute('controls')).toBe(true)
    expect(player.getAttribute('src')).toBe('/__ssh_man_remote__/raw/home/deploy/capture.mp4')
    expect(screen.getByText('Video preview')).toBeTruthy()
    expect(screen.queryByTitle('capture.mp4 browser preview')).toBeNull()
  })

  test('focuses a detached preview and restores it after the window closes', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    expect(await screen.findByRole('heading', { name: 'Production', level: 1 })).toBeTruthy()
    await user.click(await screen.findByRole('button', { name: 'Open README.md preview in new window' }))

    await waitFor(() => expect(api.openPreview).toHaveBeenCalledWith('/home/deploy/README.md'))
    expect(await screen.findByText('Preview open in window')).toBeTruthy()
    expect(screen.queryByRole('heading', { name: 'Production', level: 1 })).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Focus preview window for README.md' }))
    expect(api.openPreview).toHaveBeenCalledTimes(1)
    expect(api.focusPreview).toHaveBeenCalledWith('/home/deploy/README.md')

    act(() => {
      api.emitPreviewWindowState({ remotePath: '/home/deploy/README.md', open: false })
    })

    expect(await screen.findByRole('heading', { name: 'Production', level: 1 })).toBeTruthy()
  })

  test('downloads the selected remote file through the native destination flow', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    await user.click(screen.getByRole('button', { name: 'Download' }))

    await waitFor(() => expect(api.download).toHaveBeenCalledWith(['/home/deploy/README.md']))
    expect(await screen.findByText(/Downloaded 1 item/)).toBeTruthy()
  })

  test('edits and saves a remote file from Monaco', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    await user.click(await screen.findByRole('button', { name: 'Source' }))
    const editor = await screen.findByRole('textbox', { name: 'README.md source' })
    await user.clear(editor)
    await user.type(editor, '# Updated remotely')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(api.saveFile).toHaveBeenCalledWith('/home/deploy/README.md', '# Updated remotely', 'revision-1'))
    expect(await screen.findByText('Saved README.md.')).toBeTruthy()
  })

  test('keeps an unsaved editor inline instead of opening a detached preview', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    await user.click(await screen.findByRole('button', { name: 'Source' }))
    const editor = await screen.findByRole('textbox', { name: 'README.md source' })
    await user.type(editor, ' unsaved')

    const popout = screen.getByRole('button', { name: 'Open README.md preview in new window' })
    expect(popout.disabled).toBe(true)
    await user.click(popout)
    expect(api.openPreview).not.toHaveBeenCalled()
  })

  test('persists Vim controls from the editor checkbox', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.click(await screen.findByRole('option', { name: /README\.md/ }))
    await user.click(await screen.findByRole('button', { name: 'Source' }))
    await user.click(screen.getByRole('checkbox', { name: 'Vim controls' }))

    expect(window.localStorage.getItem('ssh-man:explorer:vim-enabled')).toBe('true')
    expect(screen.getByTestId('monaco-editor').dataset.vim).toBe('enabled')
  })

  test('favorites the current folder per server and reopens it from the sidebar', async () => {
    const user = userEvent.setup()
    const api = fakeApi()
    render(<ExplorerApp api={api} />)

    await user.dblClick(await screen.findByRole('option', { name: /site/ }))
    await screen.findByText('This folder is empty')
    await user.click(screen.getByRole('button', { name: 'Favorite current folder' }))
    await user.click(screen.getByRole('button', { name: 'Open favorite site' }))

    await waitFor(() => expect(api.listDirectory).toHaveBeenLastCalledWith('/home/deploy/site'))
    expect(JSON.parse(window.localStorage.getItem('ssh-man:explorer:favorites:server-1'))).toEqual(['/home/deploy/site'])
  })
})
