const matchModes = new Set(['regex', 'starts_with', 'ends_with', 'contains'])

function directCommandTemplateError(template) {
  const arguments_ = []
  let argument = ''
  let quote = ''
  let wordStarted = false
  let replacedURL = false

  const finishArgument = () => {
    if (!wordStarted) return
    arguments_.push(argument)
    argument = ''
    wordStarted = false
  }

  for (let index = 0; index < template.length;) {
    if (template.startsWith('<URL>', index)) {
      argument += 'https://ssh-man.invalid/'
      wordStarted = true
      replacedURL = true
      index += '<URL>'.length
      continue
    }

    const value = template[index]
    if (!quote && [' ', '\t', '\r', '\n'].includes(value)) {
      finishArgument()
      index += 1
      continue
    }
    if (!quote && '|&;<>'.includes(value)) {
      return 'Use a direct command with quoted arguments; shell operators and redirects are not supported.'
    }
    if (value === '\\' && quote !== "'") {
      if (index + 1 >= template.length) {
        return 'Close all command quotes and escapes.'
      }
      argument += template[index + 1]
      wordStarted = true
      index += 2
      continue
    }
    if ((value === "'" || value === '"')) {
      if (!quote) {
        quote = value
        wordStarted = true
        index += 1
        continue
      }
      if (quote === value) {
        quote = ''
        index += 1
        continue
      }
    }
    argument += value
    wordStarted = true
    index += 1
  }

  if (quote) return 'Close all command quotes and escapes.'
  finishArgument()
  if (!replacedURL) return 'Include <URL> in the command.'
  if (!arguments_.length || !arguments_[0]) return 'Add the command to run.'

  if (!['open', '/usr/bin/open'].includes(arguments_[0])) {
    return 'Use an open command, such as open -a "Browser" "<URL>".'
  }
  if (arguments_.slice(1).includes('--args')) {
    return 'Use an open command that sends the link directly to the selected app.'
  }
  return ''
}

export function normalizeURLRule(rule = {}) {
  const matchMode = matchModes.has(rule.matchMode) ? rule.matchMode : 'regex'
  return {
    ...rule,
    matchMode,
    openDirect: Boolean(rule.openDirect),
  }
}

export function settingsBrowserChoices(catalog = [], customBrowsers = [], disabledBrowserIds = []) {
  const disabled = new Set(disabledBrowserIds)
  const choices = []
  const seen = new Set()

  for (const browser of catalog) {
    if (!browser?.id || disabled.has(browser.id) || seen.has(browser.id)) continue
    seen.add(browser.id)
    choices.push({
      ...browser,
      custom: false,
    })
  }

  for (const browser of customBrowsers) {
    if (!browser?.id || disabled.has(browser.id) || seen.has(browser.id)) continue
    seen.add(browser.id)
    const legacy = !browser.command && Boolean(browser.launchReference)
    choices.push({
      id: browser.id,
      displayName: browser.displayName || 'Unnamed custom browser',
      command: browser.command || '',
      icon: browser.icon || '',
      supportsProxyLaunch: legacy && ['chromium', 'firefox'].includes(browser.engine),
      custom: true,
      legacy,
    })
  }

  return choices
}

export function reconcileBrowserReferences(input, _catalog = [], removedBrowserIds = []) {
  const draft = {
    ...input,
    disabledBrowserIds: [...(input.disabledBrowserIds || [])],
    customBrowsers: [...(input.customBrowsers || [])],
    urlRules: (input.urlRules || []).map(normalizeURLRule),
    urlPortAssignments: [...(input.urlPortAssignments || [])],
  }
  const removed = new Set(removedBrowserIds)

  if (removed.has(draft.defaultBrowserId)) {
    draft.defaultBrowserId = ''
  }
  if (removed.has(draft.proxyBrowserId)) {
    draft.proxyBrowserId = ''
  }

  draft.urlRules = draft.urlRules.map((rule) => (
    rule.action === 'browser' && removed.has(rule.browserId)
      ? { ...rule, browserId: '' }
      : rule
  ))
  draft.urlPortAssignments = draft.urlPortAssignments.map((assignment) => (
    removed.has(assignment.browserId)
      ? { ...assignment, browserId: '' }
      : assignment
  ))

  return draft
}

export function setBrowserEnabled(input, browserId, enabled, catalog = []) {
  const disabled = new Set(input.disabledBrowserIds || [])
  if (enabled) disabled.delete(browserId)
  else disabled.add(browserId)
  return reconcileBrowserReferences({
    ...input,
    disabledBrowserIds: [...disabled],
  }, catalog, enabled ? [] : [browserId])
}

export function removeCustomBrowser(input, browserId, catalog = []) {
  return reconcileBrowserReferences({
    ...input,
    customBrowsers: (input.customBrowsers || []).filter((browser) => browser.id !== browserId),
    disabledBrowserIds: (input.disabledBrowserIds || []).filter((id) => id !== browserId),
  }, catalog, [browserId])
}

function persistedCommandsById(items = [], include = () => true) {
  const commands = new Map()
  for (const item of items) {
    const id = String(item?.id || '').trim()
    const command = String(item?.command || '').trim()
    if (id && command && include(item)) commands.set(id, command)
  }
  return commands
}

export function settingsDraftErrors(draft, catalog = [], persistedDraft = {}) {
  const errors = {}
  const browserChoices = settingsBrowserChoices(catalog, draft.customBrowsers, draft.disabledBrowserIds)
  const browserIds = new Set(browserChoices.map((browser) => browser.id))
  const proxyIds = new Set(browserChoices.filter((browser) => browser.supportsProxyLaunch).map((browser) => browser.id))
  const persistedBrowserCommands = persistedCommandsById(persistedDraft.customBrowsers)
  const persistedRuleCommands = persistedCommandsById(
    persistedDraft.urlRules,
    (rule) => rule.action === 'command',
  )

  for (const browser of draft.customBrowsers || []) {
    if (!browser.displayName?.trim()) {
      errors[`browser:${browser.id}:name`] = 'Add a name for this browser.'
    }
    if (browser.command) {
      const commandError = directCommandTemplateError(browser.command)
      const unchanged = persistedBrowserCommands.get(String(browser.id || '').trim()) === String(browser.command).trim()
      if (commandError && !unchanged) errors[`browser:${browser.id}:command`] = commandError
    } else if (!browser.launchReference) {
      errors[`browser:${browser.id}:command`] = 'Add a command that includes <URL>.'
    }
  }

  for (const rule of draft.urlRules || []) {
    if (!rule.pattern?.trim()) {
      errors[`rule:${rule.id}:pattern`] = 'Add a value to match.'
    }
    if (rule.action === 'browser' && !browserIds.has(rule.browserId)) {
      errors[`rule:${rule.id}:browser`] = 'Choose an enabled browser.'
    }
    if (rule.action === 'command') {
      const commandError = directCommandTemplateError(rule.command || '')
      const unchanged = persistedRuleCommands.get(String(rule.id || '').trim()) === String(rule.command || '').trim()
      if (commandError && !unchanged) errors[`rule:${rule.id}:command`] = commandError
    }
  }

  for (const assignment of draft.urlPortAssignments || []) {
    const port = Number(assignment.port)
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      errors[`port:${assignment.id}:port`] = 'Enter a port from 1 to 65535.'
    }
    if (!assignment.serverId) {
      errors[`port:${assignment.id}:server`] = 'Choose a host.'
    }
    if (!proxyIds.has(assignment.browserId)) {
      errors[`port:${assignment.id}:browser`] = 'Choose a proxy-capable browser.'
    }
  }

  return errors
}
