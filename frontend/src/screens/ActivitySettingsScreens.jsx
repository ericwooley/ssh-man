import {
  ArrowRight,
  LoaderCircle,
  RefreshCw,
  Square,
  Wifi,
  WifiOff,
} from 'lucide-react'

import { EmptyState, IconButton, StatusPill } from '../components/AppChrome'
import { configurationEndpoint, findConfigurationRecord } from '../model/appModel'

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
