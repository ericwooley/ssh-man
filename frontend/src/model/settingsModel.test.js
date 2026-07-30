import { describe, expect, it } from 'vitest'

import {
  normalizeURLRule,
  reconcileBrowserReferences,
  removeCustomBrowser,
  setBrowserEnabled,
  settingsBrowserChoices,
  settingsDraftErrors,
} from './settingsModel'

const catalog = [
  { id: 'chrome', displayName: 'Chrome', supportsProxyLaunch: true },
  { id: 'safari', displayName: 'Safari', supportsProxyLaunch: false },
]

function draft(overrides = {}) {
  return {
    defaultBrowserId: 'chrome',
    proxyBrowserId: 'chrome',
    disabledBrowserIds: [],
    customBrowsers: [],
    urlRules: [],
    urlPortAssignments: [],
    ...overrides,
  }
}

describe('settings browser choices', () => {
  it('excludes disabled installed and custom browsers from consumer choices', () => {
    expect(settingsBrowserChoices(catalog, [{
      id: 'work',
      displayName: 'Work',
      command: 'open <URL>',
      icon: 'icon:briefcase',
    }], ['safari']).map((browser) => browser.id)).toEqual(['chrome', 'work'])
  })

  it('treats command browsers as regular browser destinations', () => {
    expect(settingsBrowserChoices([], [{
      id: 'work',
      displayName: 'Work',
      command: 'open <URL>',
      icon: 'icon:briefcase',
    }], [])[0]).toMatchObject({
      supportsProxyLaunch: false,
      custom: true,
      icon: 'icon:briefcase',
    })
  })
})

describe('browser reference reconciliation', () => {
  it('clears every selected destination when its browser is disabled', () => {
    const result = setBrowserEnabled(draft({
      urlRules: [{ id: 'rule-1', pattern: 'example', action: 'browser', browserId: 'chrome' }],
      urlPortAssignments: [{ id: 'port-1', port: 3000, serverId: 'server-1', browserId: 'chrome' }],
    }), 'chrome', false, catalog)

    expect(result.defaultBrowserId).toBe('')
    expect(result.proxyBrowserId).toBe('')
    expect(result.urlRules[0].browserId).toBe('')
    expect(result.urlPortAssignments[0].browserId).toBe('')
    expect(settingsDraftErrors(result, catalog)).toMatchObject({
      'rule:rule-1:browser': 'Choose an enabled browser.',
      'port:port-1:browser': 'Choose a proxy-capable browser.',
    })
  })

  it('removes a custom browser without silently retargeting its defaults or rules', () => {
    const result = removeCustomBrowser(draft({
      defaultBrowserId: 'work',
      customBrowsers: [{ id: 'work', displayName: 'Work', command: 'open <URL>' }],
      urlRules: [{ id: 'rule-1', pattern: 'example', action: 'browser', browserId: 'work' }],
    }), 'work', catalog)

    expect(result.defaultBrowserId).toBe('')
    expect(result.urlRules[0].browserId).toBe('')
  })

  it('normalizes legacy rules to regex without changing their pattern', () => {
    expect(normalizeURLRule({ pattern: '^https:', action: 'browser' })).toMatchObject({
      pattern: '^https:',
      matchMode: 'regex',
      openDirect: false,
    })
    expect(reconcileBrowserReferences(draft({
      urlRules: [{ id: 'rule-1', pattern: '^https:', action: 'browser', browserId: 'chrome' }],
    }), catalog).urlRules[0].matchMode).toBe('regex')
  })

  it('preserves saved references when a browser is temporarily unavailable', () => {
    const result = reconcileBrowserReferences(draft({
      defaultBrowserId: 'arc',
      proxyBrowserId: 'arc',
      urlRules: [{ id: 'rule-1', pattern: 'example', action: 'browser', browserId: 'arc' }],
      urlPortAssignments: [{ id: 'port-1', port: 3000, serverId: 'server-1', browserId: 'arc' }],
    }), catalog)

    expect(result.defaultBrowserId).toBe('arc')
    expect(result.proxyBrowserId).toBe('arc')
    expect(result.urlRules[0].browserId).toBe('arc')
    expect(result.urlPortAssignments[0].browserId).toBe('arc')
  })

  it('keeps legacy Chromium and Firefox custom browsers proxy-capable', () => {
    const customBrowsers = [{
      id: 'legacy',
      displayName: 'Legacy browser',
      launchReference: '/Applications/Legacy.app',
      engine: 'chromium',
    }]
    const choices = settingsBrowserChoices([], customBrowsers, [])
    const result = reconcileBrowserReferences(draft({
      defaultBrowserId: 'legacy',
      proxyBrowserId: 'legacy',
      customBrowsers,
      urlPortAssignments: [{ id: 'port-1', port: 3000, serverId: 'server-1', browserId: 'legacy' }],
    }), [])

    expect(choices[0].supportsProxyLaunch).toBe(true)
    expect(result.proxyBrowserId).toBe('legacy')
    expect(result.urlPortAssignments[0].browserId).toBe('legacy')
  })
})

