import {
  ArrowRight,
  FolderOpen,
  Globe2,
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
import {
  configurationEndpoint,
  isManagedSOCKSConfiguration,
  sessionCapabilities,
  userConfigurations,
} from '../model/appModel'
import { EmptyState, IconButton, StatusPill } from '../components/AppChrome'

function serverLiveCount(item, sessions) {
  const ids = new Set(item.configurations.map((configuration) => configuration.id))
  return sessions.filter((session) => ids.has(session.configurationId) && ['starting', 'connected', 'reconnecting', 'needs_attention'].includes(session.status)).length
}

export function ServersScreen({
  servers,
  sessions,
  pending = {},
  onAdd,
  onOpen,
  onOpenBrowser,
  onOpenExplorer,
}) {
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
          const tunnelCount = userConfigurations(item.configurations).length
          const managedProxy = item.configurations.find(isManagedSOCKSConfiguration) || null
          const managedSession = managedProxy
            ? sessions.find((session) => session.configurationId === managedProxy.id) || null
            : null
          const browserPending = Boolean(pending[`browser:${item.server.id}`])
          const explorerPending = Boolean(pending[`explore-server:${item.server.id}`])
          return (
            <li key={item.server.id} className="server-row">
              <button
                className="row-main server-row__main"
                type="button"
                aria-label={`Open ${item.server.name} details`}
                onClick={() => onOpen(item.server.id)}
              >
                <span className="server-avatar" aria-hidden="true">{item.server.name.slice(0, 2).toUpperCase()}</span>
                <span className="row-copy">
                  <strong>{item.server.name}</strong>
                  <span className="row-meta">{item.server.username}@{item.server.host}:{item.server.port}</span>
                  <span className="row-detail">
                    {tunnelCount} tunnel{tunnelCount === 1 ? '' : 's'}
                    {liveCount ? <span className="live-inline"> · {liveCount} active</span> : null}
                  </span>
                </span>
              </button>
              <div className="server-row__actions">
                <IconButton
                label={`Open browser through ${item.server.name}`}
                  className={`server-row__quick-action is-browser ${managedSession?.status === 'connected' ? 'is-connected' : ''}`.trim()}
                  disabled={!managedProxy || browserPending}
                  onClick={() => onOpenBrowser(item.server.id)}
                >
                  {browserPending
                    ? <LoaderCircle className="spin" aria-hidden="true" />
                    : <Globe2 aria-hidden="true" />}
                </IconButton>
                <IconButton
                  label={`Open ${item.server.name} explorer`}
                  className="server-row__quick-action is-explorer"
                  disabled={explorerPending}
                  onClick={() => onOpenExplorer(item.server.id)}
                >
                  {explorerPending
                    ? <LoaderCircle className="spin" aria-hidden="true" />
                    : <FolderOpen aria-hidden="true" />}
                </IconButton>
                <IconButton
                  label={`Show ${item.server.name} details`}
                  className="server-row__details"
                  onClick={() => onOpen(item.server.id)}
                >
                  <ArrowRight aria-hidden="true" />
                </IconButton>
              </div>
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
}) {
  const { server, configurations } = record
  const tunnels = userConfigurations(configurations)
  const managedProxy = configurations.find(isManagedSOCKSConfiguration) || null
  const managedSession = managedProxy
    ? sessions.find((session) => session.configurationId === managedProxy.id) || null
    : null
  const managedCapabilities = sessionCapabilities(managedSession)
  const liveCount = serverLiveCount(record, sessions)
  const startableCount = tunnels.filter((configuration) => {
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
            <div><strong>{tunnels.length}</strong><span>Tunnels</span></div>
            <div><strong>{liveCount}</strong><span>Active</span></div>
            <div className={runtimeFresh ? 'is-fresh' : 'is-stale'}>
              <ShieldCheck aria-hidden="true" />
              <span>{runtimeFresh ? 'Live status' : 'Status stale'}</span>
            </div>
          </div>
          <div className="managed-proxy-line">
            <div>
              <Globe2 aria-hidden="true" />
              <span><strong>Browser proxy</strong><small>localhost:{server.socksPort}</small></span>
            </div>
            <StatusPill status={managedSession?.status || 'stopped'} compact />
            {managedProxy && managedCapabilities.canStop ? (
              <IconButton
                label="Stop browser proxy"
                className="tunnel-quick-action is-stop"
                disabled={Boolean(pending[`session:${managedProxy.id}`])}
                onClick={() => onStopTunnel(managedProxy.id)}
              >
                {pending[`session:${managedProxy.id}`]
                  ? <LoaderCircle className="spin" aria-hidden="true" />
                  : <Square aria-hidden="true" />}
              </IconButton>
            ) : null}
          </div>
          {!runtimeFresh && tunnels.length ? (
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

          {tunnels.length ? (
            <ul className="row-list row-list--compact">
              {tunnels.map((configuration) => (
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
