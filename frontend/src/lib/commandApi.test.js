import { afterEach, describe, expect, test, vi } from 'vitest'
import {
  cancelCommand,
  completePath,
  connect,
  deleteHistory,
  hasCommandRuntime,
  initialState,
  runCommand,
} from './commandApi'

afterEach(() => {
  delete window.go
})

describe('command window bindings', () => {
  test('routes command operations through the companion runtime', async () => {
    const CommandBindings = {
      InitialState: vi.fn(async () => ({ server: { id: 'server-1' }, history: [] })),
      Connect: vi.fn(async () => ({ connected: true })),
      CompletePath: vi.fn(async () => ({ items: [] })),
      RunCommand: vi.fn(async () => ({ id: 'history-1' })),
      CancelCommand: vi.fn(async () => undefined),
      DeleteHistory: vi.fn(async () => undefined),
    }
    window.go = { bindings: { CommandBindings } }

    expect(hasCommandRuntime()).toBe(true)
    await initialState()
    await connect('secret')
    await completePath('/var/lo')
    await runCommand('pwd')
    await cancelCommand()
    await deleteHistory('history-1')

    expect(CommandBindings.Connect).toHaveBeenCalledWith('secret')
    expect(CommandBindings.CompletePath).toHaveBeenCalledWith('/var/lo')
    expect(CommandBindings.RunCommand).toHaveBeenCalledWith('pwd')
    expect(CommandBindings.DeleteHistory).toHaveBeenCalledWith('history-1')
  })
})
