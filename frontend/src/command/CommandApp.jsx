import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  CheckCircle2,
  CircleAlert,
  Clock,
  Copy,
  File,
  Folder,
  KeyRound,
  LoaderCircle,
  Play,
  Square,
  Terminal,
  Trash2,
  X,
} from 'lucide-react'
import { IconButton } from '../components/AppChrome'
import * as defaultApi from '../lib/commandApi'
import { activeShellWord, applyPathCompletion } from './commandCompletion'

async function writeClipboard(text) {
  if (!navigator.clipboard?.writeText) {
    throw new Error('Clipboard access is unavailable.')
  }
  await navigator.clipboard.writeText(text)
}

function defaultConfirmDelete() {
  return window.confirm('Delete this command and its saved output from history?')
}

function formatTimestamp(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown time'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function formatDuration(entry) {
  const startedAt = new Date(entry.startedAt).getTime()
  const endedAt = new Date(entry.endedAt).getTime()
  if (!Number.isFinite(startedAt) || !Number.isFinite(endedAt)) return ''
  const milliseconds = Math.max(0, endedAt - startedAt)
  if (milliseconds < 1000) return `${milliseconds} ms`
  return `${(milliseconds / 1000).toFixed(milliseconds < 10_000 ? 1 : 0)} s`
}

function entryStatus(entry) {
  if (entry.exitCode === 0 && !entry.error) return { label: 'Completed', className: 'is-success', icon: CheckCircle2 }
  if (entry.error === 'Command stopped.') return { label: 'Stopped', className: 'is-stopped', icon: Square }
  return { label: entry.exitCode >= 0 ? `Exit ${entry.exitCode}` : 'Failed', className: 'is-failure', icon: CircleAlert }
}

function copyableOutput(entry) {
  if (entry.output) return entry.output
  return entry.error || ''
}

