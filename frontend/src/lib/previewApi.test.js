import { afterEach, describe, expect, test, vi } from 'vitest'
import { fullscreenState, toggleFullscreen } from './previewApi'

afterEach(() => {
  delete window.runtime
})

describe('preview window runtime', () => {
  test('enters and exits native fullscreen', async () => {
    const WindowFullscreen = vi.fn()
    const WindowUnfullscreen = vi.fn()
    const WindowIsFullscreen = vi.fn()
      .mockResolvedValueOnce(false)
      .mockResolvedValue(true)
    window.runtime = {
      WindowFullscreen,
      WindowUnfullscreen,
      WindowIsFullscreen,
    }

    await expect(toggleFullscreen()).resolves.toBe(true)
    expect(WindowFullscreen).toHaveBeenCalledOnce()

    await expect(toggleFullscreen()).resolves.toBe(false)
    expect(WindowUnfullscreen).toHaveBeenCalledOnce()
    await expect(fullscreenState()).resolves.toBe(true)
    expect(WindowIsFullscreen).toHaveBeenCalledTimes(3)
  })
})
