import { afterEach, describe, expect, test, vi } from 'vitest'
import {
  focusPreview,
  openPreview,
  previewWindowState,
  subscribeFileDrop,
  subscribePreviewWindowState,
  subscribeUploadProgress,
  uploadFiles,
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

  test('uploads local paths through the explorer binding', async () => {
    const Upload = vi.fn(async (_uploadID, remoteDirectory, localPaths) => (
      {
        uploaded: localPaths.map((localPath) => `${remoteDirectory}/${localPath.split('/').at(-1)}`),
        failures: [],
      }
    ))
    window.go = { bindings: { ExplorerBindings: { Upload } } }

    await expect(uploadFiles(7, '/srv/site', ['/Users/eric/report.txt']))
      .resolves.toEqual({ uploaded: ['/srv/site/report.txt'], failures: [] })
    expect(Upload).toHaveBeenCalledWith(7, '/srv/site', ['/Users/eric/report.txt'])
  })

  test('subscribes to native file drops on explorer drop targets', () => {
    const OnFileDrop = vi.fn()
    const OnFileDropOff = vi.fn()
    window.runtime = { OnFileDrop, OnFileDropOff }
    const listener = vi.fn()

    const unsubscribe = subscribeFileDrop(listener)
    expect(OnFileDrop).toHaveBeenCalledWith(listener, true)

    unsubscribe()
    expect(OnFileDropOff).toHaveBeenCalledTimes(1)
  })

  test('subscribes to upload progress events', () => {
    const callback = vi.fn()
    const unsubscribe = vi.fn()
    let listener
    window.runtime = {
      EventsOn: vi.fn((eventName, nextListener) => {
        listener = nextListener
        return unsubscribe
      }),
    }

    expect(subscribeUploadProgress(callback)).toBe(unsubscribe)
    expect(window.runtime.EventsOn).toHaveBeenCalledWith('explorer:upload-progress', expect.any(Function))

    listener({ fileIndex: 0, status: 'transferring', bytesTransferred: 12 })
    expect(callback).toHaveBeenCalledWith({ fileIndex: 0, status: 'transferring', bytesTransferred: 12 })
  })
})
