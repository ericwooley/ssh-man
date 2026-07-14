import { useMemo, useState } from 'react'
import {
  Check,
  ChevronDown,
  Clock3,
  Copy,
  ExternalLink,
  Globe2,
  KeyRound,
  LoaderCircle,
  Pencil,
  Play,
  RefreshCw,
  RotateCcw,
  Square,
  Trash2,
  WifiOff,
} from 'lucide-react'
import { configurationEndpoint, sessionCapabilities } from '../model/appModel'
import { EmptyState, IconButton, StatusPill } from '../components/AppChrome'

function HistoryList({ history, loading, onCopy }) {
  const [expanded, setExpanded] = useState(false)
  const visible = expanded ? history : history.slice(0, 5)
  const hidden = Math.max(history.length - visible.length, 0)

  return (
    <section className="section-block history-section" aria-labelledby="history-heading">
      <div className="section-heading">
        <div>
          <span className="eyebrow">Troubleshooting trail</span>
          <h2 id="history-heading">Recent history</h2>
        </div>
        <IconButton label="Copy connection history" disabled={!history.length} onClick={onCopy}>
          <Copy aria-hidden="true" />
        </IconButton>
      </div>

      {loading && !history.length ? (
        <div className="inline-loading" role="status"><LoaderCircle className="spin" aria-hidden="true" /> Loading history…</div>
      ) : history.length ? (
        <>
          <ol className="timeline" aria-label="Recent connection history entries">
            {visible.map((entry) => (
              <li key={entry.id}>
                <span className="timeline__marker" aria-hidden="true" />
                <div>
                  <span className="timeline__topline">
                    <strong>{entry.outcome.replaceAll('_', ' ')}</strong>
                    <time dateTime={entry.endedAt || entry.startedAt}>{new Date(entry.endedAt || entry.startedAt).toLocaleString()}</time>
                  </span>
                  <p>{entry.message}</p>
                </div>
              </li>
            ))}
          </ol>
          {history.length > 5 ? (
            <button className="text-button history-more" type="button" aria-expanded={expanded} onClick={() => setExpanded((value) => !value)}>
              {expanded ? 'Show recent only' : `Show ${hidden} older entr${hidden === 1 ? 'y' : 'ies'}`}
              <ChevronDown className={expanded ? 'rotate-180' : ''} aria-hidden="true" />
            </button>
          ) : null}
        </>
      ) : (
        <div className="compact-empty">
          <Clock3 aria-hidden="true" />
          <div><strong>No connection events yet</strong><span>Start this tunnel to build a history.</span></div>
        </div>
      )}
    </section>
  )
}

function BrowserLauncher({ configuration, session, browserState, pending, onRefresh, onSelect, onLaunch }) {
  const selectedBrowser = browserState.items.find((browser) => browser.id === browserState.selectedId) || null
  const canUseTunnel = session?.status === 'connected'
  const canLaunch = canUseTunnel && selectedBrowser?.supportsProxyLaunch

  return (
    <section className="section-block browser-section" aria-labelledby="browser-heading">
      <div className="section-heading">
        <div>
          <span className="eyebrow">SOCKS workflow</span>
          <h2 id="browser-heading">Browser through tunnel</h2>
        </div>
        <IconButton label="Refresh installed browsers" disabled={browserState.loading} onClick={() => onRefresh(configuration.id)}>
          <RefreshCw className={browserState.loading ? 'spin' : ''} aria-hidden="true" />
        </IconButton>
      </div>

      {!canUseTunnel ? (
        <div className="compact-empty compact-empty--warning">
          <WifiOff aria-hidden="true" />
          <div><strong>Start the SOCKS tunnel first</strong><span>The launch controls will unlock when it connects.</span></div>
        </div>
      ) : browserState.items.length ? (
        <div className="browser-controls">
          <label className="field-group" htmlFor="browser-select">
            <span>Installed browser</span>
            <select id="browser-select" value={browserState.selectedId} onChange={(event) => onSelect(event.target.value)}>
              <option value="">Choose a browser</option>
              {browserState.items.map((browser) => (
                <option key={browser.id} value={browser.id}>
                  {browser.displayName}{browser.supportsProxyLaunch ? '' : ' — unsupported'}
                </option>
              ))}
            </select>
          </label>

          {selectedBrowser && !selectedBrowser.supportsProxyLaunch ? (
            <p className="field-message field-message--error" role="alert">
              {selectedBrowser.displayName} cannot be launched with an isolated SOCKS proxy. Choose Chrome, Chromium, Brave, or Firefox.
            </p>
          ) : null}

          <button className="primary-button primary-button--full" type="button" disabled={!canLaunch || pending} onClick={onLaunch}>
            {pending ? <LoaderCircle className="spin" aria-hidden="true" /> : <Globe2 aria-hidden="true" />}
            Launch through SOCKS
            <ExternalLink aria-hidden="true" />
          </button>

          {browserState.preview ? (
            <details className="command-preview">
              <summary>Preview launch command</summary>
              <code>{browserState.preview}</code>
            </details>
          ) : null}
        </div>
      ) : browserState.loading ? (
        <div className="inline-loading" role="status"><LoaderCircle className="spin" aria-hidden="true" /> Finding browsers…</div>
      ) : (
        <div className="compact-empty">
          <Globe2 aria-hidden="true" />
          <div><strong>No supported browsers found</strong><span>Install Chrome, Chromium, Brave, or Firefox and refresh.</span></div>
        </div>
      )}
    </section>
  )
}

