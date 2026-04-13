import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import ConfigEditor from './ConfigEditor.svelte'

describe('ConfigEditor', () => {
  it('shows field validation state for a local forward', () => {
    render(ConfigEditor, {
      props: {
        server: { id: 'server-1', name: 'Primary' },
        value: {
          serverId: 'server-1',
          label: '',
          connectionType: 'local_forward',
          localPort: '',
          remoteHost: '',
          remotePort: '',
          notes: '',
          autoReconnectEnabled: true,
        },
        errors: {
          label: 'A label is required.',
          localPort: 'Local port is required.',
          remoteHost: 'Remote host is required.',
          remotePort: 'Remote port is required.',
        },
      },
    })

    expect(screen.getByText('A label is required.')).toBeTruthy()
    expect(screen.getByLabelText('Label').getAttribute('aria-invalid')).toBe('true')
    expect(screen.getByLabelText('Local port').getAttribute('aria-invalid')).toBe('true')
  })

  it('submits the current form value', async () => {
    const onSubmit = vi.fn()

    render(ConfigEditor, {
      props: {
        server: { id: 'server-1', name: 'Primary' },
        value: {
          serverId: 'server-1',
          label: 'Docs tunnel',
          connectionType: 'socks_proxy',
          socksPort: '1080',
          notes: '',
          autoReconnectEnabled: true,
        },
        onSubmit,
      },
    })

    await fireEvent.click(screen.getByRole('button', { name: 'Save tunnel' }))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit.mock.calls[0][0].label).toBe('Docs tunnel')
  })

  it('supports auto SOCKS port mode', async () => {
    const onSubmit = vi.fn()

    render(ConfigEditor, {
      props: {
        server: { id: 'server-1', name: 'Primary' },
        value: {
          serverId: 'server-1',
          label: 'Auto SOCKS',
          connectionType: 'socks_proxy',
          socksPort: 'auto',
          notes: '',
          autoReconnectEnabled: true,
        },
        onSubmit,
      },
    })

    expect(screen.getByLabelText('SOCKS port mode').value).toBe('auto')
    expect(screen.queryByLabelText('SOCKS port')).toBeNull()

    await fireEvent.click(screen.getByRole('button', { name: 'Save tunnel' }))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit.mock.calls[0][0].socksPort).toBe('auto')
  })

  it('uses numeric inputs for port fields', async () => {
    render(ConfigEditor, {
      props: {
        server: { id: 'server-1', name: 'Primary' },
        value: {
          serverId: 'server-1',
          label: 'Docs tunnel',
          connectionType: 'local_forward',
          localPort: '',
          remoteHost: '127.0.0.1',
          remotePort: '',
          notes: '',
          autoReconnectEnabled: true,
        },
      },
    })

    expect(screen.getByLabelText('Local port').getAttribute('type')).toBe('number')
    expect(screen.getByLabelText('Local port').getAttribute('min')).toBe('1')
    expect(screen.getByLabelText('Remote port').getAttribute('type')).toBe('number')

    await fireEvent.change(screen.getByLabelText('Type'), { target: { value: 'socks_proxy' } })
    await fireEvent.change(screen.getByLabelText('SOCKS port mode'), { target: { value: 'manual' } })

    expect(screen.getByLabelText('SOCKS port').getAttribute('type')).toBe('number')
    expect(screen.getByLabelText('SOCKS port').getAttribute('max')).toBe('65535')
  })
})
