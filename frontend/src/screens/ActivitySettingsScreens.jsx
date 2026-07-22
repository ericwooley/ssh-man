import { useState } from 'react'
import {
  ArrowRight,
  Check,
  Copy,
  Database,
  ExternalLink,
  FolderOpen,
  LoaderCircle,
  Keyboard,
  Moon,
  Power,
  RefreshCw,
  Server,
  Square,
  Sun,
  SquareTerminal,
  Wifi,
  WifiOff,
} from 'lucide-react'
import moonpixelsLogo from '../../../moonpixels.png'
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

export function SettingsScreen({
  preferences,
  diagnostics,
  storageIssue,
  runtimeFresh,
  onToggleTheme,
  onSetBrowserSwitcherShortcut,
  onSetBrowserSwitcherBackwardShortcut,
  onOpenBrowserSwitcher,
  onReload,
  onRefreshRuntime,
  onCopyPath,
  onOpenDevTools,
  onOpenExternalURL,
  onQuit,
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

      <section className="about-card card">
        <img src={moonpixelsLogo} alt="MoonPixels" />
        <div>
          <span className="eyebrow">About SSH Man</span>
          <h2>Gifted with love by MoonPixels</h2>
          <p>Custom apps and fast MVPs for startups and small teams.</p>
          <button className="text-button" type="button" onClick={() => onOpenExternalURL('https://moonpixels.tech')}>
            moonpixels.tech <ExternalLink aria-hidden="true" />
          </button>
        </div>
      </section>

      <section className="quit-section">
        <button className="secondary-button secondary-button--danger secondary-button--full" type="button" onClick={onQuit}>
          <Power aria-hidden="true" /> Quit SSH Man
        </button>
        <p>Closing the menu-bar window keeps your tunnels running. Quit stops them safely.</p>
      </section>
    </div>
  )
}
