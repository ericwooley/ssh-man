import { afterEach, describe, expect, test, vi } from 'vitest'
import {
  copy,
  createFolder,
  deleteItems,
  focusPreview,
  move,
  openPreview,
  previewWindowState,
  rename,
  subscribePreviewWindowState,
} from './explorerApi'

afterEach(() => {
  delete window.go
  delete window.runtime
})

describe('preview window lifecycle', () => {
  test('opens and queries previews through the launcher binding', async () => {
    const Open = vi.fn(async (remotePath) => ({ remotePath, open: true }))
    const Focus = vi.fn(async (remotePath) => ({ remotePath, open: true }))
    const State = vi.fn(async (remotePath) => ({ remotePath, open: true }))
    window.go = { bindings: { PreviewLauncherBindings: { Focus, Open, State } } }

    await expect(openPreview('/tmp/report.pdf')).resolves.toEqual({
      remotePath: '/tmp/report.pdf',
      open: true,
    })
    await expect(previewWindowState('/tmp/report.pdf')).resolves.toEqual({
      remotePath: '/tmp/report.pdf',
      open: true,
    })
    await expect(focusPreview('/tmp/report.pdf')).resolves.toEqual({
      remotePath: '/tmp/report.pdf',
      open: true,
    })
    expect(Focus).toHaveBeenCalledWith('/tmp/report.pdf')
  })

  test('forwards preview process state events and returns the runtime cleanup', () => {
    const cleanup = vi.fn()
    let eventListener
    window.runtime = {
      EventsOn: vi.fn((name, listener) => {
        expect(name).toBe('preview-window:state')
        eventListener = listener
        return cleanup
      }),
    }
    const listener = vi.fn()

    const unsubscribe = subscribePreviewWindowState(listener)
    eventListener({ remotePath: '/tmp/report.pdf', open: false })
    unsubscribe()

    expect(listener).toHaveBeenCalledWith({ remotePath: '/tmp/report.pdf', open: false })
    expect(cleanup).toHaveBeenCalledTimes(1)
  })
})

describe('remote file operations', () => {
  test('forwards Finder-style file mutations to the explorer binding', async () => {
    const ExplorerBindings = {
      CreateFolder: vi.fn(async () => '/tmp/releases'),
      Rename: vi.fn(async () => '/tmp/notes.md'),
      Copy: vi.fn(async () => ['/tmp/archive/report.md']),
      Move: vi.fn(async () => ['/tmp/archive/report.md']),
      Delete: vi.fn(async () => undefined),
    }
    window.go = { bindings: { ExplorerBindings } }

    await expect(createFolder('/tmp', 'releases')).resolves.toBe('/tmp/releases')
    await expect(rename('/tmp/report.md', 'notes.md')).resolves.toBe('/tmp/notes.md')
    await expect(copy(['/tmp/report.md'], '/tmp/archive')).resolves.toEqual(['/tmp/archive/report.md'])
    await expect(move(['/tmp/report.md'], '/tmp/archive')).resolves.toEqual(['/tmp/archive/report.md'])
    await expect(deleteItems(['/tmp/report.md'])).resolves.toBeUndefined()

    expect(ExplorerBindings.CreateFolder).toHaveBeenCalledWith('/tmp', 'releases')
    expect(ExplorerBindings.Rename).toHaveBeenCalledWith('/tmp/report.md', 'notes.md')
    expect(ExplorerBindings.Copy).toHaveBeenCalledWith(['/tmp/report.md'], '/tmp/archive')
    expect(ExplorerBindings.Move).toHaveBeenCalledWith(['/tmp/report.md'], '/tmp/archive')
    expect(ExplorerBindings.Delete).toHaveBeenCalledWith(['/tmp/report.md'])
  })
})
