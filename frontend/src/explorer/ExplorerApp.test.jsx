import { render, screen, waitFor } from '@testing-library/react'
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
  return {
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
  }
}

describe('server explorer window', () => {
  beforeEach(() => window.localStorage.clear())

  test('uses browser-compatible local storage', () => {
    expect(typeof window.localStorage.clear).toBe('function')
    expect(typeof window.localStorage.getItem).toBe('function')
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
