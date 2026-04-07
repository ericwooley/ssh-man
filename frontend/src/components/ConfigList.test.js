import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import ConfigList from './ConfigList.svelte'

describe('ConfigList', () => {
  it('renders tunnel runtime status and actions', async () => {
    const onCreate = vi.fn()
    const onStartAll = vi.fn()
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
        onStartAll,
        onSelect,
        onEdit,
        onDelete,
      },
    })

    expect(screen.getByLabelText('Tunnel status connected')).toBeTruthy()

    await fireEvent.click(screen.getByRole('button', { name: 'Start all' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Add tunnel' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Select tunnel SOCKS' }))
    await fireEvent.click(screen.getByRole('button', { name: 'More actions for SOCKS' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Edit SOCKS' }))
    await fireEvent.click(screen.getByRole('button', { name: 'More actions for SOCKS' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Delete SOCKS' }))

    expect(onStartAll).toHaveBeenCalledTimes(1)
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
    expect(screen.getByRole('button', { name: 'Start all' }).hasAttribute('disabled')).toBe(true)
  })

  it('prefers the latest session when stale entries exist', () => {
    render(ConfigList, {
      props: {
        selectedConfigurationId: 'config-1',
        configurations: [{
          id: 'config-1',
          label: 'SOCKS',
          connectionType: 'socks_proxy',
          socksPort: 1080,
        }],
        sessions: [
          { configurationId: 'config-1', status: 'stopped' },
          { configurationId: 'config-1', status: 'connected' },
        ],
      },
    })

    expect(screen.getByLabelText('Tunnel status connected')).toBeTruthy()
  })

  it('closes the action menu on outside click and escape', async () => {
    render(ConfigList, {
      props: {
        configurations: [{
          id: 'config-1',
          label: 'SOCKS',
          connectionType: 'socks_proxy',
          socksPort: 1080,
        }],
        sessions: [],
      },
    })

    await fireEvent.click(screen.getByRole('button', { name: 'More actions for SOCKS' }))
    expect(screen.getByRole('menu', { name: 'Actions for SOCKS' })).toBeTruthy()

    await fireEvent.click(window)
    expect(screen.queryByRole('menu', { name: 'Actions for SOCKS' })).toBeNull()

    await fireEvent.click(screen.getByRole('button', { name: 'More actions for SOCKS' }))
    await fireEvent.keyDown(window, { key: 'Escape' })
    expect(screen.queryByRole('menu', { name: 'Actions for SOCKS' })).toBeNull()
  })
})
