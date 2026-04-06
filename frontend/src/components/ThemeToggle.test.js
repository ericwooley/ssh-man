import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'

import ThemeToggle from './ThemeToggle.svelte'

describe('ThemeToggle', () => {
  it('announces the current theme and toggles on click', async () => {
    const onToggle = vi.fn()

    render(ThemeToggle, { props: { theme: 'dark', onToggle } })

    expect(screen.getByRole('button', { name: 'Toggle light and dark theme' }).textContent).toContain('Dark mode')

    await fireEvent.click(screen.getByRole('button', { name: 'Toggle light and dark theme' }))

    expect(onToggle).toHaveBeenCalledTimes(1)
  })
})
