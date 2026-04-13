import { render, screen } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'

import SessionStatus from '../SessionStatus.svelte'

describe('session status visual contract', () => {
  it('applies the running status class and visible label', () => {
    const { container } = render(SessionStatus, {
      props: {
        configuration: { id: 'config-1', label: 'SOCKS', connectionType: 'socks_proxy', socksPort: 1080 },
        session: { status: 'connected', statusDetail: 'Listening on localhost:1080' },
      },
    })

    expect(screen.getByLabelText('Session status connected').textContent).toContain('Connected')
    expect(container.querySelector('.status-pill.status-running')).toBeTruthy()
    expect(container.querySelector('.status-callout.status-running')).toBeTruthy()
  })

  it('applies the attention class and explanatory detail', () => {
    const { container } = render(SessionStatus, {
      props: {
        configuration: { id: 'config-1', label: 'SOCKS' },
        session: { status: 'needs_attention', statusDetail: 'Unlock the SSH key to continue' },
      },
    })

    expect(screen.getByText('Unlock the SSH key to continue')).toBeTruthy()
    expect(container.querySelector('.status-pill.status-attention')).toBeTruthy()
  })
})
