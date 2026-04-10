import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve(import.meta.dirname, '..')
const css = readFileSync(resolve(root, 'src/app.css'), 'utf8')

const requiredSelectors = [
  '.gap-sm',
  '.gap-md',
  '.gap-lg',
  '.is-selected',
  '.is-disabled',
  '.state-warning',
  '.state-error',
  '.field-error',
  '.dialog',
  '.dialog-body',
  '.dialog-actions',
  '.status-running',
  '.status-reconnecting',
  '.status-stopped',
  '.status-failed',
  '.status-attention',
]

const missing = requiredSelectors.filter((selector) => !css.includes(selector))

if (missing.length > 0) {
  console.error('Missing required CSS contract selectors:')
  for (const selector of missing) {
    console.error(`- ${selector}`)
  }
  process.exit(1)
}

console.log('CSS contract check passed.')