export function TunnelDetailScreen({
  configuration,
  session,
  history,
  historyLoading,
  runtimeFresh,
  browserState,
  pending,
  onStart,
  onStop,
  onRetry,
  onEdit,
  onDelete,
  onCopyHistory,
  onRefreshBrowsers,
  onSelectBrowser,
  onLaunchBrowser,
  onUnlock,
  onRefreshRuntime,
}) {
  const capabilities = sessionCapabilities(session)
  const status = session?.status || 'stopped'
  const isWorking = Boolean(pending[`session:${configuration.id}`])
  const primaryAction = !runtimeFresh
    ? 'refresh'
    : status === 'needs_attention'
      ? 'unlock'
      : capabilities.canStop ? 'stop' : status === 'failed' ? 'retry' : 'start'

  const action = useMemo(() => {
    if (primaryAction === 'refresh') return { label: 'Refresh live status', Icon: RefreshCw, handler: () => onRefreshRuntime({ quiet: false }), tone: 'primary' }
    if (primaryAction === 'unlock') return { label: 'Unlock SSH key', Icon: KeyRound, handler: () => onUnlock(configuration.id), tone: 'primary' }
    if (primaryAction === 'stop') return { label: 'Stop tunnel', Icon: Square, handler: () => onStop(configuration.id), tone: 'danger' }
    if (primaryAction === 'retry') return { label: 'Retry connection', Icon: RotateCcw, handler: () => onRetry(configuration.id), tone: 'primary' }
    return { label: 'Start tunnel', Icon: Play, handler: () => onStart(configuration.id), tone: 'primary' }
  }, [configuration.id, onRefreshRuntime, onRetry, onStart, onStop, onUnlock, primaryAction])

  return (
    <div className="detail-layout tunnel-detail" aria-label={`${configuration.label} details`}>
      <div className="screen-scroll">
        <section className={`runtime-hero tone-${status === 'connected' ? 'positive' : status === 'failed' ? 'danger' : status === 'needs_attention' || status === 'reconnecting' ? 'warning' : 'neutral'}`}>
          <div className="runtime-hero__topline">
            <StatusPill status={status} />
            <IconButton label={`Edit ${configuration.label}`} onClick={() => onEdit(configuration)}>
              <Pencil aria-hidden="true" />
            </IconButton>
          </div>
          <h2>{configuration.label}</h2>
          <p className="endpoint-text">{configurationEndpoint(configuration, session)}</p>
          <div className="runtime-message" role="status" aria-live="polite">
            {session?.statusDetail || 'This saved tunnel is ready when you are.'}
          </div>
          {!runtimeFresh ? <p className="stale-warning"><WifiOff aria-hidden="true" /> Live status is temporarily unavailable.</p> : null}
        </section>

        <section className="connection-facts card" aria-label="Tunnel settings summary">
          <div><span>Type</span><strong>{configuration.connectionType === 'socks_proxy' ? 'SOCKS proxy' : 'Local forward'}</strong></div>
          <div><span>Reconnect</span><strong>{configuration.autoReconnectEnabled ? <><Check aria-hidden="true" /> Automatic</> : 'Off'}</strong></div>
          {session?.boundPort ? <div><span>Bound port</span><strong>{session.boundPort}</strong></div> : null}
          {session?.reconnectAttemptCount ? <div><span>Retry attempts</span><strong>{session.reconnectAttemptCount}</strong></div> : null}
          {configuration.notes ? <div className="connection-facts__notes"><span>Notes</span><p>{configuration.notes}</p></div> : null}
        </section>

        {configuration.connectionType === 'socks_proxy' ? (
          <BrowserLauncher
            configuration={configuration}
            session={session}
            browserState={browserState}
            pending={Boolean(pending['launch-browser'])}
            onRefresh={onRefreshBrowsers}
            onSelect={onSelectBrowser}
            onLaunch={onLaunchBrowser}
          />
        ) : null}

        <HistoryList history={history} loading={historyLoading} onCopy={() => onCopyHistory(configuration.id)} />

        <section className="danger-zone">
          <button className="text-button text-button--danger" type="button" onClick={() => onDelete(configuration)}>
            <Trash2 aria-hidden="true" /> Delete tunnel
          </button>
        </section>
      </div>

      <footer className="persistent-action-bar">
        <button
          className={action.tone === 'danger' ? 'danger-button danger-button--full' : 'primary-button primary-button--full'}
          type="button"
          disabled={isWorking || (!['refresh', 'unlock'].includes(primaryAction) && !capabilities.canStart && !capabilities.canStop)}
          onClick={action.handler}
        >
          {isWorking ? <LoaderCircle className="spin" aria-hidden="true" /> : <action.Icon aria-hidden="true" />}
          {isWorking ? 'Working…' : action.label}
        </button>
      </footer>
    </div>
  )
}
