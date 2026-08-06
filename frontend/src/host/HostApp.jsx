import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  AppWindow,
  ArrowUpRight,
  Clock3,
  CircleAlert,
  Gauge,
  Globe2,
  KeyRound,
  LoaderCircle,
  MemoryStick,
  Network,
  Pencil,
  RefreshCw,
  Save,
  Search,
  Server,
  Trash2,
  X,
} from 'lucide-react'
import { IconButton } from '../components/AppChrome'
import * as defaultApi from '../lib/hostApi'

function portRecords(ports, links) {
  const byPort = new Map()
  for (const port of ports) {
    byPort.set(port.port, { ...port, available: true, link: null })
  }
  for (const link of links) {
    const current = byPort.get(link.port) || {
      port: link.port,
      addresses: [],
      suggestedScheme: link.scheme || 'http',
      available: false,
    }
    byPort.set(link.port, { ...current, link })
  }
  return Array.from(byPort.values()).sort((first, second) => {
    if (Boolean(first.link) !== Boolean(second.link)) return first.link ? -1 : 1
    return first.port - second.port
  })
}

function defaultLink(record, serverId) {
  return record.link || {
    id: '',
    serverId,
    port: record.port,
    name: `Port ${record.port}`,
    scheme: record.suggestedScheme || 'http',
    faviconDataUrl: '',
  }
}

function PortIcon({ link }) {
  if (link?.faviconDataUrl) {
    return <img src={link.faviconDataUrl} alt="" />
  }
  return <Globe2 aria-hidden="true" />
}

function formatMemory(bytes) {
  if (bytes === undefined || bytes === null) return 'Unavailable'
  const gibibytes = bytes / (1024 ** 3)
  return `${gibibytes >= 10 ? Math.round(gibibytes) : gibibytes.toFixed(1)} GB`
}

function formatUptime(seconds) {
  if (!seconds) return 'Unavailable'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  if (days) return `${days}d ${hours}h`
  if (hours) return `${hours}h`
  return `${Math.max(1, Math.floor(seconds / 60))}m`
}

function applicationForPort(applications, port) {
  return applications.find((application) => application.ports?.includes(port))
}

