import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import BrowserLauncher from './BrowserLauncher.svelte'

describe('BrowserLauncher', () => {
  it('keeps launch disabled until a running socks session is available', () => {
    render(BrowserLauncher, {
      props: {
        configuration: { id: 'config-1', connectionType: 'local_forward' },
        session: { status: 'stopped' },
      },
    })

    expect(screen.getByText('Start a SOCKS tunnel to enable browser launch.')).toBeTruthy()
  })

  it('shows unsupported browser messaging and launches supported selections', async () => {
    const onLaunch = vi.fn()
    const onSelect = vi.fn()

    const { rerender } = render(BrowserLauncher, {
      props: {
        configuration: { id: 'config-1', connectionType: 'socks_proxy' },
        session: { status: 'connected' },
        browsers: [
          { id: 'firefox', displayName: 'Firefox', supportsProxyLaunch: false },
          { id: 'chromium', displayName: 'Chromium', supportsProxyLaunch: true },
        ],
        selectedBrowserId: 'firefox',
        onLaunch,
        onSelect,
      },
    })

    expect(screen.getByRole('alert').textContent).toContain('cannot launch it through a SOCKS proxy')
    expect(screen.getByRole('button', { name: 'Launch through SOCKS' }).hasAttribute('disabled')).toBe(true)

    await rerender({
      configuration: { id: 'config-1', connectionType: 'socks_proxy' },
      session: { status: 'connected' },
      browsers: [
        { id: 'firefox', displayName: 'Firefox', supportsProxyLaunch: false },
        { id: 'chromium', displayName: 'Chromium', supportsProxyLaunch: true },
      ],
      selectedBrowserId: 'chromium',
      onLaunch,
      onSelect,
    })
    await fireEvent.click(screen.getByRole('button', { name: 'Launch through SOCKS' }))

    expect(onLaunch).toHaveBeenCalledWith('config-1', 'chromium')
  })
})
