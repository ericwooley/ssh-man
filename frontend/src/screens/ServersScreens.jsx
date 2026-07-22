import {
  ArrowRight,
  FolderOpen,
  KeyRound,
  LoaderCircle,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Server,
  ShieldCheck,
  Square,
  Trash2,
} from 'lucide-react'
import { configurationEndpoint, sessionCapabilities } from '../model/appModel'
import { EmptyState, IconButton, StatusPill } from '../components/AppChrome'

function serverLiveCount(item, sessions) {
  const ids = new Set(item.configurations.map((configuration) => configuration.id))
  return sessions.filter((session) => ids.has(session.configurationId) && ['starting', 'connected', 'reconnecting', 'needs_attention'].includes(session.status)).length
}

export function ServersScreen({ servers, sessions, onAdd, onOpen }) {
  if (!servers.length) {
    return (
      <div className="screen-scroll screen-scroll--centered" aria-label="Servers">
        <EmptyState
          icon={Server}
          title="Add your first server"
          description="Save the SSH connection once, then keep every forward and SOCKS proxy under it."
          action={(
            <button className="primary-button" type="button" onClick={onAdd}>
              <Plus aria-hidden="true" /> Add server
            </button>
          )}
        />
      </div>
    )
  }

  return (
    <div className="screen-scroll" aria-label="Servers">
      <section className="screen-intro">
        <div>
          <span className="eyebrow">Saved connections</span>
          <h2>{servers.length} server{servers.length === 1 ? '' : 's'}</h2>
          <p>Choose a server to manage its tunnels.</p>
        </div>
        <IconButton label="Add server" className="icon-button--accent" onClick={onAdd}>
          <Plus aria-hidden="true" />
        </IconButton>
      </section>

      <ul className="row-list" aria-label="Saved servers">
        {servers.map((item) => {
          const liveCount = serverLiveCount(item, sessions)
          return (
            <li key={item.server.id} className="server-row">
              <button className="row-main" type="button" onClick={() => onOpen(item.server.id)}>
                <span className="server-avatar" aria-hidden="true">{item.server.name.slice(0, 2).toUpperCase()}</span>
                <span className="row-copy">
                  <strong>{item.server.name}</strong>
                  <span className="row-meta">{item.server.username}@{item.server.host}:{item.server.port}</span>
                  <span className="row-detail">
                    {item.configurations.length} tunnel{item.configurations.length === 1 ? '' : 's'}
                    {liveCount ? <span className="live-inline"> · {liveCount} active</span> : null}
                  </span>
                </span>
                <ArrowRight aria-hidden="true" />
              </button>
            </li>
          )
        })}
      </ul>
    </div>
  )
}

export function TunnelRow({ configuration, session, pending, runtimeFresh = true, onOpen, onStart, onStop }) {
  const capabilities = sessionCapabilities(session)
  const isWorking = Boolean(pending)
  const actionLabel = !runtimeFresh
    ? `Refresh live status before controlling ${configuration.label}`
    : capabilities.canStop ? `Stop ${configuration.label}` : `Start ${configuration.label}`

  return (
    <li className="tunnel-row">
      <button className="row-main tunnel-row__main" type="button" onClick={() => onOpen(configuration.id)}>
        <span className="row-copy">
          <span className="row-heading">
            <strong>{configuration.label}</strong>
            <StatusPill status={session?.status || 'stopped'} compact />
          </span>
          <span className="row-meta endpoint-text">{configurationEndpoint(configuration, session)}</span>
          {session?.statusDetail ? <span className="row-detail line-clamp">{session.statusDetail}</span> : null}
        </span>
        <ArrowRight aria-hidden="true" />
      </button>
      <IconButton
        label={actionLabel}
        className={capabilities.canStop ? 'tunnel-quick-action is-stop' : 'tunnel-quick-action is-start'}
        disabled={!runtimeFresh || isWorking || (!capabilities.canStop && !capabilities.canStart)}
        onClick={() => capabilities.canStop ? onStop(configuration.id) : onStart(configuration.id)}
      >
        {isWorking ? <LoaderCircle className="spin" aria-hidden="true" /> : capabilities.canStop ? <Square aria-hidden="true" /> : <Play aria-hidden="true" />}
      </IconButton>
    </li>
  )
}

