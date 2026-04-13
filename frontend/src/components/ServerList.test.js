import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import ServerList from './ServerList.svelte'

describe('ServerList', () => {
  it('renders empty state copy', () => {
    render(ServerList, { props: { servers: [] } })

    expect(screen.getByText('No servers yet')).toBeTruthy()
    expect(screen.getByText('Create a server to start saving SSH tunnels.')).toBeTruthy()
  })

  it('selects and exposes labeled actions for saved servers', async () => {
    const onCreate = vi.fn()
    const onSelect = vi.fn()
    const onEdit = vi.fn()
    const onDelete = vi.fn()

    render(ServerList, {
      props: {
        selectedServerId: 'server-1',
        servers: [{
          server: { id: 'server-1', name: 'Primary', username: 'eric', host: 'example.com', port: 22 },
          configurations: [{ id: 'config-1' }],
        }],
        onCreate,
        onSelect,
        onEdit,
        onDelete,
      },
    })

    const primaryButton = screen.getByRole('button', { name: 'Select server Primary' })
    expect(primaryButton.getAttribute('aria-pressed')).toBe('true')

    await fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await fireEvent.click(primaryButton)
    await fireEvent.click(screen.getByRole('button', { name: 'Edit Primary' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Delete Primary' }))

    expect(onCreate).toHaveBeenCalledTimes(1)
    expect(onSelect).toHaveBeenCalledWith('server-1')
    expect(onEdit).toHaveBeenCalledTimes(1)
    expect(onDelete).toHaveBeenCalledWith('server-1')
  })

  it('renders inline edit and delete actions for each server', () => {
    render(ServerList, {
      props: {
        servers: [{
          server: { id: 'server-1', name: 'Primary', username: 'eric', host: 'example.com', port: 22 },
          configurations: [],
        }],
      },
    })

    expect(screen.getByRole('button', { name: 'Edit Primary' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Delete Primary' })).toBeTruthy()
  })
})
