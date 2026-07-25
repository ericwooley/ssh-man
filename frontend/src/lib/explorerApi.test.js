import { afterEach, describe, expect, test, vi } from 'vitest'
import {
  focusPreview,
  openPreview,
  previewWindowState,
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
