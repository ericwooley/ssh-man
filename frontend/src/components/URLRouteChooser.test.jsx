import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { URLRouteChooser } from './URLRouteChooser'

const request = {
  id: 'route-1',
  url: 'http://localhost:3000/dashboard',
  defaultChoiceId: 'browser:safari',
  timeoutMilliseconds: 5000,
  choices: [
    {
      id: 'browser:safari',
      kind: 'browser',
      label: 'Safari',
      detail: 'Regular browser',
      browserId: 'safari',
    },
    {
      id: 'proxy:server-socks:staging:firefox',
      kind: 'proxy',
      label: 'Firefox through Staging',
      detail: 'SOCKS5 proxy',
      serverId: 'staging',
      serverName: 'Staging',
      configurationId: 'server-socks:staging',
      browserId: 'firefox',
      browserName: 'Firefox',
    },
  ],
}

describe('URLRouteChooser', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  test('opens the preselected default when its timeout finishes', async () => {
    vi.useFakeTimers()
    const onChoose = vi.fn(async () => {})
    render(<URLRouteChooser request={request} onChoose={onChoose} onDismiss={() => {}} />)

    expect(screen.getByText('Opening Safari in 5s')).toBeTruthy()
    await act(async () => {
      vi.advanceTimersByTime(5000)
      await Promise.resolve()
    })

    expect(onChoose).toHaveBeenCalledWith('browser:safari')
  })

  test('pauses the timer on interaction and lets the keyboard select another destination', async () => {
    vi.useFakeTimers()
    const onChoose = vi.fn(async () => {})
    render(<URLRouteChooser request={request} onChoose={onChoose} onDismiss={() => {}} />)
    const dialog = screen.getByRole('dialog', { name: 'Choose where to open this link' })

    fireEvent.keyDown(dialog, { key: 'ArrowDown' })
    expect(screen.getByText('Timer paused')).toBeTruthy()
    act(() => {
      vi.advanceTimersByTime(10000)
    })
    expect(onChoose).not.toHaveBeenCalled()

    await act(async () => {
      fireEvent.keyDown(dialog, { key: 'Enter' })
      await Promise.resolve()
    })
    expect(onChoose).toHaveBeenCalledWith('proxy:server-socks:staging:firefox')
  })
})
