import { afterEach, describe, expect, test, vi } from 'vitest'
import {
  openSettingsWindow,
  onBrowserSwitcherCancelRequested,
  onBrowserSwitcherCommitRequested,
  onBrowserSwitcherRequested,
  openServerCommand,
  saveBrowserAppearance,
  setURLRouteWindowMode,
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

describe('URL route window sizing', () => {
  test('uses the expanded chooser size and restores the main window size', () => {
    window.runtime = {
      WindowSetSize: vi.fn(),
      WindowCenter: vi.fn(),
    }

    setURLRouteWindowMode(true)
    setURLRouteWindowMode(false)

    expect(window.runtime.WindowSetSize).toHaveBeenNthCalledWith(1, 520, 600)
    expect(window.runtime.WindowSetSize).toHaveBeenNthCalledWith(2, 420, 720)
    expect(window.runtime.WindowCenter).toHaveBeenCalledTimes(2)
  })
})

describe('companion window launchers', () => {
  test('uses the dedicated settings launcher binding', async () => {
    const Open = vi.fn(async () => undefined)
    window.go = { bindings: { SettingsLauncherBindings: { Open } } }

    await openSettingsWindow()

    expect(Open).toHaveBeenCalledTimes(1)
  })

  test('opens the quick command window through its dedicated binding', async () => {
    const Open = vi.fn(async () => undefined)
    window.go = { bindings: { CommandLauncherBindings: { Open } } }

    await openServerCommand('server-1')

    expect(Open).toHaveBeenCalledWith('server-1')
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
