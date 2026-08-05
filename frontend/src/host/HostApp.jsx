import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowUpRight,
  CircleAlert,
  Globe2,
  KeyRound,
  LoaderCircle,
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

export default function HostApp({ api = defaultApi }) {
  const [server, setServer] = useState(null)
  const [links, setLinks] = useState([])
  const [ports, setPorts] = useState([])
  const [phase, setPhase] = useState('loading')
  const [passphrase, setPassphrase] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [editing, setEditing] = useState(null)
  const [pendingPort, setPendingPort] = useState(0)
  const [saving, setSaving] = useState(false)
  const [findingFavicon, setFindingFavicon] = useState(false)
  const startedRef = useRef(false)

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
      setPassphrase('')
      setPhase('ready')
    } catch (nextError) {
      setError(nextError.message || 'The listening ports could not be loaded.')
      setPhase('error')
    }
  }, [api])

  useEffect(() => {
    if (startedRef.current) return
    startedRef.current = true
    async function start() {
      try {
        const state = await api.initialState()
        setServer(state.server)
        setLinks(state.links || [])
        document.title = `${state.server.name} — SSH Man`
        await discover('')
      } catch (nextError) {
        setError(nextError.message || 'The host window could not start.')
        setPhase('error')
      }
    }
    void start()
  }, [api, discover])

  const records = useMemo(() => portRecords(ports, links), [links, ports])

  async function openRecord(record) {
    if (!record.available || pendingPort) return
    const link = defaultLink(record, server.id)
    setPendingPort(record.port)
    setError('')
    setNotice('')
    try {
      const result = await api.openPort(record.port, link.scheme)
      await api.openExternalURL(result.url)
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
      setEditing(null)
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
      setLinks((current) => current.filter((link) => link.id !== editing.id))
      setEditing(null)
      setNotice('Saved link removed.')
    } catch (nextError) {
      setError(nextError.message || 'The saved link could not be removed.')
    } finally {
      setSaving(false)
    }
  }

  async function findFavicon() {
    if (!editing || findingFavicon) return
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
          <button className="primary-button" type="button" onClick={() => discover('')}>Try again</button>
        </main>
      ) : (
        <main className="host-content">
          <section className="host-intro">
            <div>
              <span className="eyebrow">Available now</span>
              <h2>{ports.length} listening port{ports.length === 1 ? '' : 's'}</h2>
              <p>Open a web service, or give it a name for later use.</p>
            </div>
          </section>

          {editing ? (
            <form className="host-link-editor" onSubmit={saveLink}>
              <div className="host-link-editor__heading">
                <div>
                  <span className="eyebrow">Port {editing.port}</span>
                  <h2>{editing.id ? 'Edit saved link' : 'Name this port'}</h2>
                </div>
                <IconButton label="Close link editor" disabled={saving || findingFavicon} onClick={() => setEditing(null)}>
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
                  disabled={saving || findingFavicon}
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

          {records.length ? (
            <ul className="host-port-list" aria-label="Host ports">
              {records.map((record) => {
                const link = defaultLink(record, server.id)
                const opening = pendingPort === record.port
                return (
                  <li key={record.port} className={record.available ? 'host-port-row' : 'host-port-row is-unavailable'}>
                    <button
                      className="host-port-open"
                      type="button"
                      aria-label={`Open ${link.name}`}
                      disabled={!record.available || Boolean(pendingPort)}
                      onClick={() => openRecord(record)}
                    >
                      <span className="host-port-icon"><PortIcon link={record.link} /></span>
                      <span className="host-port-copy">
                        <span className="host-port-title">
                          <strong>{link.name}</strong>
                          {record.link ? <span>Saved</span> : null}
                        </span>
                        <span>{link.scheme.toUpperCase()} · Port {record.port}</span>
                        <small>{record.available ? `Listening on ${record.addresses.join(', ')}` : 'Not listening now'}</small>
                      </span>
                      {opening ? <LoaderCircle className="spin" aria-hidden="true" /> : <ArrowUpRight aria-hidden="true" />}
                    </button>
                    <IconButton
                      label={record.link ? `Edit ${link.name}` : `Name port ${record.port}`}
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
        </main>
      )}
    </div>
  )
}
