import { remoteContentURL } from '../lib/explorerApi'

export function parentRemotePath(value) {
  const normalized = normalizeRemotePath(value)
  if (normalized === '/') return '/'
  return normalized.slice(0, normalized.lastIndexOf('/')) || '/'
}

export function normalizeRemotePath(value) {
  const parts = String(value || '/').split('/')
  const stack = []
  for (const part of parts) {
    if (!part || part === '.') continue
    if (part === '..') stack.pop()
    else stack.push(part)
  }
  return `/${stack.join('/')}`
}

export function resolveRemoteReference(filePath, reference) {
  if (!reference || reference.startsWith('#') || /^[a-z][a-z\d+.-]*:/i.test(reference) || reference.startsWith('//')) {
    return reference
  }
  const match = String(reference).match(/^([^?#]*)([?#].*)?$/)
  const referencePath = decodePath(match?.[1] || '')
  const suffix = match?.[2] || ''
  const base = referencePath.startsWith('/') ? referencePath : `${parentRemotePath(filePath)}/${referencePath}`
  return `${remoteContentURL(normalizeRemotePath(base))}${suffix}`
}

function decodePath(value) {
  try {
    return decodeURIComponent(value)
  } catch {
    return value
  }
}

export function editorLanguage(fileName) {
  const name = String(fileName || '').toLowerCase()
  const extension = name.includes('.') ? name.slice(name.lastIndexOf('.') + 1) : ''
  const languages = {
    bash: 'shell', c: 'c', cc: 'cpp', conf: 'ini', cpp: 'cpp', css: 'css',
    env: 'ini', go: 'go', h: 'c', hpp: 'cpp', htm: 'html', html: 'html',
    ini: 'ini', java: 'java', js: 'javascript', json: 'json', jsx: 'javascript',
    log: 'plaintext', md: 'markdown', php: 'php', py: 'python', rb: 'ruby',
    rs: 'rust', scss: 'scss', sh: 'shell', sql: 'sql', svelte: 'html',
    toml: 'ini', ts: 'typescript', tsx: 'typescript', txt: 'plaintext',
    xml: 'xml', yaml: 'yaml', yml: 'yaml', zsh: 'shell',
  }
  if (name === 'dockerfile') return 'dockerfile'
  if (name === 'makefile') return 'plaintext'
  return languages[extension] || 'plaintext'
}

export function formatFileSize(bytes) {
  if (!Number.isFinite(bytes) || bytes < 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`
}
