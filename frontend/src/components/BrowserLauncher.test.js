import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import BrowserLauncher from './BrowserLauncher.svelte'

describe('BrowserLauncher', () => {
  it('shows guidance when a SOCKS tunnel is selected but not connected', () => {
    render(BrowserLauncher, {
      props: {
        configuration: { id: 'config-1', connectionType: 'socks_proxy' },
        session: { status: 'stopped' },
      },
    })

    expect(screen.getByText('Start a SOCKS tunnel to enable browser launch.')).toBeTruthy()
  })

  it('launches Firefox when it is supported', async () => {
    const onLaunch = vi.fn()
    const onSelect = vi.fn()

    render(BrowserLauncher, {
      props: {
        configuration: { id: 'config-1', connectionType: 'socks_proxy' },
        session: { status: 'connected' },
        browsers: [
          { id: 'firefox', displayName: 'Firefox', supportsProxyLaunch: true },
          { id: 'chromium', displayName: 'Chromium', supportsProxyLaunch: true },
        ],
        selectedBrowserId: 'firefox',
        launchPreview: 'firefox -new-instance -profile /tmp/ssh-man-browser-profiles/firefox-43123',
        onLaunch,
        onSelect,
      },
    })

    expect(screen.getByText('firefox -new-instance -profile /tmp/ssh-man-browser-profiles/firefox-43123')).toBeTruthy()

    await fireEvent.click(screen.getByRole('button', { name: 'Launch through SOCKS' }))

    expect(onLaunch).toHaveBeenCalledWith('config-1', 'firefox')
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('shows unsupported browser messaging and launches supported selections', async () => {
    const onLaunch = vi.fn()
    const onSelect = vi.fn()

    const { rerender } = render(BrowserLauncher, {
      props: {
        configuration: { id: 'config-1', connectionType: 'socks_proxy' },
        session: { status: 'connected' },
        browsers: [
          { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
          { id: 'chromium', displayName: 'Chromium', supportsProxyLaunch: true },
        ],
        selectedBrowserId: 'safari',
        onLaunch,
        onSelect,
      },
    })

    expect(screen.getByRole('option', { name: 'Safari (unsupported)' })).toBeTruthy()
    expect(screen.getByRole('alert').textContent).toContain('cannot launch it through a SOCKS proxy')
    expect(screen.getByRole('button', { name: 'Launch through SOCKS' }).hasAttribute('disabled')).toBe(true)

    await rerender({
      configuration: { id: 'config-1', connectionType: 'socks_proxy' },
      session: { status: 'connected' },
      browsers: [
        { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
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
