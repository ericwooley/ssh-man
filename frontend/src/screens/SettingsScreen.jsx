import { useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowDown,
  ArrowUp,
  BadgeInfo,
  Check,
  Copy,
  Database,
  FolderOpen,
  Globe2,
  Keyboard,
  LoaderCircle,
  Moon,
  Network,
  Plus,
  Power,
  RefreshCw,
  Route,
  Settings2,
  SquareTerminal,
  Sun,
  Trash2,
  Wifi,
  WifiOff,
  X,
} from 'lucide-react'

import { EmptyState, IconButton } from '../components/AppChrome'
import {
  BROWSER_APPEARANCE_ICON_OPTIONS,
  BrowserAppearanceMark,
} from '../components/BrowserAppearance'
import { UnsavedSettingsDialog } from '../components/Dialogs'
import { shortcutFromKeyboardEvent } from '../model/appModel'
import {
  normalizeURLRule,
  reconcileBrowserReferences,
  removeCustomBrowser,
  setBrowserEnabled,
  settingsBrowserChoices,
  settingsDraftErrors,
} from '../model/settingsModel'

const settingsPages = [
  { id: 'general', label: 'General', Icon: Settings2 },
  { id: 'browsers', label: 'Browsers', Icon: Globe2 },
  { id: 'routing', label: 'URL routing', Icon: Route },
  { id: 'health', label: 'App health', Icon: BadgeInfo },
]

const matchModeOptions = [
  { value: 'starts_with', label: 'Starts with' },
  { value: 'ends_with', label: 'Ends with' },
  { value: 'contains', label: 'Contains' },
  { value: 'regex', label: 'Regex' },
]

const matchModeLabels = Object.fromEntries(matchModeOptions.map((option) => [option.value, option.label]))

const matchModePlaceholders = {
  starts_with: 'https://github.com/',
  ends_with: '/login',
  contains: 'github.com',
  regex: String.raw`^https://github\.com/`,
}

function settingsDraft(preferences, catalog) {
  const input = {
    defaultBrowserId: preferences.defaultBrowserId || '',
    proxyBrowserId: preferences.proxyBrowserId || '',
    disabledBrowserIds: preferences.disabledBrowserIds || [],
    customBrowsers: preferences.customBrowsers || [],
    urlRules: (preferences.urlRules || []).map(normalizeURLRule),
    urlPortAssignments: preferences.urlPortAssignments || [],
  }
  return reconcileBrowserReferences(input, catalog, input.disabledBrowserIds)
}

function browserDraftSignature(draft) {
  return JSON.stringify({
    defaultBrowserId: draft.defaultBrowserId,
    proxyBrowserId: draft.proxyBrowserId,
    disabledBrowserIds: draft.disabledBrowserIds,
    customBrowsers: draft.customBrowsers,
  })
}

function routingDraftSignature(draft) {
  return JSON.stringify({
    urlRules: draft.urlRules,
    urlPortAssignments: draft.urlPortAssignments,
  })
}

function settingsDraftSignature(draft) {
  return `${browserDraftSignature(draft)}\n${routingDraftSignature(draft)}`
}

function regexRulesSignature(rules) {
  return JSON.stringify(rules.map((rule) => [rule.id, rule.matchMode, rule.pattern]))
}

function settingsErrorCounts(errors) {
  return Object.keys(errors).reduce((counts, key) => {
    if (key.startsWith('browser:')) counts.browsers += 1
    if (key.startsWith('rule:') || key.startsWith('port:')) counts.routing += 1
    return counts
  }, { browsers: 0, routing: 0 })
}

function settingsSaveMessage(errorCounts, dirty) {
  if (errorCounts.browsers && errorCounts.routing) {
    const total = errorCounts.browsers + errorCounts.routing
    return `${total} fields need attention across Browsers and URL routing.`
  }
  if (errorCounts.browsers) {
    return `${errorCounts.browsers} browser field${errorCounts.browsers === 1 ? '' : 's'} need${errorCounts.browsers === 1 ? 's' : ''} attention.`
  }
  if (errorCounts.routing) {
    return `${errorCounts.routing} URL routing field${errorCounts.routing === 1 ? '' : 's'} need${errorCounts.routing === 1 ? 's' : ''} attention.`
  }
  return dirty ? 'Unsaved browser and routing changes are ready to save.' : 'All browser and routing settings are saved.'
}

function PathCard({ icon: Icon, label, value, onCopy }) {
  return (
    <div className="path-card">
      <Icon aria-hidden="true" />
      <div>
        <span>{label}</span>
        <code>{value || 'Unavailable'}</code>
      </div>
      <IconButton label={`Copy ${label.toLowerCase()}`} disabled={!value} onClick={() => onCopy(label, value)}>
        <Copy aria-hidden="true" />
      </IconButton>
    </div>
  )
}

function ShortcutRecorder({ id, label, description, ariaLabel, value, fallback, onChange }) {
  const [recording, setRecording] = useState(false)
  const [message, setMessage] = useState('')
  const messageId = `${id}-message`

  function handleKeyDown(event) {
    if (!recording) return
    event.preventDefault()
    event.stopPropagation()
    if (event.key === 'Escape') {
      setRecording(false)
      setMessage('')
      return
    }
    const shortcut = shortcutFromKeyboardEvent(event)
    if (!shortcut) {
      setMessage('Include Control, Alt, or Command plus a supported key.')
      return
    }
    setRecording(false)
    setMessage('')
    void onChange(shortcut)
  }

  return (
    <div className="shortcut-setting">
      <div className="shortcut-setting__copy">
        <span className="shortcut-setting__icon" aria-hidden="true"><Keyboard /></span>
        <div>
          <strong>{label}</strong>
          <span>{description}</span>
        </div>
      </div>
      <button
        className={`shortcut-recorder ${recording ? 'is-recording' : ''}`}
        type="button"
        aria-label={ariaLabel}
        aria-describedby={message ? messageId : undefined}
        onClick={() => {
          setRecording(true)
          setMessage('Press the new shortcut. Escape cancels.')
        }}
        onKeyDown={handleKeyDown}
        onBlur={() => {
          setRecording(false)
          setMessage('')
        }}
      >
        {recording ? 'Press keys…' : <kbd>{value || fallback}</kbd>}
      </button>
      {message ? <p id={messageId} className="shortcut-setting__message">{message}</p> : null}
    </div>
  )
}

function PageHeading({ eyebrow, title, description, action }) {
  return (
    <header className="settings-page-heading">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h2>{title}</h2>
        {description ? <p>{description}</p> : null}
      </div>
      {action}
    </header>
  )
}

function SaveBar({ saving, disabled, hasErrors, message, onSave, onRetryValidation }) {
  return (
    <div className="settings-save-bar">
      <span role="status">{message}</span>
      {onRetryValidation ? (
        <button className="secondary-button" type="button" onClick={onRetryValidation}>
          <RefreshCw aria-hidden="true" /> Retry validation
        </button>
      ) : null}
      <button className="primary-button" type="button" disabled={saving || disabled || hasErrors} onClick={onSave}>
        {saving ? <LoaderCircle className="spin" aria-hidden="true" /> : <Check aria-hidden="true" />}
        Save all settings
      </button>
    </div>
  )
}

