import { useEffect, useState } from 'react'
import {
  ArrowRight,
  ArrowUp,
  ArrowDown,
  Check,
  Copy,
  Database,
  FolderOpen,
  LoaderCircle,
  Keyboard,
  Moon,
  Power,
  RefreshCw,
  Route,
  Server,
  Plus,
  Square,
  Sun,
  SquareTerminal,
  Trash2,
  Wifi,
  WifiOff,
  X,
} from 'lucide-react'
import { configurationEndpoint, findConfigurationRecord, shortcutFromKeyboardEvent } from '../model/appModel'
import { EmptyState, IconButton, StatusPill } from '../components/AppChrome'

export function ActivityScreen({ servers, sessions, pending, runtimeFresh, onOpen, onStop, onRefresh }) {
  const records = sessions.map((session) => ({ session, record: findConfigurationRecord(servers, session.configurationId) })).filter((item) => item.record)

  return (
    <div className="screen-scroll" aria-label="Active tunnels">
      <section className="screen-intro">
        <div>
          <span className="eyebrow">Live now</span>
          <h2>{records.length} active tunnel{records.length === 1 ? '' : 's'}</h2>
          <p>{runtimeFresh ? 'Connection status is up to date.' : 'Showing the last known connection state.'}</p>
        </div>
        <IconButton label="Refresh connection status" onClick={() => onRefresh({ quiet: false })}>
          <RefreshCw aria-hidden="true" />
        </IconButton>
      </section>

      {!runtimeFresh ? (
        <div className="inline-alert inline-alert--warning" role="status">
          <WifiOff aria-hidden="true" />
          <div><strong>Live status is unavailable</strong><span>Controls still work; refresh before trusting an old state.</span></div>
        </div>
      ) : null}

      {records.length ? (
        <ul className="row-list row-list--compact" aria-label="Active connection list">
          {records.map(({ session, record }) => (
            <li key={session.configurationId} className="activity-row">
              <button className="row-main" type="button" onClick={() => onOpen(session.configurationId)}>
                <span className="activity-row__icon" aria-hidden="true"><Wifi /></span>
                <span className="row-copy">
                  <span className="row-heading">
                    <strong>{record.configuration.label}</strong>
                    <StatusPill status={session.status} compact />
                  </span>
                  <span className="row-meta">{record.server.name}</span>
                  <span className="row-detail endpoint-text">{configurationEndpoint(record.configuration, session)}</span>
                </span>
                <ArrowRight aria-hidden="true" />
              </button>
              <IconButton
                label={`Stop ${record.configuration.label}`}
                className="tunnel-quick-action is-stop"
                disabled={Boolean(pending[`session:${session.configurationId}`])}
                onClick={() => onStop(session.configurationId)}
              >
                {pending[`session:${session.configurationId}`] ? <LoaderCircle className="spin" aria-hidden="true" /> : <Square aria-hidden="true" />}
              </IconButton>
            </li>
          ))}
        </ul>
      ) : (
        <EmptyState
          icon={Wifi}
          title="No active tunnels"
          description="Start a saved tunnel and it will appear here for quick access."
        />
      )}
    </div>
  )
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

function URLRoutingSettings({
  preferences,
  state,
  onLoad,
  onSave,
  onChooseBrowserApplication,
  onSetDefault,
}) {
  const [draft, setDraft] = useState({
    defaultBrowserId: preferences.defaultBrowserId || '',
    proxyBrowserId: preferences.proxyBrowserId || '',
    customBrowsers: preferences.customBrowsers || [],
    urlRules: preferences.urlRules || [],
  })
  const [saving, setSaving] = useState(false)
  const [settingDefault, setSettingDefault] = useState(false)

  useEffect(() => {
    void onLoad()
  }, [onLoad])

  useEffect(() => {
    const firstBrowser = state.browsers[0]?.id || ''
    const firstProxyBrowser = state.browsers.find((browser) => browser.supportsProxyLaunch)?.id || ''
    setDraft({
      defaultBrowserId: preferences.defaultBrowserId || firstBrowser,
      proxyBrowserId: preferences.proxyBrowserId || firstProxyBrowser,
      customBrowsers: preferences.customBrowsers || [],
      urlRules: preferences.urlRules || [],
    })
  }, [preferences.customBrowsers, preferences.defaultBrowserId, preferences.proxyBrowserId, preferences.urlRules, state.browsers])

  const browserChoices = [...state.browsers]
  for (const customBrowser of draft.customBrowsers) {
    if (!browserChoices.some((browser) => browser.id === customBrowser.id)) {
      browserChoices.push({
        id: customBrowser.id,
        displayName: customBrowser.displayName || 'Unnamed custom browser',
        engine: customBrowser.engine,
        supportsProxyLaunch: customBrowser.engine !== 'regular',
        custom: true,
      })
    }
  }

  function addCustomBrowser() {
    setDraft((current) => ({
      ...current,
      customBrowsers: current.customBrowsers.concat({
        id: `custom-browser-${Date.now().toString(36)}-${current.customBrowsers.length + 1}`,
        displayName: '',
        launchReference: '',
        engine: 'regular',
      }),
    }))
  }

  function updateCustomBrowser(index, update) {
    setDraft((current) => ({
      ...current,
      customBrowsers: current.customBrowsers.map((browser, browserIndex) => browserIndex === index ? { ...browser, ...update } : browser),
    }))
  }

  function removeCustomBrowser(index) {
    setDraft((current) => {
      const removedID = current.customBrowsers[index]?.id
      const customBrowsers = current.customBrowsers.filter((_, browserIndex) => browserIndex !== index)
      const remainingChoices = state.browsers
        .concat(customBrowsers.map((browser) => ({
          id: browser.id,
          supportsProxyLaunch: browser.engine !== 'regular',
        })))
        .filter((browser) => browser.id !== removedID)
      const fallbackID = remainingChoices[0]?.id || ''
      const proxyID = remainingChoices.find((browser) => browser.supportsProxyLaunch)?.id || ''
      return {
        ...current,
        customBrowsers,
        defaultBrowserId: current.defaultBrowserId === removedID ? fallbackID : current.defaultBrowserId,
        proxyBrowserId: current.proxyBrowserId === removedID ? proxyID : current.proxyBrowserId,
        urlRules: current.urlRules.map((rule) => (
          rule.action === 'browser' && rule.browserId === removedID
            ? { ...rule, browserId: fallbackID }
            : rule
        )),
      }
    })
  }

  async function browseForCustomBrowser(index) {
    const path = await onChooseBrowserApplication()
    if (!path) return
    updateCustomBrowser(index, { launchReference: path })
  }

  function updateRule(index, update) {
    setDraft((current) => ({
      ...current,
      urlRules: current.urlRules.map((rule, ruleIndex) => ruleIndex === index ? { ...rule, ...update } : rule),
    }))
  }

  function moveRule(index, offset) {
    setDraft((current) => {
      const nextRules = [...current.urlRules]
      const target = index + offset
      if (target < 0 || target >= nextRules.length) return current
      ;[nextRules[index], nextRules[target]] = [nextRules[target], nextRules[index]]
      return { ...current, urlRules: nextRules }
    })
  }

  function removeRule(index) {
    setDraft((current) => ({
      ...current,
      urlRules: current.urlRules.filter((_, ruleIndex) => ruleIndex !== index),
    }))
  }

  function addRule() {
    const browserId = draft.defaultBrowserId || browserChoices[0]?.id || ''
    setDraft((current) => ({
      ...current,
      urlRules: current.urlRules.concat({
        id: `rule-${Date.now().toString(36)}-${current.urlRules.length + 1}`,
        pattern: '',
        action: 'browser',
        browserId,
        command: '',
      }),
    }))
  }

  async function save() {
    setSaving(true)
    try {
      return await onSave({
        ...draft,
        urlRules: draft.urlRules.map((rule) => ({
          ...rule,
          browserId: rule.action === 'browser' ? rule.browserId : '',
          command: rule.action === 'command' ? rule.command : '',
        })),
      })
    } finally {
      setSaving(false)
    }
  }

  async function setDefault() {
    if (settingDefault) return
    setSettingDefault(true)
    try {
      const saved = await save()
      if (saved) await onSetDefault()
    } finally {
      setSettingDefault(false)
    }
  }

  return (
    <section className="section-block url-routing-settings" aria-labelledby="url-routing-heading">
      <div className="section-heading">
        <div>
          <span className="eyebrow">Links</span>
          <h2 id="url-routing-heading">URL routing</h2>
        </div>
        <span className={state.defaultBrowser.isDefault ? 'health-badge is-healthy' : 'health-badge'}>
          {state.defaultBrowser.isDefault ? 'Default' : 'Inactive'}
        </span>
      </div>
      <p className="section-description">
        Route matching URLs by rule. Unmatched localhost ports are checked against connected server proxies before using your fallback browser.
      </p>

      <div className="custom-browsers-heading">
        <div>
          <strong>Custom browsers</strong>
          <span>Add any installed application and choose how SSH Man should launch it.</span>
        </div>
        <button className="text-button" type="button" onClick={addCustomBrowser}>
          <Plus aria-hidden="true" /> Add custom browser
        </button>
      </div>

      {draft.customBrowsers.length ? (
        <ol className="custom-browser-list">
          {draft.customBrowsers.map((browser, index) => (
            <li className="custom-browser-card" key={browser.id}>
              <div className="custom-browser-card__header">
                <strong>Custom browser {index + 1}</strong>
                <IconButton label={`Remove custom browser ${index + 1}`} onClick={() => removeCustomBrowser(index)}>
                  <Trash2 aria-hidden="true" />
                </IconButton>
              </div>
              <label>
                <span>Name</span>
                <input
                  aria-label={`Custom browser ${index + 1} name`}
                  value={browser.displayName}
                  placeholder="Kagi Browser"
                  onChange={(event) => updateCustomBrowser(index, { displayName: event.target.value })}
                />
              </label>
              <label>
                <span>Application path</span>
                <div className="custom-browser-path">
                  <input
                    aria-label={`Custom browser ${index + 1} application path`}
                    value={browser.launchReference}
                    placeholder="/Applications/Browser.app"
                    onChange={(event) => updateCustomBrowser(index, { launchReference: event.target.value })}
                  />
                  <button
                    className="secondary-button"
                    type="button"
                    aria-label={`Browse for custom browser ${index + 1}`}
                    onClick={() => browseForCustomBrowser(index)}
                  >
                    <FolderOpen aria-hidden="true" /> Browse
                  </button>
                </div>
              </label>
              <label>
                <span>Launch compatibility</span>
                <select
                  aria-label={`Custom browser ${index + 1} engine`}
                  value={browser.engine}
                  onChange={(event) => updateCustomBrowser(index, { engine: event.target.value })}
                >
                  <option value="regular">Regular links only</option>
                  <option value="chromium">Chromium-compatible SOCKS profiles</option>
                  <option value="firefox">Firefox-compatible SOCKS profiles</option>
                </select>
                <small>Use regular-only if the app does not accept Chromium or Firefox profile arguments.</small>
              </label>
            </li>
          ))}
        </ol>
      ) : <p className="custom-browsers-empty">Chrome, Zen, Firefox, Edge, Arc, Brave, and other common browsers are detected automatically.</p>}

      <div className="url-routing-browser-grid">
        <label>
          <span>Fallback browser</span>
          <select
            aria-label="Fallback browser"
            value={draft.defaultBrowserId}
            disabled={state.loading || !browserChoices.length}
            onChange={(event) => setDraft((current) => ({ ...current, defaultBrowserId: event.target.value }))}
          >
            {browserChoices.map((browser) => <option key={browser.id} value={browser.id}>{browser.displayName}</option>)}
          </select>
        </label>
        <label>
          <span>SOCKS proxy browser</span>
          <select
            aria-label="SOCKS proxy browser"
            value={draft.proxyBrowserId}
            disabled={state.loading || !browserChoices.some((browser) => browser.supportsProxyLaunch)}
            onChange={(event) => setDraft((current) => ({ ...current, proxyBrowserId: event.target.value }))}
          >
            {browserChoices.filter((browser) => browser.supportsProxyLaunch).map((browser) => (
              <option key={browser.id} value={browser.id}>{browser.displayName}</option>
            ))}
          </select>
        </label>
      </div>

      <div className="default-browser-control">
        <Route aria-hidden="true" />
        <div>
          <strong>{state.defaultBrowser.isDefault ? 'SSH Man is your default browser.' : 'Make SSH Man your default browser'}</strong>
          <span>macOS will send HTTP and HTTPS links here for routing and may ask you to confirm.</span>
        </div>
        <button
          className="secondary-button"
          type="button"
          disabled={!state.defaultBrowser.supported || state.defaultBrowser.isDefault || settingDefault}
          onClick={setDefault}
        >
          {state.defaultBrowser.isDefault
            ? <><Check aria-hidden="true" /> Default</>
            : settingDefault
              ? <><LoaderCircle className="spin" aria-hidden="true" /> Waiting for macOS…</>
              : 'Make SSH Man default'}
        </button>
      </div>

      <div className="url-rules-heading">
        <div>
          <strong>Ordered rules</strong>
          <span>First matching regular expression wins.</span>
        </div>
        <button className="text-button" type="button" onClick={addRule}>
          <Plus aria-hidden="true" /> Add URL rule
        </button>
      </div>

      {draft.urlRules.length ? (
        <ol className="url-rule-list">
          {draft.urlRules.map((rule, index) => (
            <li key={rule.id} className="url-rule-card">
              <div className="url-rule-card__header">
                <strong>Rule {index + 1}</strong>
                <div>
                  <IconButton label={`Move rule ${index + 1} up`} disabled={index === 0} onClick={() => moveRule(index, -1)}>
                    <ArrowUp aria-hidden="true" />
                  </IconButton>
                  <IconButton label={`Move rule ${index + 1} down`} disabled={index === draft.urlRules.length - 1} onClick={() => moveRule(index, 1)}>
                    <ArrowDown aria-hidden="true" />
                  </IconButton>
                  <IconButton label={`Remove rule ${index + 1}`} onClick={() => removeRule(index)}>
                    <Trash2 aria-hidden="true" />
                  </IconButton>
                </div>
              </div>
              <label>
                <span>Regular expression</span>
                <input
                  aria-label={`Rule ${index + 1} regular expression`}
                  value={rule.pattern}
                  placeholder={'https:\\/\\/github\\.com\\/workorg\\/.*'}
                  onChange={(event) => updateRule(index, { pattern: event.target.value })}
                />
              </label>
              <label>
                <span>Action</span>
                <select
                  aria-label={`Rule ${index + 1} action`}
                  value={rule.action}
                  onChange={(event) => updateRule(index, { action: event.target.value })}
                >
                  <option value="browser">Open in browser</option>
                  <option value="command">Run command</option>
                </select>
              </label>
              {rule.action === 'command' ? (
                <label>
                  <span>Command template</span>
                  <textarea
                    aria-label={`Rule ${index + 1} command`}
                    value={rule.command}
                    placeholder={'open -a "Zen" "ext+container:name=Work&url=<URL>"'}
                    onChange={(event) => updateRule(index, { command: event.target.value })}
                  />
                  <small>Use <code>&lt;URL&gt;</code> where the safely escaped URL should be inserted.</small>
                </label>
              ) : (
                <label>
                  <span>Browser</span>
                  <select
                    aria-label={`Rule ${index + 1} browser`}
                    value={rule.browserId}
                    onChange={(event) => updateRule(index, { browserId: event.target.value })}
                  >
                    {browserChoices.map((browser) => <option key={browser.id} value={browser.id}>{browser.displayName}</option>)}
                  </select>
                </label>
              )}
            </li>
          ))}
        </ol>
      ) : <p className="url-rules-empty">No URL rules. Unmatched links use automatic localhost routing and the fallback browser.</p>}

      <button className="primary-button url-routing-save" type="button" disabled={saving || state.loading} onClick={save}>
        {saving ? <LoaderCircle className="spin" aria-hidden="true" /> : <Check aria-hidden="true" />}
        Save URL routing
      </button>
    </section>
  )
}

export function SettingsScreen({
  preferences,
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
  onChooseBrowserApplication,
  onSetDefaultBrowser,
  onReload,
  onRefreshRuntime,
  onCopyPath,
  onOpenDevTools,
  onQuit,
  standalone = false,
}) {
  return (
    <div className="screen-scroll settings-screen" aria-label="Settings">
      <section className="section-block" aria-labelledby="appearance-heading">
        <div className="section-heading">
          <div>
            <span className="eyebrow">Display</span>
            <h2 id="appearance-heading">Appearance</h2>
          </div>
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
          <div>
            <span className="eyebrow">Keyboard</span>
            <h2 id="shortcuts-heading">Quick browser switching</h2>
          </div>
          <button className="text-button" type="button" onClick={onOpenBrowserSwitcher}>Customize</button>
        </div>
        <p className="section-description">Set a recognizable icon and color for each browser, or preview the centered selector.</p>
        <ShortcutRecorder
          id="browser-switcher-forward-shortcut"
          label="Next browser"
          description="Opens the switcher and moves the selection forward."
          ariaLabel="Next browser shortcut"
          value={preferences.browserSwitcherShortcut}
          fallback="Alt+X"
          onChange={onSetBrowserSwitcherShortcut}
        />
        <ShortcutRecorder
          id="browser-switcher-backward-shortcut"
          label="Previous browser"
          description="Opens the switcher and moves the selection backward."
          ariaLabel="Previous browser shortcut"
          value={preferences.browserSwitcherBackwardShortcut}
          fallback="Alt+Z"
          onChange={onSetBrowserSwitcherBackwardShortcut}
        />
      </section>

      <URLRoutingSettings
        preferences={preferences}
        state={urlRoutingState}
        onLoad={onLoadURLRouting}
        onSave={onSaveURLRouting}
        onChooseBrowserApplication={onChooseBrowserApplication}
        onSetDefault={onSetDefaultBrowser}
      />

      <section className="section-block" aria-labelledby="health-heading">
        <div className="section-heading">
          <div>
            <span className="eyebrow">Diagnostics</span>
            <h2 id="health-heading">App health</h2>
          </div>
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
          <div><strong>Runtime status</strong><span>{runtimeFresh ? 'Live session polling is healthy.' : 'The last polling request failed.'}</span></div>
          <IconButton label="Refresh runtime status" onClick={() => onRefreshRuntime({ quiet: false })}>
            <RefreshCw aria-hidden="true" />
          </IconButton>
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
        <p>
          {standalone
            ? 'Your changes are saved immediately. Closing this window keeps SSH Man, tunnels, and explorer windows running.'
            : 'Closing the menu-bar window keeps tunnels and explorer windows running. Quit stops tunnels and closes explorer windows safely.'}
        </p>
      </section>
    </div>
  )
}