describe('settings draft validation', () => {
  it('requires command browsers and command rules to include the URL placeholder', () => {
    const errors = settingsDraftErrors(draft({
      customBrowsers: [{ id: 'work', displayName: 'Work', command: 'open example.com' }],
      urlRules: [{ id: 'rule-1', matchMode: 'contains', pattern: 'example', action: 'command', command: 'open example.com' }],
    }), catalog)

    expect(errors['browser:work:command']).toContain('<URL>')
    expect(errors['rule:rule-1:command']).toContain('<URL>')
  })

  it('rejects shell grammar and nested command-string interpreters before saving', () => {
    const errors = settingsDraftErrors(draft({
      customBrowsers: [{
        id: 'work',
        displayName: 'Work',
        command: `printf '%s' <URL> | open`,
      }],
      urlRules: [{
        id: 'rule-1',
        matchMode: 'contains',
        pattern: 'example',
        action: 'command',
        command: `/usr/bin/nice /bin/sh -c "open <URL>"`,
      }],
    }), catalog)

    expect(errors['browser:work:command']).toContain('direct command')
    expect(errors['rule:rule-1:command']).toContain('open command')
  })

  it('allows unchanged legacy commands to remain repairable', () => {
    const legacyDraft = draft({
      customBrowsers: [{
        id: 'legacy-browser',
        displayName: 'Legacy browser',
        command: `/bin/zsh -lc "open <URL>"`,
      }],
      urlRules: [{
        id: 'legacy-rule',
        matchMode: 'contains',
        pattern: 'example.com',
        action: 'command',
        command: `/bin/zsh -lc "open <URL>"`,
      }],
    })

    expect(settingsDraftErrors(legacyDraft, catalog, legacyDraft)).toEqual({})
  })

  it('still rejects new or edited legacy command syntax', () => {
    const persisted = draft({
      customBrowsers: [{
        id: 'legacy-browser',
        displayName: 'Legacy browser',
        command: `/bin/zsh -lc "open <URL>"`,
      }],
      urlRules: [{
        id: 'legacy-rule',
        matchMode: 'contains',
        pattern: 'example.com',
        action: 'command',
        command: `/bin/zsh -lc "open <URL>"`,
      }],
    })
    const changed = {
      ...persisted,
      customBrowsers: persisted.customBrowsers.concat({
        id: 'new-browser',
        displayName: 'New browser',
        command: `/bin/zsh -lc "open <URL>"`,
      }),
      urlRules: persisted.urlRules.map((rule) => ({
        ...rule,
        command: `/bin/sh -lc "open <URL>"`,
      })),
    }

    const errors = settingsDraftErrors(changed, catalog, persisted)

    expect(errors['browser:new-browser:command']).toContain('open command')
    expect(errors['rule:legacy-rule:command']).toContain('open command')
  })

  it('accepts quoted direct-command arguments and URL embedding', () => {
    expect(settingsDraftErrors(draft({
      customBrowsers: [{
        id: 'work',
        displayName: 'Work',
        command: `open -a "Work Browser" "container:<URL>"`,
      }],
    }), catalog)).toEqual({})
  })

  it('accepts the absolute macOS open path', () => {
    expect(settingsDraftErrors(draft({
      customBrowsers: [{
        id: 'work',
        displayName: 'Work',
        command: `/usr/bin/open <URL>`,
      }],
    }), catalog)).toEqual({})
  })

  it.each([
    ['non-breaking space', `open\u00a0<URL>`],
    ['form feed', `open\f<URL>`],
    ['vertical tab', `open\v<URL>`],
  ])('matches the backend command grammar for %s', (_name, command) => {
    const errors = settingsDraftErrors(draft({
      customBrowsers: [{ id: 'work', displayName: 'Work', command }],
    }), catalog)

    expect(errors['browser:work:command']).toContain('open command')
  })

  it.each([
    `python3.13 -c "print('<URL>')"`,
    `nodejs --eval "console.log('<URL>')"`,
    `powershell.exe -Command "Write-Output '<URL>'"`,
    `cmd.exe /c "echo <URL>"`,
    `python3 "-cprint('<URL>')"`,
    `node "--eval=console.log('<URL>')"`,
    `perl5.34 "-eprint('<URL>')"`,
    `lua5.5 "-eprint('<URL>')"`,
    `/usr/bin/caffeinate /bin/sh -c "printf '%s' <URL>"`,
    `cmd.exe "/cecho <URL>"`,
    `awk "BEGIN { system(\\"printf %s <URL>\\") }"`,
    `awk <URL>`,
    `/usr/bin/caffeinate /usr/bin/osascript -e "do shell script \\"echo <URL>\\""`,
    `/usr/bin/caffeinate /usr/bin/env -S "printf %s <URL>"`,
    `/usr/bin/caffeinate parallel <URL> ::: 1`,
    `su -c <URL>`,
    `ssh host <URL>`,
    `script -c <URL>`,
    `csh -c <URL>`,
    `open -a Terminal --args -c <URL>`,
    `/tmp/open <URL>`,
    `./open <URL>`,
    `OPEN.EXE <URL>`,
    `<URL>/open`,
  ])('rejects interpreter command variant %s', (command) => {
    const errors = settingsDraftErrors(draft({
      customBrowsers: [{ id: 'work', displayName: 'Work', command }],
    }), catalog)

    expect(errors['browser:work:command']).toContain('open command')
  })

  it('accepts a valid literal direct-open browser rule', () => {
    expect(settingsDraftErrors(draft({
      urlRules: [{
        id: 'rule-1',
        matchMode: 'starts_with',
        pattern: 'https://work.example/',
        action: 'browser',
        browserId: 'chrome',
        openDirect: true,
      }],
    }), catalog)).toEqual({})
  })

  it('leaves Go regex syntax to the authoritative backend validator', () => {
    expect(settingsDraftErrors(draft({
      urlRules: [{
        id: 'rule-1',
        matchMode: 'regex',
        pattern: '(?P<name>work)',
        action: 'browser',
        browserId: 'chrome',
      }],
    }), catalog)).toEqual({})
  })
})
