import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import UnlockKeyDialog from './UnlockKeyDialog.svelte'

describe('UnlockKeyDialog', () => {
  it('collects the passphrase directly in the modal', async () => {
    const onSubmit = vi.fn()
    const onClose = vi.fn()

    render(UnlockKeyDialog, {
      props: {
        open: true,
        configurationLabel: 'Staging SOCKS',
        detail: 'Unlock the SSH key to continue',
        onSubmit,
        onClose,
      },
    })

    expect(screen.getByText('Enter the passphrase for Staging SOCKS to continue.')).toBeTruthy()

    await fireEvent.input(screen.getByLabelText('SSH key passphrase'), { target: { value: 'hunter2' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Unlock key' }))

    expect(onSubmit).toHaveBeenCalledWith('hunter2')
  })

  it('disables submission until a passphrase is provided', () => {
    render(UnlockKeyDialog, {
      props: {
        open: true,
      },
    })

    expect(screen.getByRole('button', { name: 'Unlock key' }).hasAttribute('disabled')).toBe(true)
  })
})
