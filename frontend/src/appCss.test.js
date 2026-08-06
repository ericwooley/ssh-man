import { readFileSync } from 'node:fs'
import { describe, expect, test } from 'vitest'

const css = readFileSync(`${process.cwd()}/src/app.css`, 'utf8').replace(/\r\n?/g, '\n')

function themeToken(themeMarker, token) {
  const start = css.indexOf(themeMarker)
  const end = css.indexOf('\n}', start)
  const block = css.slice(start, end)
  const match = block.match(new RegExp(`--${token}:\\s*(#[0-9a-fA-F]{6})`))
  if (!match) throw new Error(`Missing --${token} in ${themeMarker}`)
  return match[1]
}

function relativeLuminance(hex) {
  const channels = hex.match(/[0-9a-f]{2}/gi).map((value) => Number.parseInt(value, 16) / 255)
  const [red, green, blue] = channels.map((value) => (
    value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  ))
  return 0.2126 * red + 0.7152 * green + 0.0722 * blue
}

function contrastRatio(first, second) {
  const [lighter, darker] = [relativeLuminance(first), relativeLuminance(second)].sort((a, b) => b - a)
  return (lighter + 0.05) / (darker + 0.05)
}

function ruleBodies(selector) {
  const bodies = []
  let offset = 0
  while (offset < css.length) {
    const start = css.indexOf(selector, offset)
    if (start < 0) break
    const end = css.indexOf('\n}', start)
    if (end < 0) throw new Error(`Unterminated rule for ${selector}`)
    bodies.push(css.slice(start, end))
    offset = end + 2
  }
  if (!bodies.length) throw new Error(`Missing rule for ${selector}`)
  return bodies
}

describe('settings color tokens', () => {
  test.each([
    [":root[data-theme='dark']", ['surface', 'surface-raised', 'surface-strong', 'surface-soft']],
    [":root[data-theme='light']", ['surface', 'surface-raised', 'surface-strong', 'surface-soft']],
  ])('keeps small secondary text readable in %s', (themeMarker, surfaces) => {
    const foreground = themeToken(themeMarker, 'settings-text-secondary')

    for (const surface of surfaces) {
      expect(contrastRatio(foreground, themeToken(themeMarker, surface))).toBeGreaterThanOrEqual(4.5)
    }
  })

  test.each([
    '.settings-sidebar__footer > span {',
    '.settings-master-list__heading span {',
    '.settings-master-list li > button small {',
    '.browser-detail__identity span,\n.rule-detail__heading span {',
    '.settings-form-grid small {',
    '.browser-facts dt {',
    '.settings-save-bar > span {',
    '.rule-order {',
  ])('uses the accessible secondary token for %s', (selector) => {
    expect(ruleBodies(selector).some((body) => (
      body.includes('color: var(--settings-text-secondary)')
    ))).toBe(true)
  })

  test('shows keyboard focus on the visible custom-browser icon tile', () => {
    expect(ruleBodies('.browser-icon-picker input:focus-visible + span {').some((body) => (
      body.includes('box-shadow: var(--focus)')
    ))).toBe(true)
  })
})
