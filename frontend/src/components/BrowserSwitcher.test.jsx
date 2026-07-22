import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import { BrowserSwitcher } from './BrowserSwitcher'

const browsers = [
  { id: 'browser:101', browserId: 'google-chrome', browserName: 'Chrome one', kind: 'regular' },
  { id: 'browser:202', browserId: 'google-chrome', browserName: 'Chrome two', kind: 'proxy', serverId: 'server-prod', serverName: 'Production' },
]

function renderSwitcher(overrides = {}) {
  const props = {
    items: browsers,
    selectedIndex: 0,
    loading: false,
    error: '',
    activating: false,
    mode: 'shortcut',
    forwardShortcut: 'Alt+X',
    backwardShortcut: 'Alt+Z',
    appearances: {},
    onSelect: vi.fn(),
    onActivate: vi.fn(),
    onSaveAppearance: vi.fn(async () => ({})),
    onRefresh: vi.fn(),
    onClose: vi.fn(),
    ...overrides,
  }
  return { ...render(<BrowserSwitcher {...props} />), props }
}

describe('BrowserSwitcher keyboard navigation', () => {
  test('uses arrow keys for option movement without trapping Tab', () => {
    const { props } = renderSwitcher()
    props.onSelect.mockClear()

    fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(props.onSelect).toHaveBeenLastCalledWith(1)
    fireEvent.keyDown(window, { key: 'ArrowLeft' })
    expect(props.onSelect).toHaveBeenLastCalledWith(1)

    const tabEvent = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true })
    window.dispatchEvent(tabEvent)
    expect(tabEvent.defaultPrevented).toBe(false)
    expect(props.onSelect).toHaveBeenCalledTimes(2)
  })

  test('does not cycle hidden options while an error is displayed', () => {
    const { props } = renderSwitcher({ error: 'Browser discovery failed.' })

    expect(screen.getByRole('alert')).toBeTruthy()
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(props.onSelect).not.toHaveBeenCalled()
  })
})

describe('BrowserSwitcher appearance customization', () => {
  test('renders a saved proxy color and mark without affecting regular Chrome', () => {
    renderSwitcher({
      selectedIndex: 1,
      appearances: {
        'proxy:server-prod:google-chrome': { icon: 'X', primaryColor: '#22C55E' },
      },
    })

    const regular = screen.getByRole('option', { name: /Chrome one/ })
    const proxy = screen.getByRole('option', { name: /Chrome two/ })
    expect(regular.classList.contains('has-custom-appearance')).toBe(false)
    expect(proxy.classList.contains('has-custom-appearance')).toBe(true)
    expect(proxy.style.getPropertyValue('--browser-primary')).toBe('#22C55E')
    expect(proxy.textContent).toContain('X')
  })

  test('saves a built-in X icon and green color from manual mode', async () => {
    const { props } = renderSwitcher({ mode: 'manual', selectedIndex: 1 })

    const customize = screen.getByRole('button', { name: 'Customize selected' })
    fireEvent.keyDown(customize, { key: 'Enter' })
    expect(props.onActivate).not.toHaveBeenCalled()
    fireEvent.click(customize)
    expect(screen.getByRole('heading', { name: 'Customize Chrome two' })).toBeTruthy()
    fireEvent.click(screen.getByRole('radio', { name: 'X icon' }))
    fireEvent.click(screen.getByRole('radio', { name: 'Green color' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(props.onSaveAppearance).toHaveBeenCalledWith(
      'proxy:server-prod:google-chrome',
      { icon: 'icon:x', primaryColor: '#22C55E' },
    ))
  })

  test('supports an emoji and resets the saved appearance', async () => {
    const { props } = renderSwitcher({
      mode: 'manual',
      appearances: {
        'regular:google-chrome': { icon: '👩🏽‍💻', primaryColor: '#8B5CF6' },
      },
    })

    fireEvent.click(screen.getByRole('button', { name: 'Customize selected' }))
    expect(screen.getByLabelText('Custom emoji or mark').value).toBe('👩🏽‍💻')
    fireEvent.click(screen.getByRole('button', { name: 'Use default' }))

    await waitFor(() => expect(props.onSaveAppearance).toHaveBeenCalledWith(
      'regular:google-chrome',
      { icon: '', primaryColor: '' },
    ))
  })
})
