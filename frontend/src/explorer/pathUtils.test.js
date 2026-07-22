import { describe, expect, test } from 'vitest'
import { editorLanguage, formatFileSize, parentRemotePath, resolveRemoteReference } from './pathUtils'

describe('remote explorer paths', () => {
  test('resolves parent paths without leaving root', () => {
    expect(parentRemotePath('/var/www/index.html')).toBe('/var/www')
    expect(parentRemotePath('/')).toBe('/')
  })

  test('maps relative preview assets through the remote content route', () => {
    expect(resolveRemoteReference('/var/www/site/index.html', '../images/logo one.png'))
      .toBe('/__ssh_man_remote__/raw/var/www/images/logo%20one.png')
    expect(resolveRemoteReference('/var/www/README.md', 'https://example.com/a.png'))
      .toBe('https://example.com/a.png')
  })

  test('maps common source files to Monaco languages', () => {
    expect(editorLanguage('main.go')).toBe('go')
    expect(editorLanguage('.env')).toBe('ini')
    expect(editorLanguage('unknown.data')).toBe('plaintext')
  })

  test('formats byte counts compactly', () => {
    expect(formatFileSize(512)).toBe('512 B')
    expect(formatFileSize(1536)).toBe('1.5 KB')
  })
})
