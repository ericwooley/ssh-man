const shellDelimiters = /[\s|&;()<>\n]/

export function activeShellWord(command, cursor = command.length) {
  const safeCursor = Math.max(0, Math.min(Number(cursor) || 0, command.length))
  let start = 0
  let quote = ''
  let escaped = false

  for (let index = 0; index < safeCursor; index += 1) {
    const character = command[index]
    if (escaped) {
      escaped = false
      continue
    }
    if (character === '\\' && quote !== "'") {
      escaped = true
      continue
    }
    if (quote) {
      if (character === quote) quote = ''
      continue
    }
    if (character === '"' || character === "'") {
      quote = character
      continue
    }
    if (shellDelimiters.test(character)) {
      start = index + 1
    }
  }

  const raw = command.slice(start, safeCursor)
  let value = ''
  quote = ''
  escaped = false
  for (const character of raw) {
    if (escaped) {
      value += character
      escaped = false
      continue
    }
    if (character === '\\' && quote !== "'") {
      escaped = true
      continue
    }
    if (quote) {
      if (character === quote) quote = ''
      else value += character
      continue
    }
    if (character === '"' || character === "'") {
      quote = character
    } else {
      value += character
    }
  }
  if (escaped) value += '\\'

  return { start, end: safeCursor, raw, value }
}

export function escapeShellPath(value) {
  return String(value).replace(/([^A-Za-z0-9_@%+=:,./~-])/g, '\\$1')
}

export function applyPathCompletion(command, context, value) {
  const replacement = escapeShellPath(value)
  const next = `${command.slice(0, context.start)}${replacement}${command.slice(context.end)}`
  return {
    command: next,
    cursor: context.start + replacement.length,
  }
}