export function ServerDetailScreen({
  record,
  sessions,
  pending,
  runtimeFresh,
  onAddTunnel,
  onEditServer,
  onDeleteServer,
  onOpenTunnel,
  onStartTunnel,
  onStopTunnel,
  onStartAll,
  onRefreshRuntime,
  onExplore,
}) {
  const { server, configurations } = record
  const liveCount = serverLiveCount(record, sessions)
  const startableCount = configurations.filter((configuration) => {
    const session = sessions.find((item) => item.configurationId === configuration.id)
    return sessionCapabilities(session).canStart
  }).length
  const bulkPending = Boolean(pending[`start-all:${server.id}`])

  return (
    <div className="detail-layout" aria-label={`${server.name} tunnels`}>
      <div className="screen-scroll">
        <section className="server-summary card">
          <div className="server-summary__top">
            <span className="server-avatar server-avatar--large" aria-hidden="true">{server.name.slice(0, 2).toUpperCase()}</span>
            <div className="server-summary__copy">
              <span className="eyebrow">SSH server</span>
              <strong>{server.username}@{server.host}:{server.port}</strong>
              <span><KeyRound aria-hidden="true" /> {server.authMode === 'private_key' ? 'Private key' : 'SSH agent'}</span>
            </div>
            <IconButton label={`Edit ${server.name}`} onClick={() => onEditServer(server)}>
              <Pencil aria-hidden="true" />
            </IconButton>
          </div>
          <div className="server-summary__stats">
            <div><strong>{configurations.length}</strong><span>Tunnels</span></div>
            <div><strong>{liveCount}</strong><span>Active</span></div>
            <div className={runtimeFresh ? 'is-fresh' : 'is-stale'}>
              <ShieldCheck aria-hidden="true" />
              <span>{runtimeFresh ? 'Live status' : 'Status stale'}</span>
            </div>
          </div>
          <button
            className="primary-button primary-button--full"
            type="button"
            disabled={Boolean(pending[`explore-server:${server.id}`])}
            onClick={() => onExplore(server.id)}
          >
            {pending[`explore-server:${server.id}`]
              ? <LoaderCircle className="spin" aria-hidden="true" />
              : <FolderOpen aria-hidden="true" />}
            Explore files
          </button>
          {!runtimeFresh && configurations.length ? (
            <button
              className="secondary-button secondary-button--full"
              type="button"
              onClick={() => onRefreshRuntime({ quiet: false })}
            >
              <RefreshCw aria-hidden="true" /> Refresh live status
            </button>
          ) : startableCount ? (
            <button
              className="secondary-button secondary-button--full"
              type="button"
              disabled={bulkPending}
              onClick={() => onStartAll(server.id)}
            >
              {bulkPending ? <LoaderCircle className="spin" aria-hidden="true" /> : <Play aria-hidden="true" />}
              Start {startableCount === 1 ? 'inactive tunnel' : `all ${startableCount} inactive tunnels`}
            </button>
          ) : null}
        </section>

        <section className="section-block" aria-labelledby="tunnels-heading">
          <div className="section-heading">
            <div>
              <span className="eyebrow">This server</span>
              <h2 id="tunnels-heading">Tunnels</h2>
            </div>
            <IconButton label="Add tunnel" className="icon-button--accent" onClick={onAddTunnel}>
              <Plus aria-hidden="true" />
            </IconButton>
          </div>

          {configurations.length ? (
            <ul className="row-list row-list--compact">
              {configurations.map((configuration) => (
                <TunnelRow
                  key={configuration.id}
                  configuration={configuration}
                  session={sessions.find((session) => session.configurationId === configuration.id)}
                  pending={pending[`session:${configuration.id}`] || bulkPending}
                  runtimeFresh={runtimeFresh}
                  onOpen={onOpenTunnel}
                  onStart={onStartTunnel}
                  onStop={onStopTunnel}
                />
              ))}
            </ul>
          ) : (
            <EmptyState
              icon={Server}
              title="No tunnels yet"
              description="Add a local forward or SOCKS proxy for this server."
              action={(
                <button className="primary-button" type="button" onClick={onAddTunnel}>
                  <Plus aria-hidden="true" /> Add tunnel
                </button>
              )}
            />
          )}
        </section>

        <section className="danger-zone">
          <button className="text-button text-button--danger" type="button" onClick={() => onDeleteServer(server)}>
            <Trash2 aria-hidden="true" /> Delete server
          </button>
        </section>
      </div>
    </div>
  )
}