function GeneralSettings({
  preferences,
  onToggleTheme,
  onSetBrowserSwitcherShortcut,
  onSetBrowserSwitcherBackwardShortcut,
  onOpenBrowserSwitcher,
}) {
  return (
    <div className="settings-page">
      <PageHeading eyebrow="Preferences" title="General" description="Choose the app theme and quick browser switching shortcuts." />

      <section className="section-block" aria-labelledby="appearance-heading">
        <div className="section-heading">
          <div><h3 id="appearance-heading">Appearance</h3></div>
        </div>
        <div className="segmented-control" role="group" aria-label="Color theme">
          <button type="button" className={preferences.theme === 'light' ? 'is-selected' : ''} aria-pressed={preferences.theme === 'light'} onClick={preferences.theme === 'dark' ? onToggleTheme : undefined}>
            <Sun aria-hidden="true" /> Light {preferences.theme === 'light' ? <Check aria-hidden="true" /> : null}
          </button>
          <button type="button" className={preferences.theme === 'dark' ? 'is-selected' : ''} aria-pressed={preferences.theme === 'dark'} onClick={preferences.theme === 'light' ? onToggleTheme : undefined}>
            <Moon aria-hidden="true" /> Dark {preferences.theme === 'dark' ? <Check aria-hidden="true" /> : null}
          </button>
        </div>
      </section>

      <section className="section-block" aria-labelledby="shortcuts-heading">
        <div className="section-heading">
          <div><h3 id="shortcuts-heading">Quick browser switching</h3></div>
          <button className="text-button" type="button" onClick={onOpenBrowserSwitcher}>Customize</button>
        </div>
        <p className="section-description">Open the browser switcher in either direction from anywhere.</p>
        <ShortcutRecorder
          id="browser-switcher-forward-shortcut"
          label="Next browser"
          description="Opens the switcher and moves forward."
          ariaLabel="Next browser shortcut"
          value={preferences.browserSwitcherShortcut}
          fallback="Alt+X"
          onChange={onSetBrowserSwitcherShortcut}
        />
        <ShortcutRecorder
          id="browser-switcher-backward-shortcut"
          label="Previous browser"
          description="Opens the switcher and moves backward."
          ariaLabel="Previous browser shortcut"
          value={preferences.browserSwitcherBackwardShortcut}
          fallback="Alt+Z"
          onChange={onSetBrowserSwitcherBackwardShortcut}
        />
      </section>
    </div>
  )
}

