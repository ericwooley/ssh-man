import { describe, expect, test } from 'vitest'
import { activeShellWord, applyPathCompletion, escapeShellPath } from './commandCompletion'

describe('command path completion', () => {
  test('extracts escaped and quoted path words', () => {
    const escaped = 'cat src/My\\ Fi'
    const quoted = 'cat "src/My Fi'
    expect(activeShellWord(escaped, escaped.length)).toMatchObject({ start: 4, value: 'src/My Fi' })
    expect(activeShellWord(quoted, quoted.length)).toMatchObject({ start: 4, value: 'src/My Fi' })
  })

  test('treats shell operators as word boundaries', () => {
    expect(activeShellWord('printf ok | cat lo')).toMatchObject({ value: 'lo' })
  })

  test('replaces only the active word and escapes spaces', () => {
    const command = 'cat src/My\\ Fi --verbose'
    const context = activeShellWord(command, 'cat src/My\\ Fi'.length)
    expect(applyPathCompletion(command, context, 'src/My File.txt')).toEqual({
      command: 'cat src/My\\ File.txt --verbose',
      cursor: 20,
    })
    expect(escapeShellPath("a'b")).toBe("a\\'b")
  })
})