export default function HostApp({ api = defaultApi }) {
  const [server, setServer] = useState(null)
  const [links, setLinks] = useState([])
  const [ports, setPorts] = useState([])
  const [metrics, setMetrics] = useState({})
  const [applications, setApplications] = useState([])
  const [phase, setPhase] = useState('loading')
  const [passphrase, setPassphrase] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [editing, setEditing] = useState(null)
  const [pendingPort, setPendingPort] = useState(0)
  const [saving, setSaving] = useState(false)
  const [findingFavicon, setFindingFavicon] = useState(false)
  const startedRef = useRef(false)
  const editButtonRefs = useRef(new Map())

  const discover = useCallback(async (secret = '') => {
    setPhase('discovering')
    setError('')
    setNotice('')
    try {
      const result = await api.discoverPorts(secret)
      if (result.needsPassphrase) {
        setPhase('locked')
        return
      }
      setPorts(result.ports || [])
      setMetrics(result.metrics || {})
      setApplications(result.applications || [])
      setPassphrase('')
      setPhase('ready')
    } catch (nextError) {
      setError(nextError.message || 'The listening ports could not be loaded.')
      setPhase('error')
    }
  }, [api])

  const start = useCallback(async () => {
    setPhase('loading')
    setError('')
    setNotice('')
    try {
      const state = await api.initialState()
      setServer(state.server)
      setLinks(state.links || [])
      const theme = state.theme === 'light' ? 'light' : 'dark'
      document.documentElement.dataset.theme = theme
      document.documentElement.style.colorScheme = theme
      document.title = `${state.server.name} — SSH Man`
      await discover('')
    } catch (nextError) {
      setError(nextError.message || 'The host window could not start.')
      setPhase('error')
    }
  }, [api, discover])

  useEffect(() => {
    if (startedRef.current) return
    startedRef.current = true
    void start()
  }, [start])

  const records = useMemo(() => portRecords(ports, links), [links, ports])
  const favoriteRecords = useMemo(() => records.filter((record) => record.link), [records])
  const openRecords = useMemo(() => records.filter((record) => record.available), [records])
  const memoryUsed = Math.max(0, (metrics.memoryTotalBytes || 0) - (metrics.memoryAvailableBytes || 0))
  const memoryPercent = metrics.memoryTotalBytes
    ? Math.min(100, Math.round((memoryUsed / metrics.memoryTotalBytes) * 100))
    : 0

  async function openRecord(record) {
    if (!record.available || pendingPort) return
    const link = defaultLink(record, server.id)
    setPendingPort(record.port)
    setError('')
    setNotice('')
    try {
      await api.openPort(record.port, link.scheme)
      setNotice(`Opened ${link.name}.`)
    } catch (nextError) {
      setError(nextError.message || `${link.name} could not be opened.`)
    } finally {
      setPendingPort(0)
    }
  }

  function editRecord(record) {
    setEditing(defaultLink(record, server.id))
    setError('')
    setNotice('')
  }

  function closeEditor(port) {
    setEditing(null)
    window.setTimeout(() => {
      editButtonRefs.current.get(port)?.focus()
    }, 0)
  }

  async function saveLink(event) {
    event.preventDefault()
    const name = String(editing?.name || '').trim()
    if (!name) {
      setError('Enter a name for this port.')
      return
    }
    setSaving(true)
    setError('')
    try {
      const saved = await api.savePortLink({ ...editing, name })
      setLinks((current) => current.filter((link) => link.port !== saved.port).concat(saved))
      closeEditor(saved.port)
      setNotice(`${saved.name} saved.`)
    } catch (nextError) {
      setError(nextError.message || 'The port name could not be saved.')
    } finally {
      setSaving(false)
    }
  }

  async function deleteLink() {
    if (!editing?.id || saving) return
    setSaving(true)
    setError('')
    try {
      await api.deletePortLink(editing.id)
      const deletedPort = editing.port
      setLinks((current) => current.filter((link) => link.id !== editing.id))
      closeEditor(deletedPort)
      setNotice('Saved link removed.')
    } catch (nextError) {
      setError(nextError.message || 'The saved link could not be removed.')
    } finally {
      setSaving(false)
    }
  }

  async function findFavicon() {
    if (!editing || saving || findingFavicon) return
    setFindingFavicon(true)
    setError('')
    setNotice('')
    try {
      const result = await api.findPortFavicon(editing.port, editing.scheme)
      if (!result.faviconDataUrl) {
        throw new Error('The service did not return a favicon.')
      }
      setEditing((current) => current ? { ...current, faviconDataUrl: result.faviconDataUrl } : current)
      setNotice('Favicon found. Save the link to keep it.')
    } catch (nextError) {
      setError(nextError.message || 'A favicon was not found for this service.')
    } finally {
      setFindingFavicon(false)
    }
  }

  if (!server && phase !== 'error') {
    return (
      <div className="host-shell host-state" role="status">
        <LoaderCircle className="spin" aria-hidden="true" />
        <strong>Opening host details</strong>
      </div>
    )
  }

  return (
    <div className="host-shell">
      <header className="host-toolbar">
        <span className="host-toolbar__mark" aria-hidden="true"><Server /></span>
        <div>
          <h1>{server?.name || 'Host details'}</h1>
          {server ? <p>{server.username}@{server.host}:{server.port}</p> : null}
        </div>
        <div className="host-toolbar__actions">
          <IconButton label="Refresh available ports" disabled={phase === 'discovering'} onClick={() => discover('')}>
            <RefreshCw className={phase === 'discovering' ? 'spin' : ''} aria-hidden="true" />
          </IconButton>
          <IconButton label="Close host window" onClick={() => api.close()}>
            <X aria-hidden="true" />
          </IconButton>
        </div>
      </header>

      {error ? (
        <div className="host-alert" role="alert">
          <CircleAlert aria-hidden="true" />
          <span>{error}</span>
        </div>
      ) : null}
      {notice ? <div className="host-notice" role="status">{notice}</div> : null}

      {phase === 'locked' ? (
        <main className="host-state">
          <span className="host-state__icon" aria-hidden="true"><KeyRound /></span>
          <h2>Unlock this host</h2>
          <p>Enter the SSH key passphrase to find listening ports.</p>
          <form onSubmit={(event) => {
            event.preventDefault()
            void discover(passphrase)
          }}>
            <label>
              <span>SSH key passphrase</span>
              <input
                type="password"
                autoFocus
                value={passphrase}
                onChange={(event) => setPassphrase(event.target.value)}
              />
            </label>
            <button className="primary-button" type="submit">Unlock host</button>
          </form>
        </main>
      ) : phase === 'loading' || phase === 'discovering' ? (
        <main className="host-state" role="status">
          <LoaderCircle className="spin" aria-hidden="true" />
          <strong>Finding listening ports</strong>
          <span>Checking services on this host…</span>
        </main>
      ) : phase === 'error' ? (
        <main className="host-state">
          <span className="host-state__icon" aria-hidden="true"><CircleAlert /></span>
          <h2>Host details did not load</h2>
          <p>Check the SSH connection, then try again.</p>
          <button className="primary-button" type="button" onClick={() => void start()}>Try again</button>
        </main>
      ) : (
        <main className="host-content">
          <section className="host-summary" aria-label="Host summary">
            <div className="host-metric">
              <span className="host-metric__icon" aria-hidden="true"><MemoryStick /></span>
              <span className="host-metric__copy">
                <span>Memory</span>
                <strong>{metrics.memoryTotalBytes ? `${formatMemory(memoryUsed)} used` : 'Unavailable'}</strong>
                <small>{metrics.memoryTotalBytes ? `${memoryPercent}% of ${formatMemory(metrics.memoryTotalBytes)}` : 'Remote metric unavailable'}</small>
              </span>
              {metrics.memoryTotalBytes ? (
                <span className="host-memory-bar" aria-label={`${memoryPercent}% memory used`}>
                  <span style={{ width: `${memoryPercent}%` }} />
                </span>
              ) : null}
            </div>
            <div className="host-metric">
              <span className="host-metric__icon" aria-hidden="true"><Clock3 /></span>
              <span className="host-metric__copy">
                <span>Uptime</span>
                <strong>{formatUptime(metrics.uptimeSeconds)}</strong>
                <small>Since the last host restart</small>
              </span>
            </div>
            <div className="host-metric">
              <span className="host-metric__icon" aria-hidden="true"><Gauge /></span>
              <span className="host-metric__copy">
                <span>Load</span>
                <strong>{metrics.cpuCount ? Number(metrics.loadOne || 0).toFixed(2) : 'Unavailable'}</strong>
                <small>{metrics.cpuCount ? `${metrics.cpuCount} CPU core${metrics.cpuCount === 1 ? '' : 's'}` : 'CPU count unavailable'}</small>
              </span>
            </div>
            <div className="host-metric">
              <span className="host-metric__icon" aria-hidden="true"><Network /></span>
              <span className="host-metric__copy">
                <span>Open ports</span>
                <strong>{ports.length}</strong>
                <small>{favoriteRecords.length} favorite{favoriteRecords.length === 1 ? '' : 's'}</small>
              </span>
            </div>
          </section>

          {editing ? (
            <form className="host-link-editor" onSubmit={saveLink}>
              <div className="host-link-editor__heading">
                <div>
                  <span className="eyebrow">Port {editing.port}</span>
                  <h2>{editing.id ? 'Edit saved link' : 'Name this port'}</h2>
                </div>
                <IconButton label="Close link editor" disabled={saving || findingFavicon} onClick={() => closeEditor(editing.port)}>
                  <X aria-hidden="true" />
                </IconButton>
              </div>
              <div className="host-link-editor__fields">
                <label>
                  <span>Name</span>
                  <input
                    autoFocus
                    value={editing.name}
                    onChange={(event) => setEditing({ ...editing, name: event.target.value })}
                  />
                </label>
                <label>
                  <span>Protocol</span>
                  <select value={editing.scheme} onChange={(event) => setEditing({ ...editing, scheme: event.target.value })}>
                    <option value="http">HTTP</option>
                    <option value="https">HTTPS</option>
                  </select>
                </label>
              </div>
              <div className="host-favicon-picker">
                <span className="host-port-icon">
                  {editing.faviconDataUrl ? (
                    <img src={editing.faviconDataUrl} alt="Selected favicon" />
                  ) : (
                    <Globe2 aria-hidden="true" />
                  )}
                </span>
                <span className="host-favicon-picker__copy">
                  <strong>Link icon</strong>
                  <span>Find the favicon from this web service.</span>
                </span>
                {editing.faviconDataUrl ? (
                  <button
                    className="text-button"
                    type="button"
                    disabled={saving || findingFavicon}
                    onClick={() => setEditing({ ...editing, faviconDataUrl: '' })}
                  >
                    Remove icon
                  </button>
                ) : null}
                <button
                  className="secondary-button"
                  type="button"
                  disabled={saving}
                  aria-busy={findingFavicon}
                  aria-disabled={findingFavicon}
                  onClick={findFavicon}
                >
                  {findingFavicon ? <LoaderCircle className="spin" aria-hidden="true" /> : <Search aria-hidden="true" />}
                  Find favicon
                </button>
              </div>
              <div className="host-link-editor__actions">
                {editing.id ? (
                  <button className="text-button text-button--danger" type="button" disabled={saving} onClick={deleteLink}>
                    <Trash2 aria-hidden="true" /> Remove
                  </button>
                ) : <span />}
                <button className="primary-button" type="submit" disabled={saving || findingFavicon}>
                  {saving ? <LoaderCircle className="spin" aria-hidden="true" /> : <Save aria-hidden="true" />}
                  Save link
                </button>
              </div>
            </form>
          ) : null}

          <section className="host-dashboard-section host-favorites">
            <div className="host-dashboard-heading">
              <div>
                <span className="eyebrow">Saved links</span>
                <h2>Favorite ports</h2>
              </div>
              <span>{favoriteRecords.filter((record) => record.available).length} running</span>
            </div>
            {favoriteRecords.length ? (
              <div className="host-favorite-grid">
                {favoriteRecords.map((record) => {
                  const link = defaultLink(record, server.id)
                  const opening = pendingPort === record.port
                  return (
                    <button
                      key={record.port}
                      className="host-favorite"
                      type="button"
                      aria-label={`Open ${link.name}`}
                      aria-busy={opening}
                      aria-disabled={!record.available || Boolean(pendingPort)}
                      disabled={!record.available}
                      onClick={() => openRecord(record)}
                    >
                      <span className="host-port-icon"><PortIcon link={record.link} /></span>
                      <span className="host-port-copy">
                        <strong>{link.name}</strong>
                        <span>{link.scheme.toUpperCase()} port {record.port}</span>
                        <small>{record.available ? record.addresses.join(', ') : 'Start the service to open it'}</small>
                      </span>
                      <span className={`host-status ${record.available ? 'is-running' : 'is-stopped'}`}>
                        {record.available ? 'Running' : 'Stopped'}
                      </span>
                    </button>
                  )
                })}
              </div>
            ) : (
              <p className="host-section-empty">Named ports will appear here.</p>
            )}
          </section>

          <div className="host-dashboard-grid">
            <section className="host-dashboard-section">
              <div className="host-dashboard-heading">
                <div>
                  <span className="eyebrow">Processes</span>
                  <h2>Applications</h2>
                </div>
                <span>{applications.length}</span>
              </div>
              {applications.length ? (
                <ul className="host-application-list">
                  {applications.map((application) => (
                    <li key={`${application.name}-${application.pid}`} className="host-application-row">
                      <span className="host-application-icon" aria-hidden="true"><AppWindow /></span>
                      <span>
                        <strong>{application.name}</strong>
                        <small>PID {application.pid}</small>
                        <span className="host-port-chips">
                          {application.ports.map((port) => <span key={port}>{port}</span>)}
                        </span>
                      </span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="host-section-empty">Process details are not available for this host.</p>
              )}
            </section>

            <section className="host-dashboard-section">
              <div className="host-dashboard-heading">
                <div>
                  <span className="eyebrow">Listening now</span>
                  <h2>All open ports</h2>
                </div>
                <span>{openRecords.length}</span>
              </div>
              {openRecords.length ? (
                <ul className="host-port-list" aria-label="Host ports">
                  {openRecords.map((record) => {
                const link = defaultLink(record, server.id)
                const opening = pendingPort === record.port
                const application = applicationForPort(applications, record.port)
                return (
                  <li key={record.port} className="host-port-row">
                    <button
                      className="host-port-open"
                      type="button"
                      aria-label={record.link ? `Open port ${record.port}` : `Open ${link.name}`}
                      aria-busy={opening}
                      aria-disabled={Boolean(pendingPort)}
                      onClick={() => openRecord(record)}
                    >
                      <span className="host-port-icon"><PortIcon link={record.link} /></span>
                      <span className="host-port-copy">
                        <span className="host-port-title">
                          <strong>{link.name}</strong>
                        </span>
                        <span>{link.scheme.toUpperCase()} port {record.port}</span>
                        <small>{application ? `${application.name} · ` : ''}{record.addresses.join(', ')}</small>
                      </span>
                      {opening ? <LoaderCircle className="spin" aria-hidden="true" /> : <ArrowUpRight aria-hidden="true" />}
                    </button>
                    <IconButton
                      label={record.link ? `Edit ${link.name}` : `Name port ${record.port}`}
                      ref={(node) => {
                        if (node) {
                          editButtonRefs.current.set(record.port, node)
                        } else {
                          editButtonRefs.current.delete(record.port)
                        }
                      }}
                      onClick={() => editRecord(record)}
                    >
                      <Pencil aria-hidden="true" />
                    </IconButton>
                  </li>
                )
              })}
                </ul>
              ) : (
                <div className="host-empty">
                  <Globe2 aria-hidden="true" />
                  <h2>No listening ports found</h2>
                  <p>Refresh after a service starts on this host.</p>
                </div>
              )}
            </section>
          </div>
        </main>
      )}
    </div>
  )
}
