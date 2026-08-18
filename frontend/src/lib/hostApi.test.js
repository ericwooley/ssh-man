import { afterEach, describe, expect, test, vi } from 'vitest'
import {
  deleteConnectionConfiguration,
  deleteServer,
  discoverBrowsers,
  launchBrowserThroughSocks,
  listRuntimeSessions,
  listSessionHistory,
  loadInitialState,
  previewBrowserLaunchThroughSocks,
  retryConfiguration,
  saveConnectionConfiguration,
  savePreferences,
  saveServer,
  startConfiguration,
  startServerConfigurations,
  stopConfiguration,
  submitKeyUnlock,
} from './hostApi'

afterEach(() => {
  delete window.go
})

describe('host tunnel bindings', () => {
  test('routes tunnel management through the host window bindings', async () => {
    const result = { ok: true }
    const HostBindings = {
      LoadAppState: vi.fn(async () => result),
      ListRuntimeSessions: vi.fn(async () => result),
      ListSessionHistory: vi.fn(async () => result),
      SaveServer: vi.fn(async () => result),
      DeleteServer: vi.fn(async () => result),
      SaveConnectionConfiguration: vi.fn(async () => result),
      DeleteConnectionConfiguration: vi.fn(async () => result),
      StartConfiguration: vi.fn(async () => result),
      StartServerConfigurations: vi.fn(async () => result),
      StopConfiguration: vi.fn(async () => result),
      RetryConfiguration: vi.fn(async () => result),
      SubmitKeyUnlock: vi.fn(async () => result),
      DiscoverBrowsers: vi.fn(async () => result),
      PreviewBrowserLaunchThroughSocks: vi.fn(async () => result),
      LaunchBrowserThroughSocks: vi.fn(async () => result),
    }
    window.go = { bindings: { HostBindings } }

    await expect(loadInitialState()).resolves.toBe(result)
    await expect(listRuntimeSessions()).resolves.toBe(result)
    await expect(listSessionHistory('tunnel-1')).resolves.toBe(result)
    await expect(saveServer({ id: 'server-1' })).resolves.toBe(result)
    await expect(deleteServer('server-1')).resolves.toBe(result)
    await expect(saveConnectionConfiguration({ id: 'tunnel-1' })).resolves.toBe(result)
    await expect(deleteConnectionConfiguration('tunnel-1')).resolves.toBe(result)
    await expect(startConfiguration('tunnel-1')).resolves.toBe(result)
    await expect(startServerConfigurations('server-1')).resolves.toBe(result)
    await expect(stopConfiguration('tunnel-1')).resolves.toBe(result)
    await expect(retryConfiguration('tunnel-1')).resolves.toBe(result)
    await expect(submitKeyUnlock('tunnel-1', 'secret')).resolves.toBe(result)
    await expect(discoverBrowsers()).resolves.toBe(result)
    await expect(previewBrowserLaunchThroughSocks('tunnel-1', 'browser-1')).resolves.toBe(result)
    await expect(launchBrowserThroughSocks('tunnel-1', 'browser-1')).resolves.toBe(result)

    expect(HostBindings.ListSessionHistory).toHaveBeenCalledWith('tunnel-1')
    expect(HostBindings.SaveServer).toHaveBeenCalledWith({ id: 'server-1' })
    expect(HostBindings.SubmitKeyUnlock).toHaveBeenCalledWith('tunnel-1', 'secret')
    expect(HostBindings.PreviewBrowserLaunchThroughSocks).toHaveBeenCalledWith('tunnel-1', 'browser-1')
    expect(HostBindings.LaunchBrowserThroughSocks).toHaveBeenCalledWith('tunnel-1', 'browser-1')
  })

  test('keeps preferences in memory because the main process owns them', async () => {
    const preferences = { theme: 'light' }

    await expect(savePreferences(preferences)).resolves.toBe(preferences)
  })
})