function BrowserSettings({
  draft,
  catalog,
  errors,
  selectedBrowserId,
  customBrowserCommandsSupported,
  onSelectBrowser,
  onChange,
  onSave,
  saving,
  loading,
  hasErrors,
  saveMessage,
  onRetryValidation,
}) {
  const managementBrowsers = [
    ...catalog.map((browser) => ({ ...browser, custom: false })),
    ...draft.customBrowsers.map((browser) => ({
      ...browser,
      supportsProxyLaunch: !browser.command
        && Boolean(browser.launchReference)
        && ['chromium', 'firefox'].includes(browser.engine),
      custom: true,
    })),
  ]
  const activeChoices = settingsBrowserChoices(catalog, draft.customBrowsers, draft.disabledBrowserIds)
  const proxyChoices = activeChoices.filter((browser) => browser.supportsProxyLaunch)
  const selectedBrowser = managementBrowsers.find((browser) => browser.id === selectedBrowserId) || managementBrowsers[0]
  const defaultBrowserAvailable = activeChoices.some((browser) => browser.id === draft.defaultBrowserId)
  const proxyBrowserAvailable = proxyChoices.some((browser) => browser.id === draft.proxyBrowserId)
  const newBrowserNameRef = useRef(null)
  const pendingBrowserFocusRef = useRef('')
  const addBrowserButtonRef = useRef(null)
  const browserRowRefs = useRef(new Map())
  const pendingBrowserRemovalFocusRef = useRef('')

  useEffect(() => {
    if (!pendingBrowserFocusRef.current || selectedBrowser?.id !== pendingBrowserFocusRef.current) return
    newBrowserNameRef.current?.focus()
    pendingBrowserFocusRef.current = ''
  }, [selectedBrowser?.id])

  useEffect(() => {
    const focusTarget = pendingBrowserRemovalFocusRef.current
    if (!focusTarget) return
    if (focusTarget === 'add') addBrowserButtonRef.current?.focus()
    else browserRowRefs.current.get(focusTarget)?.focus()
    pendingBrowserRemovalFocusRef.current = ''
  }, [managementBrowsers.length, selectedBrowser?.id])

  function addCustomBrowser() {
    const id = `custom-browser-${Date.now().toString(36)}-${draft.customBrowsers.length + 1}`
    pendingBrowserFocusRef.current = id
    onChange({
      ...draft,
      customBrowsers: draft.customBrowsers.concat({
        id,
        displayName: '',
        command: '',
        icon: '',
      }),
    })
    onSelectBrowser(id)
  }

  function removeSelectedCustomBrowser() {
    const selectedIndex = managementBrowsers.findIndex((browser) => browser.id === selectedBrowser.id)
    const remaining = managementBrowsers.filter((browser) => browser.id !== selectedBrowser.id)
    const adjacentBrowser = remaining[Math.min(selectedIndex, remaining.length - 1)]
    pendingBrowserRemovalFocusRef.current = adjacentBrowser?.id || 'add'
    onChange(removeCustomBrowser(draft, selectedBrowser.id, catalog))
    onSelectBrowser(adjacentBrowser?.id || '')
  }

  function updateCustomBrowser(update) {
    onChange({
      ...draft,
      customBrowsers: draft.customBrowsers.map((browser) => (
        browser.id === selectedBrowser.id
          ? {
              ...browser,
              ...update,
              ...(update.command !== undefined ? { launchReference: '', engine: '' } : {}),
            }
          : browser
      )),
    })
  }

  const selectedEnabled = selectedBrowser
    ? !draft.disabledBrowserIds.includes(selectedBrowser.id)
    : false

  return (
    <div className="settings-page">
      <PageHeading
        eyebrow="Destinations"
        title="Browsers"
        description={customBrowserCommandsSupported
          ? 'Choose which installed browsers SSH Man can offer, or add a command-based browser.'
          : 'Choose which installed browsers SSH Man can offer.'}
        action={customBrowserCommandsSupported
          ? <button ref={addBrowserButtonRef} className="secondary-button" type="button" onClick={addCustomBrowser}><Plus aria-hidden="true" /> Add custom browser</button>
          : null}
      />

      <div className="settings-master-detail">
        <section className="settings-master-list" aria-label="Browser catalog">
          <div className="settings-master-list__heading">
            <strong>Available browsers</strong>
            <span>{activeChoices.length} enabled</span>
          </div>
          {managementBrowsers.length ? (
            <ul>
              {managementBrowsers.map((browser) => {
                const enabled = !draft.disabledBrowserIds.includes(browser.id)
                const selected = browser.id === selectedBrowser?.id
                return (
                  <li key={browser.id} className={selected ? 'is-selected' : ''}>
                    <button
                      ref={(element) => {
                        if (element) browserRowRefs.current.set(browser.id, element)
                        else browserRowRefs.current.delete(browser.id)
                      }}
                      type="button"
                      aria-label={`Edit ${browser.displayName || 'custom browser'}`}
                      aria-current={selected ? 'true' : undefined}
                      onClick={() => onSelectBrowser(browser.id)}
                    >
                      <span className="browser-catalog-icon" aria-hidden="true">
                        <BrowserAppearanceMark
                          target={{ kind: 'regular' }}
                          appearance={{ icon: browser.icon || '', primaryColor: '' }}
                          fallbackIcon={browser.supportsProxyLaunch ? Network : Globe2}
                        />
                      </span>
                      <span>
                        <strong>{browser.displayName || 'New custom browser'}</strong>
                        <small>{browser.custom ? 'Custom command' : browser.supportsProxyLaunch ? 'Installed · SOCKS ready' : 'Installed'}</small>
                      </span>
                    </button>
                    <label className="browser-enabled-toggle">
                      <span className="sr-only">{enabled ? 'Enabled' : 'Disabled'}</span>
                      <input
                        type="checkbox"
                        aria-label={`Enable ${browser.displayName || 'custom browser'}`}
                        checked={enabled}
                        onChange={(event) => onChange(setBrowserEnabled(draft, browser.id, event.target.checked, catalog))}
                      />
                    </label>
                  </li>
                )
              })}
            </ul>
          ) : (
            <EmptyState
              icon={Globe2}
              title="No browsers found"
              description={customBrowserCommandsSupported
                ? 'Add a custom browser to create a destination.'
                : 'Install a browser on this computer, then reopen Settings to detect it.'}
            />
          )}
        </section>

        <section className="section-block browser-detail" aria-label="Browser details">
          {selectedBrowser ? (
            <>
              <div className="browser-detail__heading">
                <div className="browser-detail__identity">
                  <span className="browser-catalog-icon" aria-hidden="true">
                    <BrowserAppearanceMark
                      target={{ kind: 'regular' }}
                      appearance={{ icon: selectedBrowser.icon || '', primaryColor: '' }}
                      fallbackIcon={selectedBrowser.supportsProxyLaunch ? Network : Globe2}
                    />
                  </span>
                  <div>
                    <h3>{selectedBrowser.displayName || 'New custom browser'}</h3>
                    <span>{selectedBrowser.custom ? 'Custom browser' : 'Installed browser'}</span>
                  </div>
                </div>
                <label className="browser-detail__toggle">
                  <input
                    type="checkbox"
                    aria-label={`Show ${selectedBrowser.displayName || 'custom browser'} in SSH Man`}
                    checked={selectedEnabled}
                    onChange={(event) => onChange(setBrowserEnabled(draft, selectedBrowser.id, event.target.checked, catalog))}
                  />
                  <span>Show in SSH Man</span>
                </label>
              </div>

              {selectedBrowser.custom ? (
                <div className="settings-form-grid">
                  <label>
                    <span>Name</span>
                    <input
                      ref={newBrowserNameRef}
                      aria-label="Custom browser name"
                      value={selectedBrowser.displayName}
                      placeholder="Work browser"
                      aria-invalid={Boolean(errors[`browser:${selectedBrowser.id}:name`])}
                      aria-describedby={errors[`browser:${selectedBrowser.id}:name`] ? `custom-browser-${selectedBrowser.id}-name-error` : undefined}
                      onChange={(event) => updateCustomBrowser({ displayName: event.target.value })}
                    />
                    {errors[`browser:${selectedBrowser.id}:name`]
                      ? <small id={`custom-browser-${selectedBrowser.id}-name-error`} className="field-error">{errors[`browser:${selectedBrowser.id}:name`]}</small>
                      : null}
                  </label>
                  <label>
                    <span>Command template</span>
                    <textarea
                      aria-label="Custom browser command"
                      value={selectedBrowser.command || ''}
                      placeholder={'open -a "Browser" "<URL>"'}
                      readOnly={!customBrowserCommandsSupported}
                      aria-invalid={Boolean(errors[`browser:${selectedBrowser.id}:command`])}
                      aria-describedby={errors[`browser:${selectedBrowser.id}:command`] ? `custom-browser-${selectedBrowser.id}-command-error` : `custom-browser-${selectedBrowser.id}-command-help`}
                      onChange={(event) => updateCustomBrowser({ command: event.target.value })}
                    />
                    <small id={`custom-browser-${selectedBrowser.id}-command-help`}>
                      {customBrowserCommandsSupported
                        ? <>Use a macOS <code>open</code> command and place <code>&lt;URL&gt;</code> where the link belongs.</>
                        : selectedBrowser.command
                          ? 'This saved command remains unchanged. Use SSH Man on macOS to edit it.'
                          : 'This browser keeps its original application setup. Use SSH Man on macOS to add a command.'}
                    </small>
                    {selectedBrowser.launchReference && !selectedBrowser.command
                      ? <small>This saved browser uses its original application setup. Add a command to update it.</small>
                      : null}
                    {errors[`browser:${selectedBrowser.id}:command`]
                      ? <small id={`custom-browser-${selectedBrowser.id}-command-error`} className="field-error">{errors[`browser:${selectedBrowser.id}:command`]}</small>
                      : null}
                  </label>
                  <fieldset className="browser-icon-picker">
                    <legend>Icon</legend>
                    {BROWSER_APPEARANCE_ICON_OPTIONS.map(({ value, label, Icon }) => (
                      <label key={value || 'default'} title={label}>
                        <input
                          type="radio"
                          name={`custom-browser-icon-${selectedBrowser.id}`}
                          aria-label={`${label} icon`}
                          checked={(selectedBrowser.icon || '') === value}
                          onChange={() => updateCustomBrowser({ icon: value })}
                        />
                        <span aria-hidden="true">{Icon ? <Icon /> : <Globe2 />}</span>
                      </label>
                    ))}
                  </fieldset>
                  <button
                    className="secondary-button secondary-button--danger"
                    type="button"
                    onClick={removeSelectedCustomBrowser}
                  >
                    <Trash2 aria-hidden="true" /> Delete custom browser
                  </button>
                </div>
              ) : (
                <dl className="browser-facts">
                  <div><dt>Availability</dt><dd>{selectedEnabled ? 'Shown across SSH Man' : 'Shown only in this catalog'}</dd></div>
                  <div><dt>Proxy launch</dt><dd>{selectedBrowser.supportsProxyLaunch ? 'Available' : 'Regular links'}</dd></div>
                </dl>
              )}
            </>
          ) : null}
        </section>
      </div>

      <section className="section-block browser-defaults" aria-labelledby="browser-defaults-heading">
        <div className="section-heading"><div><h3 id="browser-defaults-heading">Defaults</h3></div></div>
        <div className="settings-form-columns">
          <label>
            <span>Fallback browser</span>
            <select
              aria-label="Fallback browser"
              value={draft.defaultBrowserId}
              disabled={!activeChoices.length}
              onChange={(event) => onChange({ ...draft, defaultBrowserId: event.target.value })}
            >
              {!defaultBrowserAvailable && draft.defaultBrowserId ? <option value={draft.defaultBrowserId}>Unavailable browser</option> : null}
              {!draft.defaultBrowserId && activeChoices.length ? <option value="">Choose a browser</option> : null}
              {!activeChoices.length ? <option value="">No browser selected</option> : null}
              {activeChoices.map((browser) => <option key={browser.id} value={browser.id}>{browser.displayName}</option>)}
            </select>
          </label>
          <label>
            <span>SOCKS proxy browser</span>
            <select
              aria-label="SOCKS proxy browser"
              value={draft.proxyBrowserId}
              disabled={!proxyChoices.length}
              onChange={(event) => onChange({ ...draft, proxyBrowserId: event.target.value })}
            >
              {!proxyBrowserAvailable && draft.proxyBrowserId ? <option value={draft.proxyBrowserId}>Unavailable browser</option> : null}
              {!draft.proxyBrowserId && proxyChoices.length ? <option value="">Choose a proxy browser</option> : null}
              {!proxyChoices.length ? <option value="">No proxy-capable browser</option> : null}
              {proxyChoices.map((browser) => <option key={browser.id} value={browser.id}>{browser.displayName}</option>)}
            </select>
          </label>
        </div>
      </section>

      <SaveBar
        saving={saving}
        disabled={loading}
        hasErrors={hasErrors}
        message={saveMessage}
        onSave={onSave}
        onRetryValidation={onRetryValidation}
      />
    </div>
  )
}

