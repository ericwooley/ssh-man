import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import ConfigList from './ConfigList.svelte'

describe('ConfigList', () => {
  it('renders tunnel runtime status and actions', async () => {
    const onCreate = vi.fn()
    const onSelect = vi.fn()
    const onEdit = vi.fn()
    const onDelete = vi.fn()

    render(ConfigList, {
      props: {
        selectedConfigurationId: 'config-1',
        configurations: [{
          id: 'config-1',
          label: 'SOCKS',
          connectionType: 'socks_proxy',
          socksPort: 1080,
        }],
        sessions: [{ configurationId: 'config-1', status: 'connected' }],
        onCreate,
        onSelect,
        onEdit,
        onDelete,
      },
    })

    expect(screen.getByLabelText('Tunnel status connected')).toBeTruthy()

    await fireEvent.click(screen.getByRole('button', { name: 'Add tunnel' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Select tunnel SOCKS' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Edit SOCKS' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Delete SOCKS' }))

    expect(onCreate).toHaveBeenCalledTimes(1)
    expect(onSelect).toHaveBeenCalledWith('config-1')
    expect(onEdit).toHaveBeenCalledTimes(1)
    expect(onDelete).toHaveBeenCalledWith('config-1')
  })

  it('disables tunnel management until a server is selected', () => {
    render(ConfigList, {
      props: {
        enabled: false,
        configurations: [],
        sessions: [],
      },
    })

    expect(screen.getByText('Select a target first')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Add tunnel' }).hasAttribute('disabled')).toBe(true)
  })
})
