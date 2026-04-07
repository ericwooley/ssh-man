import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import ActiveConnections from './ActiveConnections.svelte'

describe('ActiveConnections', () => {
  it('renders active tunnels and stop actions', async () => {
    const onSelect = vi.fn()
    const onStop = vi.fn()

    render(ActiveConnections, {
      props: {
        connections: [{
          configurationId: 'config-1',
          configurationLabel: 'SOCKS',
          serverName: 'Primary',
          status: 'connected',
          statusDetail: 'Listening on localhost:1080',
        }],
        onSelect,
        onStop,
      },
    })

    expect(screen.getByText('Listening on localhost:1080')).toBeTruthy()

    await fireEvent.click(screen.getByRole('button', { name: 'Show SOCKS' }))
    await fireEvent.click(screen.getByRole('button', { name: 'More actions for SOCKS' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Disconnect SOCKS' }))

    expect(onSelect).toHaveBeenCalledWith('config-1')
    expect(onStop).toHaveBeenCalledWith('config-1')
  })
})