function RoutingSettings({
  draft,
  catalog,
  servers,
  errors,
  selectedRuleId,
  onSelectRule,
  onChange,
  onSave,
  saving,
  loading,
  defaultBrowser,
  commandTemplatesSupported,
  onSetDefault,
  hasErrors,
  saveMessage,
  onRetryValidation,
}) {
  const browserChoices = settingsBrowserChoices(catalog, draft.customBrowsers, draft.disabledBrowserIds)
  const proxyChoices = browserChoices.filter((browser) => browser.supportsProxyLaunch)
  const selectedRuleIndex = draft.urlRules.findIndex((rule) => rule.id === selectedRuleId)
  const selectedRule = draft.urlRules[selectedRuleIndex] || draft.urlRules[0]
  const selectedRuleBrowserAvailable = selectedRule
    ? browserChoices.some((browser) => browser.id === selectedRule.browserId)
    : true
  const newRulePatternRef = useRef(null)
  const pendingRuleFocusRef = useRef('')
  const addRuleButtonRef = useRef(null)
  const ruleRowRefs = useRef(new Map())
  const pendingRuleRemovalFocusRef = useRef('')
  const addPortAssignmentButtonRef = useRef(null)
  const portInputRefs = useRef(new Map())
  const pendingPortRemovalFocusRef = useRef('')

  useEffect(() => {
    if (!pendingRuleFocusRef.current || selectedRule?.id !== pendingRuleFocusRef.current) return
    newRulePatternRef.current?.focus()
    pendingRuleFocusRef.current = ''
  }, [selectedRule?.id])

  useEffect(() => {
    const focusTarget = pendingRuleRemovalFocusRef.current
    if (!focusTarget) return
    if (focusTarget === 'add') addRuleButtonRef.current?.focus()
    else ruleRowRefs.current.get(focusTarget)?.focus()
    pendingRuleRemovalFocusRef.current = ''
  }, [draft.urlRules.length, selectedRule?.id])

  useEffect(() => {
    const focusTarget = pendingPortRemovalFocusRef.current
    if (!focusTarget) return
    if (focusTarget === 'add') {
      const addPortButton = addPortAssignmentButtonRef.current
      if (addPortButton && !addPortButton.disabled) addPortButton.focus()
      else addRuleButtonRef.current?.focus()
    } else {
      portInputRefs.current.get(focusTarget)?.focus()
    }
    pendingPortRemovalFocusRef.current = ''
  }, [draft.urlPortAssignments])

  function addRule() {
    const id = `rule-${Date.now().toString(36)}-${draft.urlRules.length + 1}`
    pendingRuleFocusRef.current = id
    onChange({
      ...draft,
      urlRules: draft.urlRules.concat({
        id,
        matchMode: 'contains',
        pattern: '',
        action: 'browser',
        browserId: draft.defaultBrowserId || browserChoices[0]?.id || '',
        command: '',
        openDirect: false,
      }),
    })
    onSelectRule(id)
  }

  function updateRule(update) {
    onChange({
      ...draft,
      urlRules: draft.urlRules.map((rule) => rule.id === selectedRule.id ? { ...rule, ...update } : rule),
    })
  }

  function updateRuleAction(action) {
    if (action === 'command') {
      updateRule({ action, browserId: '' })
      return
    }
    updateRule({
      action,
      command: '',
      browserId: selectedRule.browserId || draft.defaultBrowserId || browserChoices[0]?.id || '',
    })
  }

  function moveRule(offset) {
    const target = selectedRuleIndex + offset
    if (selectedRuleIndex < 0 || target < 0 || target >= draft.urlRules.length) return
    const rules = [...draft.urlRules]
    ;[rules[selectedRuleIndex], rules[target]] = [rules[target], rules[selectedRuleIndex]]
    onChange({ ...draft, urlRules: rules })
  }

  function removeRule() {
    const remaining = draft.urlRules.filter((rule) => rule.id !== selectedRule.id)
    const adjacentRule = remaining[Math.min(selectedRuleIndex, remaining.length - 1)]
    pendingRuleRemovalFocusRef.current = adjacentRule?.id || 'add'
    onChange({ ...draft, urlRules: remaining })
    onSelectRule(adjacentRule?.id || '')
  }

  function addPortAssignment() {
    onChange({
      ...draft,
      urlPortAssignments: draft.urlPortAssignments.concat({
        id: `port-${Date.now().toString(36)}-${draft.urlPortAssignments.length + 1}`,
        port: '',
        serverId: servers[0]?.server?.id || '',
        browserId: draft.proxyBrowserId || proxyChoices[0]?.id || '',
      }),
    })
  }

  function updatePortAssignment(id, update) {
    onChange({
      ...draft,
      urlPortAssignments: draft.urlPortAssignments.map((assignment) => (
        assignment.id === id ? { ...assignment, ...update } : assignment
      )),
    })
  }

  function removePortAssignment(id) {
    const selectedIndex = draft.urlPortAssignments.findIndex((assignment) => assignment.id === id)
    const remaining = draft.urlPortAssignments.filter((assignment) => assignment.id !== id)
    const adjacentAssignment = remaining[Math.min(selectedIndex, remaining.length - 1)]
    pendingPortRemovalFocusRef.current = adjacentAssignment?.id || 'add'
    onChange({ ...draft, urlPortAssignments: remaining })
  }

  return (
    <div className="settings-page">
      <PageHeading
        eyebrow="Links"
        title="URL routing"
        description="Rules run from top to bottom. Choose a simple text match or a regular expression."
        action={<button ref={addRuleButtonRef} className="secondary-button" type="button" onClick={addRule}><Plus aria-hidden="true" /> Add rule</button>}
      />

      <div className="default-browser-control">
        <Route aria-hidden="true" />
        <div>
          <strong>
            {!defaultBrowser.supported
              ? 'Default browser integration is unavailable'
              : defaultBrowser.isDefault
                ? 'SSH Man is your default browser.'
                : 'Make SSH Man your default browser'}
          </strong>
          <span>
            {defaultBrowser.supported
              ? 'Route links through your browser rules and connected servers.'
              : 'You can still launch any detected browser through an SSH tunnel.'}
          </span>
        </div>
        <button
          className="secondary-button"
          type="button"
          disabled={!defaultBrowser.supported || defaultBrowser.isDefault || saving || loading || Object.keys(errors).length > 0}
          onClick={onSetDefault}
        >
          {defaultBrowser.isDefault
            ? <><Check aria-hidden="true" /> Default</>
            : !defaultBrowser.supported
              ? 'Unavailable'
              : 'Make SSH Man default'}
        </button>
      </div>

      <div className={`settings-master-detail settings-master-detail--rules${draft.urlRules.length ? '' : ' is-empty'}`}>
        <section className="settings-master-list" aria-label="Ordered URL rules">
          <div className="settings-master-list__heading">
            <strong>Rules</strong>
            <span>First match wins</span>
          </div>
          {draft.urlRules.length ? (
            <ol>
              {draft.urlRules.map((rule, index) => (
                <li key={rule.id} className={rule.id === selectedRule?.id ? 'is-selected' : ''}>
                  <button
                    ref={(element) => {
                      if (element) ruleRowRefs.current.set(rule.id, element)
                      else ruleRowRefs.current.delete(rule.id)
                    }}
                    type="button"
                    aria-label={`Edit rule ${index + 1}: ${rule.pattern || 'New rule'}; ${matchModeLabels[rule.matchMode] || 'Regex'}; ${rule.openDirect ? 'Open directly' : 'Show chooser'}`}
                    aria-current={rule.id === selectedRule?.id ? 'true' : undefined}
                    onClick={() => onSelectRule(rule.id)}
                  >
                    <span className="rule-order" aria-hidden="true">{index + 1}</span>
                    <span>
                      <strong>{rule.pattern || 'New rule'}</strong>
                      <small>{matchModeLabels[rule.matchMode] || 'Regex'} · {rule.openDirect ? 'Open directly' : 'Show chooser'}</small>
                    </span>
                  </button>
                </li>
              ))}
            </ol>
          ) : (
            <EmptyState icon={Route} title="No URL rules" description="Add a rule to preselect or directly open a destination." />
          )}
        </section>

        {draft.urlRules.length ? (
          <section className="section-block rule-detail" aria-label="Rule details">
            {selectedRule ? (
              <>
              <div className="rule-detail__heading">
                <div>
                  <span className="eyebrow">Rule {selectedRuleIndex + 1}</span>
                  <h3>{selectedRule.pattern || 'New rule'}</h3>
                </div>
                <div>
                  <IconButton label={`Move rule ${selectedRuleIndex + 1} up`} disabled={selectedRuleIndex === 0} onClick={() => moveRule(-1)}>
                    <ArrowUp aria-hidden="true" />
                  </IconButton>
                  <IconButton label={`Move rule ${selectedRuleIndex + 1} down`} disabled={selectedRuleIndex === draft.urlRules.length - 1} onClick={() => moveRule(1)}>
                    <ArrowDown aria-hidden="true" />
                  </IconButton>
                  <IconButton label={`Remove rule ${selectedRuleIndex + 1}`} onClick={removeRule}>
                    <Trash2 aria-hidden="true" />
                  </IconButton>
                </div>
              </div>

              <fieldset className="match-mode-control">
                <legend>Match mode</legend>
                <div className="segmented-control">
                  {matchModeOptions.map((option) => (
                    <button
                      key={option.value}
                      type="button"
                      aria-pressed={selectedRule.matchMode === option.value}
                      className={selectedRule.matchMode === option.value ? 'is-selected' : ''}
                      onClick={() => updateRule({ matchMode: option.value })}
                    >
                      {option.label}
                    </button>
                  ))}
                </div>
              </fieldset>

              <div className="settings-form-grid">
                <label>
                  <span>Match value</span>
                  <input
                    ref={newRulePatternRef}
                    aria-label={`Rule ${selectedRuleIndex + 1} match value`}
                    value={selectedRule.pattern}
                    placeholder={matchModePlaceholders[selectedRule.matchMode] || matchModePlaceholders.contains}
                    aria-invalid={Boolean(errors[`rule:${selectedRule.id}:pattern`])}
                    aria-describedby={[
                      selectedRule.matchMode === 'regex' ? '' : `rule-${selectedRule.id}-pattern-help`,
                      errors[`rule:${selectedRule.id}:pattern`] ? `rule-${selectedRule.id}-pattern-error` : '',
                    ].filter(Boolean).join(' ') || undefined}
                    onChange={(event) => updateRule({ pattern: event.target.value })}
                  />
                  {selectedRule.matchMode !== 'regex'
                    ? <small id={`rule-${selectedRule.id}-pattern-help`}>SSH Man compares this value with the complete link.</small>
                    : null}
                  {errors[`rule:${selectedRule.id}:pattern`]
                    ? <small id={`rule-${selectedRule.id}-pattern-error`} className="field-error">{errors[`rule:${selectedRule.id}:pattern`]}</small>
                    : null}
                </label>
                <label>
                  <span>Action</span>
                  <select
                    aria-label={`Rule ${selectedRuleIndex + 1} action`}
                    value={selectedRule.action}
                    onChange={(event) => updateRuleAction(event.target.value)}
                  >
                    <option value="browser">Open in browser</option>
                    {commandTemplatesSupported
                      ? <option value="command">Run command</option>
                      : selectedRule.action === 'command'
                        ? <option value="command" disabled>Run command (saved)</option>
                        : null}
                  </select>
                </label>
                {selectedRule.action === 'command' ? (
                  <label>
                    <span>Command template</span>
                    <textarea
                      aria-label={`Rule ${selectedRuleIndex + 1} command`}
                      value={selectedRule.command}
                      placeholder={'open -a "Browser" "<URL>"'}
                      readOnly={!commandTemplatesSupported}
                      aria-invalid={Boolean(errors[`rule:${selectedRule.id}:command`])}
                      aria-describedby={[
                        `rule-${selectedRule.id}-command-help`,
                        errors[`rule:${selectedRule.id}:command`] ? `rule-${selectedRule.id}-command-error` : '',
                      ].filter(Boolean).join(' ')}
                      onChange={(event) => updateRule({ command: event.target.value })}
                    />
                    <small id={`rule-${selectedRule.id}-command-help`}>
                      {commandTemplatesSupported
                        ? <>Use a macOS <code>open</code> command and place <code>&lt;URL&gt;</code> where the link belongs.</>
                        : 'This saved command remains unchanged. Switch the action to Open in browser, or use SSH Man on macOS to edit it.'}
                    </small>
                    {errors[`rule:${selectedRule.id}:command`]
                      ? <small id={`rule-${selectedRule.id}-command-error`} className="field-error">{errors[`rule:${selectedRule.id}:command`]}</small>
                      : null}
                  </label>
                ) : (
                  <label>
                    <span>Browser</span>
                    <select
                      aria-label={`Rule ${selectedRuleIndex + 1} browser`}
                      value={selectedRule.browserId}
                      aria-invalid={Boolean(errors[`rule:${selectedRule.id}:browser`])}
                      aria-describedby={errors[`rule:${selectedRule.id}:browser`] ? `rule-${selectedRule.id}-browser-error` : undefined}
                      onChange={(event) => updateRule({ browserId: event.target.value })}
                    >
                      {!selectedRuleBrowserAvailable && selectedRule.browserId ? <option value={selectedRule.browserId}>Unavailable browser</option> : null}
                      {!selectedRule.browserId ? <option value="">Choose a browser</option> : null}
                      {browserChoices.map((browser) => <option key={browser.id} value={browser.id}>{browser.displayName}</option>)}
                    </select>
                    {errors[`rule:${selectedRule.id}:browser`]
                      ? <small id={`rule-${selectedRule.id}-browser-error`} className="field-error">{errors[`rule:${selectedRule.id}:browser`]}</small>
                      : null}
                  </label>
                )}
                <label className="direct-open-option">
                  <input
                    type="checkbox"
                    aria-label={`Skip selection for rule ${selectedRuleIndex + 1}`}
                    checked={selectedRule.openDirect}
                    onChange={(event) => updateRule({ openDirect: event.target.checked })}
                  />
                  <span>
                    <strong>Open directly</strong>
                    <small>Open immediately when available. Otherwise, SSH Man shows the chooser.</small>
                  </span>
                </label>
              </div>
              </>
            ) : (
              <EmptyState icon={Route} title="Select a rule" description="Choose a rule from the list or add a new one." />
            )}
          </section>
        ) : null}
      </div>

      <section className="section-block" aria-labelledby="port-defaults-heading">
        <div className="section-heading">
          <div>
            <h3 id="port-defaults-heading">Port defaults</h3>
            <p className="section-description">Preselect a connected host and proxy-capable browser for a URL port.</p>
          </div>
          <button ref={addPortAssignmentButtonRef} className="text-button" type="button" disabled={!servers.length || !proxyChoices.length} onClick={addPortAssignment}>
            <Plus aria-hidden="true" /> Assign port
          </button>
        </div>
        {draft.urlPortAssignments.length ? (
          <ol className="url-port-assignment-list">
            {draft.urlPortAssignments.map((assignment, index) => {
              const portError = errors[`port:${assignment.id}:port`]
              const serverError = errors[`port:${assignment.id}:server`]
              const browserError = errors[`port:${assignment.id}:browser`]
              const portErrorId = `port-${assignment.id}-port-error`
              const serverErrorId = `port-${assignment.id}-server-error`
              const browserErrorId = `port-${assignment.id}-browser-error`
              const serverAvailable = servers.some((item) => item.server.id === assignment.serverId)
              return (
                <li className="url-port-assignment-card" key={assignment.id}>
                  <label>
                    <span>Port</span>
                    <input
                      ref={(element) => {
                        if (element) portInputRefs.current.set(assignment.id, element)
                        else portInputRefs.current.delete(assignment.id)
                      }}
                      type="number"
                      inputMode="numeric"
                      min="1"
                      max="65535"
                      aria-label={`Port assignment ${index + 1} port`}
                      value={assignment.port}
                      aria-invalid={Boolean(portError)}
                      aria-describedby={portError ? portErrorId : undefined}
                      onChange={(event) => updatePortAssignment(assignment.id, { port: event.target.value })}
                    />
                    {portError ? <small id={portErrorId} className="field-error">{portError}</small> : null}
                  </label>
                  <label>
                    <span>Host</span>
                    <select
                      aria-label={`Port assignment ${index + 1} host`}
                      value={assignment.serverId}
                      aria-invalid={Boolean(serverError)}
                      aria-describedby={serverError ? serverErrorId : undefined}
                      onChange={(event) => updatePortAssignment(assignment.id, { serverId: event.target.value })}
                    >
                      {!serverAvailable && assignment.serverId ? <option value={assignment.serverId}>Unavailable host</option> : null}
                      {!assignment.serverId ? <option value="">Choose a host</option> : null}
                      {servers.map((item) => <option key={item.server.id} value={item.server.id}>{item.server.name}</option>)}
                    </select>
                    {serverError ? <small id={serverErrorId} className="field-error">{serverError}</small> : null}
                  </label>
                  <label>
                    <span>Browser</span>
                    <select
                      aria-label={`Port assignment ${index + 1} browser`}
                      value={assignment.browserId}
                      aria-invalid={Boolean(browserError)}
                      aria-describedby={browserError ? browserErrorId : undefined}
                      onChange={(event) => updatePortAssignment(assignment.id, { browserId: event.target.value })}
                    >
                      {!proxyChoices.some((browser) => browser.id === assignment.browserId) && assignment.browserId
                        ? <option value={assignment.browserId}>Unavailable browser</option>
                        : null}
                      {!assignment.browserId ? <option value="">Choose a browser</option> : null}
                      {proxyChoices.map((browser) => <option key={browser.id} value={browser.id}>{browser.displayName}</option>)}
                    </select>
                    {browserError ? <small id={browserErrorId} className="field-error">{browserError}</small> : null}
                  </label>
                  <IconButton
                    label={`Remove port assignment ${index + 1}`}
                    onClick={() => removePortAssignment(assignment.id)}
                  >
                    <Trash2 aria-hidden="true" />
                  </IconButton>
                </li>
              )
            })}
          </ol>
        ) : <p className="url-rules-empty">No port defaults yet.</p>}
      </section>

      <SaveBar
        saving={saving}
        disabled={loading}
        hasErrors={hasErrors}
        message={saveMessage}
        onSave={onSave}
        onRetryValidation={onRetryValidation}
      />
    </div>
  )
}

