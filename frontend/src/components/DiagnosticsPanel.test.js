import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import DiagnosticsPanel from './DiagnosticsPanel.svelte'

describe('DiagnosticsPanel', () => {
  it('renders storage locations and recovery actions', async () => {
    const onReload = vi.fn()
    const onOpenDevTools = vi.fn()
    const onCopyPath = vi.fn()

    render(DiagnosticsPanel, {
      props: {
        diagnostics: {
          appDataPath: '/tmp/ssh-man',
          databasePath: '/tmp/ssh-man/ssh-man.db',
        },
        issue: 'database is locked',
        hasWarning: true,
        onReload,
        onOpenDevTools,
        onCopyPath,
      },
    })

    expect(screen.getByText('/tmp/ssh-man')).toBeTruthy()
    expect(screen.getByText('/tmp/ssh-man/ssh-man.db')).toBeTruthy()
    expect(screen.getByText('database is locked')).toBeTruthy()

    await fireEvent.click(screen.getAllByRole('button', { name: 'Copy path' })[0])
    await fireEvent.click(screen.getByRole('button', { name: 'Reload saved data' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Open devtools' }))

    expect(onCopyPath).toHaveBeenCalledWith('App data path', '/tmp/ssh-man')
    expect(onReload).toHaveBeenCalledTimes(1)
    expect(onOpenDevTools).toHaveBeenCalledTimes(1)
  })
})
