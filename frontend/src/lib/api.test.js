import { afterEach, describe, expect, test, vi } from 'vitest'
import {
  onBrowserSwitcherCancelRequested,
  onBrowserSwitcherCommitRequested,
  onBrowserSwitcherRequested,
  saveBrowserAppearance,
} from './api'

function installRuntime() {
  const listeners = new Map()
  window.runtime = {
    EventsOn: vi.fn((name, callback) => {
      listeners.set(name, callback)
      return () => listeners.delete(name)
    }),
  }
  return listeners
}

afterEach(() => {
  delete window.runtime
  delete window.go
})

describe('browser appearance persistence', () => {
  test('uses the granular Wails binding for live appearance changes', async () => {
    const saved = {
      theme: 'dark',
      browserAppearances: {
        'proxy:server-1:google-chrome': { icon: 'icon:x', primaryColor: '#22C55E' },
      },
    }
    const SaveBrowserAppearance = vi.fn(async () => saved)
    window.go = { bindings: { AppBindings: { SaveBrowserAppearance } } }

    await expect(saveBrowserAppearance(
      'proxy:server-1:google-chrome',
      { icon: 'icon:x', primaryColor: '#22C55E' },
    )).resolves.toEqual(saved)
    expect(SaveBrowserAppearance).toHaveBeenCalledWith(
      'proxy:server-1:google-chrome',
      { icon: 'icon:x', primaryColor: '#22C55E' },
    )
  })
})

describe('browser switcher runtime events', () => {
  test('preserves production direction and session payloads', () => {
    const listeners = installRuntime()
    const onOpen = vi.fn()
    const onCommit = vi.fn()
    const onCancel = vi.fn()
    onBrowserSwitcherRequested(onOpen)
    onBrowserSwitcherCommitRequested(onCommit)
    onBrowserSwitcherCancelRequested(onCancel)

    listeners.get('browser-switcher:open')({ direction: 'backward', sessionId: 'session-1' })
    listeners.get('browser-switcher:commit')({ sessionId: 'session-1' })
    listeners.get('browser-switcher:cancel')({ sessionId: 'session-1' })

    expect(onOpen).toHaveBeenCalledWith({ direction: 'backward', sessionId: 'session-1' })
    expect(onCommit).toHaveBeenCalledWith({ sessionId: 'session-1' })
    expect(onCancel).toHaveBeenCalledWith({ sessionId: 'session-1' })
  })

  test('keeps legacy direction and sessionless events usable', () => {
    const listeners = installRuntime()
    const onOpen = vi.fn()
    const onCommit = vi.fn()
    onBrowserSwitcherRequested(onOpen)
    onBrowserSwitcherCommitRequested(onCommit)

    listeners.get('browser-switcher:open')('backward')
    listeners.get('browser-switcher:commit')()

    expect(onOpen).toHaveBeenCalledWith({ direction: 'backward', sessionId: '' })
    expect(onCommit).toHaveBeenCalledWith({ sessionId: '' })
  })
})