export default function CommandApp({
  api = defaultApi,
  copyText = writeClipboard,
  confirmDelete = defaultConfirmDelete,
}) {
  const [server, setServer] = useState(null)
  const [phase, setPhase] = useState('loading')
  const [history, setHistory] = useState([])
  const [selectedID, setSelectedID] = useState('')
  const [command, setCommand] = useState('')
  const [cursor, setCursor] = useState(0)
  const [completionContext, setCompletionContext] = useState(null)
  const [completions, setCompletions] = useState([])
  const [completionIndex, setCompletionIndex] = useState(0)
  const [running, setRunning] = useState(false)
  const [runningCommand, setRunningCommand] = useState('')
  const [stopping, setStopping] = useState(false)
  const [deletingID, setDeletingID] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const inputRef = useRef(null)
  const completionRequestRef = useRef(0)

  const selectedEntry = useMemo(
    () => history.find((entry) => entry.id === selectedID) || history[0] || null,
    [history, selectedID],
  )

  useEffect(() => {
    let active = true
    async function start() {
      setPhase('loading')
      setError('')
      try {
        const state = await api.initialState()
        if (!active) return
        const entries = state.history || []
        setServer(state.server)
        setHistory(entries)
        setSelectedID(entries[0]?.id || '')
        document.title = `${state.server.name} — SSH Man Command`
        setPhase('connecting')
        const result = await api.connect('')
        if (!active) return
        if (result.needsPassphrase) {
          setPhase('locked')
          return
        }
        setPhase('ready')
        requestAnimationFrame(() => inputRef.current?.focus())
      } catch (nextError) {
        if (!active) return
        setError(nextError.message || 'The quick command window could not connect.')
        setPhase('error')
      }
    }
    start()
    return () => { active = false }
  }, [api])

  const connect = useCallback(async (secret = '') => {
    setPhase('connecting')
    setError('')
    try {
      const result = await api.connect(secret)
      if (result.needsPassphrase) {
        setPhase('locked')
        setError(secret ? 'That passphrase did not unlock the SSH key.' : '')
        return false
      }
      setPassphrase('')
      setPhase('ready')
      requestAnimationFrame(() => inputRef.current?.focus())
      return true
    } catch (nextError) {
      setError(nextError.message || 'The SSH connection could not be opened.')
      setPhase(secret ? 'locked' : 'error')
      return false
    }
  }, [api])

  useEffect(() => {
    completionRequestRef.current += 1
    const requestID = completionRequestRef.current
    if (phase !== 'ready' || running || !command.trim() || cursor <= 0) {
      setCompletionContext(null)
      setCompletions([])
      return undefined
    }
    const context = activeShellWord(command, cursor)
    const timer = window.setTimeout(() => {
      api.completePath(context.value)
        .then((result) => {
          if (completionRequestRef.current !== requestID) return
          const items = (result.items || []).filter((item) => item.value !== context.value)
          setCompletionContext(context)
          setCompletions(items)
          setCompletionIndex(0)
        })
        .catch(() => {
          if (completionRequestRef.current === requestID) {
            setCompletionContext(null)
            setCompletions([])
          }
        })
    }, 120)
    return () => window.clearTimeout(timer)
  }, [api, command, cursor, phase, running])

  const applyCompletion = useCallback((item) => {
    if (!item || !completionContext) return
    const next = applyPathCompletion(command, completionContext, item.value)
    setCommand(next.command)
    setCursor(next.cursor)
    setCompletions([])
    setCompletionContext(null)
    requestAnimationFrame(() => {
      inputRef.current?.focus()
      inputRef.current?.setSelectionRange(next.cursor, next.cursor)
    })
  }, [command, completionContext])

  const run = useCallback(async () => {
    const submitted = command
    if (!submitted.trim() || running) return
    completionRequestRef.current += 1
    setCompletions([])
    setCompletionContext(null)
    setRunning(true)
    setRunningCommand(submitted)
    setError('')
    setNotice('')
    try {
      const entry = await api.runCommand(submitted)
      setHistory((current) => [entry, ...current.filter((item) => item.id !== entry.id)])
      setSelectedID(entry.id)
      setCommand('')
      setCursor(0)
      setNotice(entry.exitCode === 0 && !entry.error ? 'Command completed.' : 'Command finished with an error.')
    } catch (nextError) {
      setError(nextError.message || 'The remote command could not be run.')
    } finally {
      setRunning(false)
      setRunningCommand('')
      setStopping(false)
      requestAnimationFrame(() => inputRef.current?.focus())
    }
  }, [api, command, running])

  const stop = useCallback(async () => {
    if (!running || stopping) return
    setStopping(true)
    try {
      await api.cancelCommand()
    } catch (nextError) {
      setError(nextError.message || 'The remote command could not be stopped.')
      setStopping(false)
    }
  }, [api, running, stopping])

  const deleteEntry = useCallback(async (entry) => {
    if (!entry || deletingID || !confirmDelete(entry)) return
    setDeletingID(entry.id)
    setError('')
    try {
      await api.deleteHistory(entry.id)
      setHistory((current) => current.filter((item) => item.id !== entry.id))
      setSelectedID((current) => current === entry.id ? '' : current)
      setNotice('Command history deleted.')
    } catch (nextError) {
      setError(nextError.message || 'The command history could not be deleted.')
    } finally {
      setDeletingID('')
    }
  }, [api, confirmDelete, deletingID])

  const copyOutput = useCallback(async () => {
    if (!selectedEntry) return
    try {
      await copyText(copyableOutput(selectedEntry))
      setNotice('Output copied.')
      setError('')
    } catch (nextError) {
      setError(nextError.message || 'The output could not be copied.')
    }
  }, [copyText, selectedEntry])

  function handleInput(event) {
    setCommand(event.target.value)
    setCursor(event.target.selectionStart ?? event.target.value.length)
    setNotice('')
  }

  function handleSelection(event) {
    setCursor(event.currentTarget.selectionStart ?? command.length)
  }

  function handleKeyDown(event) {
    if (event.key === 'ArrowDown' && completions.length) {
      event.preventDefault()
      setCompletionIndex((index) => (index + 1) % completions.length)
      return
    }
    if (event.key === 'ArrowUp' && completions.length) {
      event.preventDefault()
      setCompletionIndex((index) => (index - 1 + completions.length) % completions.length)
      return
    }
    if (event.key === 'Tab' && completions.length) {
      event.preventDefault()
      applyCompletion(completions[completionIndex])
      return
    }
    if (event.key === 'Escape' && completions.length) {
      event.preventDefault()
      setCompletions([])
      return
    }
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      void run()
    }
  }

  if (phase === 'loading' || phase === 'connecting') {
    return (
      <div className="command-startup" role="status">
        <LoaderCircle className="spin" aria-hidden="true" />
        <strong>{phase === 'loading' ? 'Opening command history…' : 'Connecting to the server…'}</strong>
      </div>
    )
  }

  if (phase === 'error') {
    return (
      <div className="command-startup command-startup--error" role="alert">
        <CircleAlert aria-hidden="true" />
        <strong>Quick command is unavailable</strong>
        <p>{error}</p>
        <button className="primary-button" type="button" onClick={() => connect('')}>Try again</button>
      </div>
    )
  }

  if (phase === 'locked') {
    return (
      <div className="command-startup">
        <KeyRound aria-hidden="true" />
        <span className="eyebrow">Key required</span>
        <h1>Unlock {server?.name || 'server'}</h1>
        <p>The passphrase is used for this command-window connection only and is never saved.</p>
        {error ? <div className="inline-alert inline-alert--warning">{error}</div> : null}
        <form
          className="command-unlock"
          onSubmit={(event) => {
            event.preventDefault()
            if (passphrase.trim()) void connect(passphrase)
          }}
        >
          <label className="field-group" htmlFor="command-passphrase">
            <span>Passphrase</span>
            <input
              id="command-passphrase"
              type="password"
              autoComplete="current-password"
              value={passphrase}
              onChange={(event) => setPassphrase(event.target.value)}
              autoFocus
            />
          </label>
          <button className="primary-button" type="submit" disabled={!passphrase.trim()}>
            <KeyRound aria-hidden="true" /> Unlock and connect
          </button>
        </form>
      </div>
    )
  }

  return (
    <div className="command-window">
      <header className="command-window__header">
        <div className="command-window__identity">
          <span className="command-window__mark" aria-hidden="true"><Terminal /></span>
          <div>
            <span className="eyebrow">Quick command</span>
            <h1>{server?.name || 'Server'}</h1>
            <p>{server ? `${server.username}@${server.host}:${server.port}` : ''}</p>
          </div>
        </div>
        <IconButton label="Close command window" onClick={() => api.close()}><X aria-hidden="true" /></IconButton>
      </header>

      <main className="command-window__layout">
        <aside className="command-history" aria-label="Command history">
          <div className="command-history__heading">
            <div>
              <span className="eyebrow">Saved</span>
              <h2>History</h2>
            </div>
            <span className="command-history__count">{history.length}</span>
          </div>
          {history.length ? (
            <ul className="command-history__list">
              {history.map((entry) => {
                const status = entryStatus(entry)
                const StatusIcon = status.icon
                return (
                  <li key={entry.id} className={entry.id === selectedEntry?.id ? 'is-selected' : ''}>
                    <button type="button" className="command-history__entry" onClick={() => setSelectedID(entry.id)}>
                      <span className={`command-history__status ${status.className}`} aria-label={status.label}><StatusIcon aria-hidden="true" /></span>
                      <span>
                        <strong>{entry.command}</strong>
                        <small>{formatTimestamp(entry.startedAt)}</small>
                      </span>
                    </button>
                    <IconButton
                      label={`Delete ${entry.command} from history`}
                      className="command-history__delete"
                      disabled={deletingID === entry.id}
                      onClick={() => deleteEntry(entry)}
                    >
                      {deletingID === entry.id ? <LoaderCircle className="spin" aria-hidden="true" /> : <Trash2 aria-hidden="true" />}
                    </IconButton>
                  </li>
                )
              })}
            </ul>
          ) : (
            <div className="command-history__empty">
              <Clock aria-hidden="true" />
              <p>Commands and their output will appear here.</p>
            </div>
          )}
        </aside>

        <section className="command-workspace">
          <form
            className="command-composer"
            onSubmit={(event) => {
              event.preventDefault()
              void run()
            }}
          >
            <label htmlFor="quick-command">
              <span>Command</span>
              <small>Enter to run · Shift+Enter for a new line · Tab to complete remote paths</small>
            </label>
            <div className="command-input-wrap">
              <span className="command-prompt" aria-hidden="true">$</span>
              <textarea
                id="quick-command"
                aria-label="Command"
                ref={inputRef}
                rows="2"
                value={command}
                disabled={running}
                spellCheck="false"
                autoCapitalize="off"
                autoCorrect="off"
                placeholder="e.g. tail -n 100 /var/log/app.log"
                onChange={handleInput}
                onSelect={handleSelection}
                onClick={handleSelection}
                onKeyUp={handleSelection}
                onKeyDown={handleKeyDown}
                onBlur={() => window.setTimeout(() => setCompletions([]), 100)}
              />
            </div>
            {completions.length ? (
              <ul className="command-completions" role="listbox" aria-label="Remote file suggestions">
                {completions.slice(0, 8).map((item, index) => {
                  const CompletionIcon = item.kind === 'directory' ? Folder : File
                  return (
                    <li
                      key={`${item.kind}:${item.value}`}
                      role="option"
                      aria-selected={index === completionIndex}
                    >
                      <button
                        type="button"
                        className={index === completionIndex ? 'is-selected' : ''}
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => applyCompletion(item)}
                      >
                        <CompletionIcon aria-hidden="true" />
                        <span>{item.name}</span>
                        <small>{item.value}</small>
                      </button>
                    </li>
                  )
                })}
              </ul>
            ) : null}
            <div className="command-composer__actions">
              <span>{completions.length ? `${completions.length} remote path${completions.length === 1 ? '' : 's'} found` : 'Remote filesystem autocomplete is active'}</span>
              {running ? (
                <button className="danger-button" type="button" disabled={stopping} onClick={stop}>
                  {stopping ? <LoaderCircle className="spin" aria-hidden="true" /> : <Square aria-hidden="true" />}
                  {stopping ? 'Stopping…' : 'Stop'}
                </button>
              ) : (
                <button className="primary-button" type="submit" disabled={!command.trim()}>
                  <Play aria-hidden="true" /> Run command
                </button>
              )}
            </div>
          </form>

          {error ? <div className="inline-alert inline-alert--danger command-notice"><CircleAlert aria-hidden="true" /><span>{error}</span></div> : null}
          {notice ? <div className="command-notice command-notice--success" role="status"><CheckCircle2 aria-hidden="true" /><span>{notice}</span></div> : null}

          <section className="command-output" aria-label="Command output">
            {running ? (
              <>
                <div className="command-output__heading">
                  <div>
                    <span className="eyebrow">Running</span>
                    <h2><code>{runningCommand}</code></h2>
                  </div>
                  <LoaderCircle className="spin" aria-label="Command is running" />
                </div>
                <div className="command-output__waiting">
                  <Terminal aria-hidden="true" />
                  <p>Waiting for the remote command to finish…</p>
                </div>
              </>
            ) : selectedEntry ? (
              <>
                <div className="command-output__heading">
                  <div>
                    <span className="eyebrow">Output</span>
                    <h2><code>{selectedEntry.command}</code></h2>
                    <p>{formatTimestamp(selectedEntry.startedAt)}{formatDuration(selectedEntry) ? ` · ${formatDuration(selectedEntry)}` : ''}</p>
                  </div>
                  <div className="command-output__actions">
                    <IconButton label="Copy command output" onClick={copyOutput}><Copy aria-hidden="true" /></IconButton>
                    <IconButton label="Delete selected command history" onClick={() => deleteEntry(selectedEntry)}><Trash2 aria-hidden="true" /></IconButton>
                  </div>
                </div>
                <div className={`command-output__status ${entryStatus(selectedEntry).className}`}>
                  {entryStatus(selectedEntry).label}
                  {selectedEntry.truncated ? <span>Output capped at 2 MB</span> : null}
                </div>
                <pre tabIndex="0">{selectedEntry.output || selectedEntry.error || '(No output)'}</pre>
                {selectedEntry.error && selectedEntry.output ? <div className="command-output__error">{selectedEntry.error}</div> : null}
              </>
            ) : (
              <div className="command-output__empty">
                <Terminal aria-hidden="true" />
                <h2>Ready for a command</h2>
                <p>Run a one-off command above. Its prompt, output, status, and timing will be saved here.</p>
              </div>
            )}
          </section>
        </section>
      </main>
    </div>
  )
}
