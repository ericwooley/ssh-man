import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import ServerEditorDialog from './ServerEditorDialog.svelte'

describe('ServerEditorDialog', () => {
  it('renders validation errors for required fields', () => {
    render(ServerEditorDialog, {
      props: {
        open: true,
        value: {
          id: '',
          name: '',
          host: '',
          port: '',
          username: '',
          authMode: 'private_key',
          keyReference: '',
        },
        errors: {
          name: 'A server name is required.',
          host: 'A server host is required.',
          username: 'An SSH username is required.',
          port: 'Port must be between 1 and 65535.',
          keyReference: 'A private key path is required.',
        },
      },
    })

    expect(screen.getByText('A server name is required.')).toBeTruthy()
    expect(screen.getByLabelText('Server name').getAttribute('aria-invalid')).toBe('true')
    expect(screen.getByLabelText('Server port').getAttribute('aria-invalid')).toBe('true')
  })

  it('submits the current server value', async () => {
    const onSubmit = vi.fn()

    render(ServerEditorDialog, {
      props: {
        open: true,
        value: {
          id: '',
          name: 'Primary',
          host: 'example.com',
          port: '22',
          username: 'eric',
          authMode: 'private_key',
          keyReference: '~/.ssh/id_ed25519',
        },
        onSubmit,
      },
    })

    await fireEvent.click(screen.getByRole('button', { name: 'Save server' }))

    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit.mock.calls[0][0].name).toBe('Primary')
  })
})
