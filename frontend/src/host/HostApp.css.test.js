import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, test } from 'vitest'

const stylesheet = readFileSync(
  resolve(process.cwd(), 'src/app.css'),
  'utf8',
)

function declarationBlock(marker) {
  const markerIndex = stylesheet.indexOf(marker)
  const start = stylesheet.indexOf('{', markerIndex)
  const end = stylesheet.indexOf('}', start)
  return stylesheet.slice(start + 1, end)
}

function variable(block, name) {
  const match = block.match(new RegExp(`--${name}:\\s*(#[0-9a-f]{6})`, 'i'))
  if (!match) throw new Error(`Missing --${name}`)
  return match[1]
}

function relativeLuminance(hex) {
  const channels = hex.match(/[0-9a-f]{2}/gi).map((value) => Number.parseInt(value, 16) / 255)
  const linear = channels.map((value) => (
    value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  ))
  return (0.2126 * linear[0]) + (0.7152 * linear[1]) + (0.0722 * linear[2])
}

function contrastRatio(first, second) {
  const light = Math.max(relativeLuminance(first), relativeLuminance(second))
  const dark = Math.min(relativeLuminance(first), relativeLuminance(second))
  return (light + 0.05) / (dark + 0.05)
}

describe('host port styles', () => {
  test('keeps the listening address above WCAG AA contrast in both themes', () => {
    expect(stylesheet).toMatch(/\.host-port-copy small\s*{\s*color:\s*var\(--text-muted\);/)

    for (const marker of [":root[data-theme='dark']", ":root[data-theme='light']"]) {
      const block = declarationBlock(marker)
      expect(contrastRatio(
        variable(block, 'text-muted'),
        variable(block, 'surface-raised'),
      )).toBeGreaterThanOrEqual(4.5)
    }
  })
})
