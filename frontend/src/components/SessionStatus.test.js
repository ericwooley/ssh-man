import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import SessionStatus from './SessionStatus.svelte'

describe('SessionStatus', () => {
  it('renders runtime messaging and session actions', async () => {
    const onStart = vi.fn()
    const onStop = vi.fn()

    render(SessionStatus, {
      props: {
        configuration: { id: 'config-1', label: 'SOCKS', connectionType: 'socks_proxy', socksPort: 1080 },
        session: { status: 'connected', statusDetail: 'Listening on localhost:1080' },
        onStart,
        onStop,
      },
    })

    expect(screen.getByLabelText('Session status connected')).toBeTruthy()
    expect(screen.getByRole('status').textContent).toContain('Listening on localhost:1080')
    expect(screen.getByText('Selected tunnel')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Start tunnel' }).disabled).toBe(true)
    expect(screen.getByRole('button', { name: 'Stop tunnel' }).disabled).toBe(false)

    await fireEvent.click(screen.getByRole('button', { name: 'Stop tunnel' }))

    expect(onStart).not.toHaveBeenCalled()
    expect(onStop).toHaveBeenCalledWith('config-1')
  })

  it('enables start for stopped or failed sessions', async () => {
    const onStart = vi.fn()

    render(SessionStatus, {
      props: {
        configuration: { id: 'config-1', label: 'SOCKS', connectionType: 'socks_proxy', socksPort: 1080 },
        session: { status: 'failed', statusDetail: 'Tunnel start failed.' },
        onStart,
      },
    })

    expect(screen.getByRole('button', { name: 'Start tunnel' }).disabled).toBe(false)
    expect(screen.getByRole('button', { name: 'Stop tunnel' }).disabled).toBe(true)

    await fireEvent.click(screen.getByRole('button', { name: 'Start tunnel' }))

    expect(onStart).toHaveBeenCalledWith('config-1')
  })

  it('shows attention state without duplicating the unlock form', async () => {
    render(SessionStatus, {
      props: {
        configuration: { id: 'config-1', label: 'SOCKS' },
        session: { status: 'needs_attention', statusDetail: 'Unlock the SSH key to continue' },
      },
    })

    expect(screen.queryByLabelText('SSH key passphrase')).toBeNull()
    expect(screen.getByRole('status').textContent).toContain('Unlock the SSH key to continue')
  })

  it('shows the runtime assigned SOCKS port when auto mode is used', () => {
    render(SessionStatus, {
      props: {
        configuration: { id: 'config-1', label: 'SOCKS', connectionType: 'socks_proxy', socksPort: 0 },
        session: { status: 'connected', boundPort: 43123, statusDetail: 'Listening on localhost:43123' },
      },
    })

    expect(screen.getByText('SOCKS proxy on :43123')).toBeTruthy()
  })

  it('renders recent history and copies it on request', async () => {
    const onCopyHistory = vi.fn()

    render(SessionStatus, {
      props: {
        configuration: { id: 'config-1', label: 'SOCKS', connectionType: 'socks_proxy', socksPort: 1080 },
        session: { status: 'connected', statusDetail: 'Listening on localhost:1080' },
        history: [
          { id: 'entry-1', configurationId: 'config-1', outcome: 'connected', endedAt: '2026-04-09T12:00:00Z', message: 'Listening on localhost:1080' },
        ],
        onCopyHistory,
      },
    })

    expect(screen.getByText('Recent connection history')).toBeTruthy()
    expect(screen.getAllByText('Listening on localhost:1080').length).toBeGreaterThan(0)

    await fireEvent.click(screen.getByRole('button', { name: 'Copy history' }))

    expect(onCopyHistory).toHaveBeenCalledWith('config-1')
  })
})