function HealthSettings({
  diagnostics,
  storageIssue,
  runtimeFresh,
  onReload,
  onRefreshRuntime,
  onCopyPath,
  onOpenDevTools,
  onQuit,
  standalone,
}) {
  return (
    <div className="settings-page">
      <PageHeading eyebrow="Diagnostics" title="App health" description="Review local paths, live status, and troubleshooting tools." />
      <section className="section-block" aria-labelledby="health-heading">
        <div className="section-heading">
          <div><h3 id="health-heading">Status</h3></div>
          <span className={storageIssue ? 'health-badge is-warning' : 'health-badge is-healthy'}>
            {storageIssue ? 'Attention' : 'Healthy'}
          </span>
        </div>

        {storageIssue ? (
          <div className="inline-alert inline-alert--warning" role="alert">
            <WifiOff aria-hidden="true" />
            <div><strong>Saved data needs attention</strong><span>{storageIssue}</span></div>
          </div>
        ) : null}

        <div className="status-line">
          {runtimeFresh ? <Wifi aria-hidden="true" /> : <WifiOff aria-hidden="true" />}
          <div><strong>Runtime status</strong><span>{runtimeFresh ? 'Live session status is current.' : 'Refresh to check the latest session status.'}</span></div>
          <IconButton label="Refresh runtime status" onClick={() => onRefreshRuntime({ quiet: false })}>
            <RefreshCw aria-hidden="true" />
          </IconButton>
        </div>
        <div className="status-line">
          <BadgeInfo aria-hidden="true" />
          <div><strong>App version</strong><span>{diagnostics.version}</span></div>
        </div>
        <PathCard icon={FolderOpen} label="App data path" value={diagnostics.appDataPath} onCopy={onCopyPath} />
        <PathCard icon={Database} label="Database path" value={diagnostics.databasePath} onCopy={onCopyPath} />
        <div className="settings-actions">
          <button className="secondary-button" type="button" onClick={() => onReload({ quiet: true })}>
            <RefreshCw aria-hidden="true" /> Reload app data
          </button>
          <button className="secondary-button" type="button" onClick={onOpenDevTools}>
            <SquareTerminal aria-hidden="true" /> Devtools
          </button>
        </div>
      </section>
      <section className="quit-section">
        <button className="secondary-button secondary-button--danger secondary-button--full" type="button" onClick={onQuit}>
          {standalone ? <X aria-hidden="true" /> : <Power aria-hidden="true" />}
          {standalone ? 'Close Settings' : 'Quit SSH Man'}
        </button>
        <p>{standalone ? 'Closing Settings keeps SSH Man and active tunnels running.' : 'Quit stops tunnels and closes SSH Man windows.'}</p>
      </section>
    </div>
  )
}

