import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import SessionStatus from './SessionStatus.svelte'

describe('SessionStatus', () => {
  it('renders runtime messaging and session actions', async () => {
    const onStart = vi.fn()
    const onStop = vi.fn()
    const onRetry = vi.fn()

    render(SessionStatus, {
      props: {
        configuration: { id: 'config-1', label: 'SOCKS' },
        session: { status: 'connected', statusDetail: 'Listening on localhost:1080' },
        onStart,
        onStop,
        onRetry,
      },
    })

    expect(screen.getByLabelText('Session status connected')).toBeTruthy()
    expect(screen.getByRole('status').textContent).toContain('Listening on localhost:1080')

    await fireEvent.click(screen.getByRole('button', { name: 'Start' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Stop' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Retry' }))

    expect(onStart).toHaveBeenCalledWith('config-1')
    expect(onStop).toHaveBeenCalledWith('config-1')
    expect(onRetry).toHaveBeenCalledWith('config-1')
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
})
