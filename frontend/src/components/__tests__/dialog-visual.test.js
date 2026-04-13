import { render, screen } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'

import ConfigEditor from '../ConfigEditor.svelte'
import ServerEditorDialog from '../ServerEditorDialog.svelte'

describe('dialog visual contract', () => {
  it('renders the server dialog with shared dialog classes', () => {
    const { container } = render(ServerEditorDialog, {
      props: {
        open: true,
        value: {
          id: '',
          name: '',
          host: '',
          port: '22',
          username: '',
          authMode: 'agent',
          keyReference: '',
        },
      },
    })

    expect(container.querySelector('.dialog-card.dialog')).toBeTruthy()
    expect(container.querySelector('.dialog-body')).toBeTruthy()
    expect(container.querySelector('.dialog-actions')).toBeTruthy()
  })

  it('renders field error classes for invalid tunnel fields', () => {
    const { container } = render(ConfigEditor, {
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
        },
      },
    })

    expect(container.querySelectorAll('.field-error').length).toBeGreaterThan(0)
    expect(screen.getByText('A label is required.')).toBeTruthy()
  })
})