export function SettingsScreen({
  preferences,
  servers = [],
  diagnostics,
  storageIssue,
  runtimeFresh,
  onToggleTheme,
  onSetBrowserSwitcherShortcut,
  onSetBrowserSwitcherBackwardShortcut,
  onOpenBrowserSwitcher,
  urlRoutingState,
  onLoadURLRouting,
  onSaveURLRouting,
  onValidateURLRulePattern,
  onSetDefaultBrowser,
  onReload,
  onRefreshRuntime,
  onCopyPath,
  onOpenDevTools,
  onQuit,
  onRegisterCloseRequestHandler,
  onAllowClose,
  standalone = false,
}) {
  const catalog = urlRoutingState.browserCatalog || []
  const [activePage, setActivePage] = useState('general')
  const [draft, setDraft] = useState(() => settingsDraft(preferences, catalog))
  const [persistedDraft, setPersistedDraft] = useState(() => settingsDraft(preferences, catalog))
  const [saving, setSaving] = useState(false)
  const [selectedBrowserId, setSelectedBrowserId] = useState('')
  const [selectedRuleId, setSelectedRuleId] = useState('')
  const [closePromptOpen, setClosePromptOpen] = useState(false)
  const [closeSaveError, setCloseSaveError] = useState('')
  const [regexValidation, setRegexValidation] = useState({
    signature: '',
    pending: false,
    errors: {},
    transportError: '',
  })
  const [regexValidationAttempt, setRegexValidationAttempt] = useState(0)
  const routingSignatureRef = useRef('')
  const closeButtonRef = useRef(null)
  const closePromptOpenerRef = useRef(null)

  useEffect(() => {
    void onLoadURLRouting()
  }, [onLoadURLRouting])

  const routingSignature = JSON.stringify({
    defaultBrowserId: preferences.defaultBrowserId || '',
    proxyBrowserId: preferences.proxyBrowserId || '',
    disabledBrowserIds: preferences.disabledBrowserIds || [],
    customBrowsers: preferences.customBrowsers || [],
    urlRules: preferences.urlRules || [],
    urlPortAssignments: preferences.urlPortAssignments || [],
    catalog,
  })

  useEffect(() => {
    if (saving) return
    if (routingSignatureRef.current === routingSignature) return
    routingSignatureRef.current = routingSignature
    const next = settingsDraft(preferences, catalog)
    setSelectedBrowserId((current) => {
      const browserIds = new Set([...catalog, ...next.customBrowsers].map((browser) => browser.id))
      return browserIds.has(current) ? current : (catalog[0]?.id || next.customBrowsers[0]?.id || '')
    })
    setSelectedRuleId((current) => (
      next.urlRules.some((rule) => rule.id === current) ? current : (next.urlRules[0]?.id || '')
    ))
    if (settingsDraftSignature(next) === settingsDraftSignature(persistedDraft)) return
    setDraft(next)
    setPersistedDraft(next)
  }, [catalog, persistedDraft, preferences, routingSignature, saving])

  const syncErrors = useMemo(() => settingsDraftErrors(draft, catalog), [catalog, draft])
  const regexRules = useMemo(() => (
    (draft.urlRules || []).filter((rule) => (
      rule.matchMode === 'regex' &&
      rule.pattern?.trim()
    ))
  ), [draft.urlRules])
  const currentRegexSignature = regexRulesSignature(regexRules)
  const regexValidationPending = Boolean(onValidateURLRulePattern) && regexRules.length > 0 && (
    regexValidation.signature !== currentRegexSignature || regexValidation.pending
  )
  const regexValidationTransportError = regexValidation.signature === currentRegexSignature
    ? regexValidation.transportError
    : ''
  const regexValidationBlocked = regexValidationPending || Boolean(regexValidationTransportError)
  const errors = useMemo(() => ({
    ...syncErrors,
    ...(regexValidation.signature === currentRegexSignature ? regexValidation.errors : {}),
  }), [currentRegexSignature, regexValidation, syncErrors])
  const errorCounts = useMemo(() => settingsErrorCounts(errors), [errors])
  const browserDirty = browserDraftSignature(draft) !== browserDraftSignature(persistedDraft)
  const routingDirty = routingDraftSignature(draft) !== routingDraftSignature(persistedDraft)
  const dirty = browserDirty || routingDirty
  const hasErrors = errorCounts.browsers + errorCounts.routing > 0
  const saveMessage = regexValidationPending
    ? 'Checking regular expressions…'
    : regexValidationTransportError || settingsSaveMessage(errorCounts, dirty)
  const pageStatus = {
    browsers: { errorCount: errorCounts.browsers, dirty: browserDirty },
    routing: { errorCount: errorCounts.routing, dirty: routingDirty },
  }

  useEffect(() => {
    if (!onValidateURLRulePattern || regexRules.length === 0) {
      setRegexValidation((current) => (
        current.signature === currentRegexSignature &&
        !current.pending &&
        Object.keys(current.errors).length === 0 &&
        !current.transportError
          ? current
          : { signature: currentRegexSignature, pending: false, errors: {}, transportError: '' }
      ))
      return undefined
    }

    let active = true
    setRegexValidation({
      signature: currentRegexSignature,
      pending: true,
      errors: {},
      transportError: '',
    })
    const timer = window.setTimeout(() => {
      void Promise.all(regexRules.map(async (rule) => {
        const result = await onValidateURLRulePattern(rule.matchMode, rule.pattern)
        const message = result?.valid === false
          ? result.message || 'Enter a valid regular expression.'
          : ''
        return [rule.id, message]
      }))
        .then((results) => {
          if (!active) return
          const nextErrors = Object.fromEntries(results
            .filter(([, message]) => Boolean(message))
            .map(([ruleId, message]) => [`rule:${ruleId}:pattern`, message]))
          setRegexValidation({
            signature: currentRegexSignature,
            pending: false,
            errors: nextErrors,
            transportError: '',
          })
        })
        .catch(() => {
          if (!active) return
          setRegexValidation({
            signature: currentRegexSignature,
            pending: false,
            errors: {},
            transportError: 'Regular expressions could not be checked. Retry validation.',
          })
        })
    }, 150)

    return () => {
      active = false
      window.clearTimeout(timer)
    }
  }, [currentRegexSignature, onValidateURLRulePattern, regexRules, regexValidationAttempt])

  useEffect(() => {
    if (!standalone || !onRegisterCloseRequestHandler) return undefined
    return onRegisterCloseRequestHandler(() => {
      if (dirty) {
        openClosePrompt()
        return
      }
      void closeSettings()
    })
  }, [dirty, onAllowClose, onQuit, onRegisterCloseRequestHandler, standalone])

  function updateDraft(next) {
    setDraft(reconcileBrowserReferences(next, catalog))
  }

  async function save() {
    if (saving || regexValidationBlocked || Object.keys(errors).length) return null
    const submittedDraftSignature = settingsDraftSignature(draft)
    setSaving(true)
    try {
      const saved = await onSaveURLRouting({
        ...draft,
        urlRules: draft.urlRules.map((rule) => ({
          ...normalizeURLRule(rule),
          browserId: rule.action === 'browser' ? rule.browserId : '',
          command: rule.action === 'command' ? rule.command : '',
        })),
        urlPortAssignments: draft.urlPortAssignments.map((assignment) => ({
          ...assignment,
          port: Number(assignment.port),
        })),
      })
      if (saved) {
        const next = settingsDraft(saved, catalog)
        setDraft((current) => (
          settingsDraftSignature(current) === submittedDraftSignature ? next : current
        ))
        setPersistedDraft(next)
      }
      return saved
    } finally {
      setSaving(false)
    }
  }

  async function setDefaultBrowser() {
    const saved = await save()
    if (saved) await onSetDefaultBrowser()
  }

  async function closeSettings() {
    await onAllowClose?.()
    await onQuit()
  }

  function openClosePrompt() {
    const activeElement = typeof document === 'undefined' ? null : document.activeElement
    closePromptOpenerRef.current = activeElement &&
      activeElement !== document.body &&
      typeof activeElement.focus === 'function'
      ? activeElement
      : closeButtonRef.current
    setCloseSaveError('')
    setClosePromptOpen(true)
  }

  function requestClose() {
    if (dirty) {
      openClosePrompt()
      return
    }
    void closeSettings()
  }

  function keepEditing() {
    const opener = closePromptOpenerRef.current
    setCloseSaveError('')
    setClosePromptOpen(false)
    requestAnimationFrame(() => {
      const focusTarget = opener?.isConnected && !opener.disabled
        ? opener
        : closeButtonRef.current
      focusTarget?.focus()
      closePromptOpenerRef.current = null
    })
  }

  async function saveAndClose() {
    setCloseSaveError('')
    const saved = await save()
    if (!saved) {
      setCloseSaveError('Settings could not be saved. Your changes are still here. Try again or keep editing.')
      return
    }
    await closeSettings()
  }

  return (
    <div className="settings-screen" aria-label="Settings">
      <aside className="settings-sidebar">
        <nav aria-label="Settings pages">
          {settingsPages.map(({ id, label, Icon }) => {
            const status = pageStatus[id]
            return (
              <button
                key={id}
                type="button"
                className={activePage === id ? 'is-active' : ''}
                aria-label={label}
                aria-current={activePage === id ? 'page' : undefined}
                aria-describedby={status?.errorCount || status?.dirty ? `settings-nav-${id}-status` : undefined}
                onClick={() => setActivePage(id)}
              >
                <Icon aria-hidden="true" />
                <span className="settings-nav-label">{label}</span>
                {status?.errorCount
                  ? (
                    <span id={`settings-nav-${id}-status`} className="settings-nav-badge is-error" title={`${status.errorCount} field${status.errorCount === 1 ? '' : 's'} need${status.errorCount === 1 ? 's' : ''} attention`}>
                      <span aria-hidden="true">{status.errorCount}</span>
                      <span className="sr-only">{`${status.errorCount} field${status.errorCount === 1 ? '' : 's'} need${status.errorCount === 1 ? 's' : ''} attention`}</span>
                    </span>
                    )
                  : status?.dirty
                    ? (
                      <span id={`settings-nav-${id}-status`} className="settings-nav-badge" title="Unsaved changes">
                        <span aria-hidden="true">•</span>
                        <span className="sr-only">Unsaved changes</span>
                      </span>
                      )
                    : null}
              </button>
            )
          })}
        </nav>
        <div className="settings-sidebar__footer">
          <span>{diagnostics.version}</span>
          <button ref={closeButtonRef} type="button" aria-label={standalone ? 'Close Settings' : 'Quit SSH Man'} onClick={requestClose}>
            {standalone ? <X aria-hidden="true" /> : <Power aria-hidden="true" />}
            <span>{standalone ? 'Close' : 'Quit'}</span>
          </button>
        </div>
      </aside>

      <div className="settings-content">
        {activePage === 'general' ? (
          <GeneralSettings
            preferences={preferences}
            onToggleTheme={onToggleTheme}
            onSetBrowserSwitcherShortcut={onSetBrowserSwitcherShortcut}
            onSetBrowserSwitcherBackwardShortcut={onSetBrowserSwitcherBackwardShortcut}
            onOpenBrowserSwitcher={onOpenBrowserSwitcher}
          />
        ) : null}
        {activePage === 'browsers' ? (
          <BrowserSettings
            draft={draft}
            catalog={catalog}
            errors={errors}
            selectedBrowserId={selectedBrowserId}
            customBrowserCommandsSupported={urlRoutingState.defaultBrowser.supported}
            onSelectBrowser={setSelectedBrowserId}
            onChange={updateDraft}
            onSave={save}
            saving={saving}
            loading={urlRoutingState.loading || regexValidationBlocked}
            hasErrors={hasErrors}
            saveMessage={saving ? 'Saving browser and routing settings…' : saveMessage}
            onRetryValidation={regexValidationTransportError
              ? () => setRegexValidationAttempt((current) => current + 1)
              : undefined}
          />
        ) : null}
        {activePage === 'routing' ? (
          <RoutingSettings
            draft={draft}
            catalog={catalog}
            servers={servers}
            errors={errors}
            selectedRuleId={selectedRuleId}
            onSelectRule={setSelectedRuleId}
            onChange={updateDraft}
            onSave={save}
            saving={saving}
            loading={urlRoutingState.loading || regexValidationBlocked}
            defaultBrowser={urlRoutingState.defaultBrowser}
            commandTemplatesSupported={urlRoutingState.defaultBrowser.supported}
            onSetDefault={setDefaultBrowser}
            hasErrors={hasErrors}
            saveMessage={saving ? 'Saving browser and routing settings…' : saveMessage}
            onRetryValidation={regexValidationTransportError
              ? () => setRegexValidationAttempt((current) => current + 1)
              : undefined}
          />
        ) : null}
        {activePage === 'health' ? (
          <HealthSettings
            diagnostics={diagnostics}
            storageIssue={storageIssue}
            runtimeFresh={runtimeFresh}
            onReload={onReload}
            onRefreshRuntime={onRefreshRuntime}
            onCopyPath={onCopyPath}
            onOpenDevTools={onOpenDevTools}
            onQuit={requestClose}
            standalone={standalone}
          />
        ) : null}
      </div>
      <UnsavedSettingsDialog
        open={closePromptOpen}
        canSave={!hasErrors && !regexValidationBlocked}
        checking={regexValidationPending && !hasErrors}
        validationUnavailable={Boolean(regexValidationTransportError) && !hasErrors}
        saveError={closeSaveError}
        pending={saving}
        onKeepEditing={keepEditing}
        onDiscard={() => {
          setCloseSaveError('')
          setClosePromptOpen(false)
          void closeSettings()
        }}
        onSave={() => {
          void saveAndClose()
        }}
      />
    </div>
  )
}
