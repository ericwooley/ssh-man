import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowDownToLine,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  ChevronRight,
  CircleAlert,
  Eye,
  File,
  FileCode2,
  FileText,
  Folder,
  FolderOpen,
  Home,
  LoaderCircle,
  Monitor,
  RefreshCw,
  Save,
  Server,
  Star,
  X,
} from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import * as defaultApi from '../lib/explorerApi'
import { remoteContentURL } from '../lib/explorerApi'
import MonacoPreview from './MonacoPreview'
import {
  addFavorite,
  readFavoritePaths,
  readVimMode,
  removeFavorite,
  writeFavoritePaths,
  writeVimMode,
} from './explorerPreferences'
import {
  editorLanguage,
  formatFileSize,
  parentRemotePath,
  resolveRemoteReference,
} from './pathUtils'

function itemIcon(entry) {
  if (entry.kind === 'directory') return Folder
  if (/\.(md|markdown)$/i.test(entry.name)) return FileText
  if (/\.(html?|css|js|jsx|ts|tsx|go|py|rs|json|ya?ml)$/i.test(entry.name)) return FileCode2
  return File
}

function formatDate(value) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function previewTitle(preview) {
  if (!preview) return 'Preview'
  if (preview.kind === 'markdown') return 'Markdown preview'
  if (preview.kind === 'browser') return 'Browser preview'
  if (preview.kind === 'image') return 'Image preview'
  return 'Source preview'
}

function favoriteName(remotePath) {
  return remotePath.split('/').filter(Boolean).at(-1) || '/'
}

export default function ExplorerApp({ api = defaultApi }) {
  const storage = window.localStorage
  const [server, setServer] = useState(null)
  const [phase, setPhase] = useState('connecting')
  const [homePath, setHomePath] = useState('')
  const [directory, setDirectory] = useState({ path: '', entries: [] })
  const [pathInput, setPathInput] = useState('')
  const [selectedPaths, setSelectedPaths] = useState([])
  const [preview, setPreview] = useState(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewMode, setPreviewMode] = useState('rendered')
  const [draftContent, setDraftContent] = useState('')
  const [dirty, setDirty] = useState(false)
  const [saving, setSaving] = useState(false)
  const [vimMode, setVimMode] = useState(() => readVimMode(storage))
  const [favoritePaths, setFavoritePaths] = useState([])
  const [error, setError] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [downloading, setDownloading] = useState(false)
  const [notice, setNotice] = useState('')
  const [historyVersion, setHistoryVersion] = useState(0)
  const historyRef = useRef([])
  const historyIndexRef = useRef(-1)
  const startedRef = useRef(false)

  const confirmDiscard = useCallback(() => {
    if (!dirty) return true
    return window.confirm('Discard the unsaved changes to this remote file?')
  }, [dirty])

  const commitHistory = useCallback((remotePath, mode) => {
    if (mode === 'history') return
    if (mode === 'replace') {
      historyRef.current = [remotePath]
      historyIndexRef.current = 0
    } else {
      historyRef.current = historyRef.current.slice(0, historyIndexRef.current + 1).concat(remotePath)
      historyIndexRef.current = historyRef.current.length - 1
    }
    setHistoryVersion((value) => value + 1)
  }, [])

  const openDirectory = useCallback(async (remotePath, mode = 'push') => {
    if (!confirmDiscard()) return null
    setError('')
    setNotice('')
    setPhase('loading')
    try {
      const next = await api.listDirectory(remotePath)
      setDirectory(next)
      setPathInput(next.path)
      setSelectedPaths([])
      setPreview(null)
      setDraftContent('')
      setDirty(false)
      setPhase('ready')
      commitHistory(next.path, mode)
      if (server?.id) storage.setItem(`ssh-man:explorer:path:${server.id}`, next.path)
      return next
    } catch (nextError) {
      setError(nextError.message || 'The remote folder could not be opened.')
      setPhase('error')
      return null
    }
  }, [api, commitHistory, confirmDiscard, server?.id, storage])

  const connect = useCallback(async (secret = '') => {
    setError('')
    setPhase('connecting')
    try {
      const result = await api.connect(secret)
      if (result.needsPassphrase) {
        setPhase('locked')
        return
      }
      setHomePath(result.homePath)
      setPassphrase('')
      const savedPath = server?.id ? storage.getItem(`ssh-man:explorer:path:${server.id}`) : ''
      const opened = await openDirectory(savedPath || result.homePath, 'replace')
      if (!opened && savedPath && savedPath !== result.homePath) {
        await openDirectory(result.homePath, 'replace')
      }
    } catch (nextError) {
      setError(nextError.message || 'The SSH connection could not be opened.')
      setPhase('error')
    }
  }, [api, openDirectory, server?.id, storage])

  useEffect(() => {
    if (startedRef.current) return
    startedRef.current = true
    let active = true
    async function start() {
      try {
        const state = await api.initialState()
        if (!active) return
        setServer(state.server)
        setFavoritePaths(readFavoritePaths(storage, state.server.id))
        document.title = `${state.server.name} — SSH Man Explorer`
        const result = await api.connect('')
        if (!active) return
        if (result.needsPassphrase) {
          setPhase('locked')
          return
        }
        setHomePath(result.homePath)
        const savedPath = storage.getItem(`ssh-man:explorer:path:${state.server.id}`)
        let next
        try {
          next = await api.listDirectory(savedPath || result.homePath)
        } catch (nextError) {
          if (!savedPath || savedPath === result.homePath) throw nextError
          next = await api.listDirectory(result.homePath)
        }
        if (!active) return
        setDirectory(next)
        setPathInput(next.path)
        setPhase('ready')
        commitHistory(next.path, 'replace')
      } catch (nextError) {
        if (!active) return
        setError(nextError.message || 'The server explorer could not connect.')
        setPhase('error')
      }
    }
    start()
    return () => { active = false }
  }, [api, commitHistory, storage])

  const selectedEntries = useMemo(() => directory.entries.filter((entry) => selectedPaths.includes(entry.path)), [directory.entries, selectedPaths])
  const selectedEntry = selectedEntries.length === 1 ? selectedEntries[0] : null
  const editablePreview = Boolean(preview?.revision && preview?.content != null && !preview.truncated)

  const saveCurrentFile = useCallback(async (contentOverride) => {
    if (!editablePreview || saving) return
    const nextContent = typeof contentOverride === 'string' ? contentOverride : draftContent
    if (nextContent === preview.content) {
      setDirty(false)
      return
    }
    setSaving(true)
    setError('')
    setNotice('')
    try {
      const saved = await api.saveFile(preview.path, nextContent, preview.revision)
      setPreview(saved)
      setDraftContent(saved.content ?? nextContent)
      setDirty(false)
      setNotice(`Saved ${saved.name}.`)
    } catch (nextError) {
      setError(nextError.message || 'The remote file could not be saved.')
    } finally {
      setSaving(false)
    }
  }, [api, draftContent, editablePreview, preview, saving])

  useEffect(() => {
    if (!dirty) return undefined
    const handleBeforeUnload = (event) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [dirty])

  useEffect(() => {
    let active = true
    setPreview(null)
    setDraftContent('')
    setDirty(false)
    if (!selectedEntry || selectedEntry.kind === 'directory') return () => { active = false }
    setPreviewLoading(true)
    api.previewFile(selectedEntry.path)
      .then((next) => {
        if (!active) return
        setPreview(next)
        setDraftContent(next.content ?? '')
        setDirty(false)
        setPreviewMode(next.kind === 'text' ? 'source' : 'rendered')
      })
      .catch((nextError) => {
        if (active) setError(nextError.message || 'The file preview could not be loaded.')
      })
      .finally(() => {
        if (active) setPreviewLoading(false)
      })
    return () => { active = false }
  }, [api, selectedEntry])

  function selectEntry(event, entry) {
    if (!selectedPaths.includes(entry.path) && !confirmDiscard()) return
    if (event.metaKey || event.ctrlKey) {
      setSelectedPaths((current) => current.includes(entry.path)
        ? current.filter((item) => item !== entry.path)
        : current.concat(entry.path))
    } else {
      setSelectedPaths([entry.path])
    }
  }

  async function navigateHistory(direction) {
    const nextIndex = historyIndexRef.current + direction
    const remotePath = historyRef.current[nextIndex]
    if (!remotePath) return
    historyIndexRef.current = nextIndex
    setHistoryVersion((value) => value + 1)
    await openDirectory(remotePath, 'history')
  }

  async function downloadSelection() {
    if (!selectedPaths.length) return
    setDownloading(true)
    setError('')
    try {
      const paths = await api.download(selectedPaths)
      if (paths?.length) setNotice(`Downloaded ${paths.length} item${paths.length === 1 ? '' : 's'} to ${parentRemotePath(paths[0])}.`)
    } catch (nextError) {
      setError(nextError.message || 'The selected items could not be downloaded.')
    } finally {
      setDownloading(false)
    }
  }

  function toggleVimMode(event) {
    const enabled = event.target.checked
    setVimMode(enabled)
    writeVimMode(storage, enabled)
  }

  function toggleCurrentFavorite() {
    if (!server?.id || !directory.path) return
    const next = favoritePaths.includes(directory.path)
      ? removeFavorite(favoritePaths, directory.path)
      : addFavorite(favoritePaths, directory.path)
    setFavoritePaths(writeFavoritePaths(storage, server.id, next))
  }

  function removeFavoritePath(remotePath) {
    if (!server?.id) return
    setFavoritePaths(writeFavoritePaths(storage, server.id, removeFavorite(favoritePaths, remotePath)))
  }

  const canGoBack = historyVersion >= 0 && historyIndexRef.current > 0
  const canGoForward = historyIndexRef.current >= 0 && historyIndexRef.current < historyRef.current.length - 1
  const currentFolderIsFavorite = favoritePaths.includes(directory.path)
  const sourceEditorVisible = preview?.kind === 'text' || (previewMode === 'source' && ['markdown', 'browser'].includes(preview?.kind))

  return (
    <div className="explorer-shell">
      <header className="explorer-toolbar">
        <div className="explorer-toolbar__nav" aria-label="Folder navigation">
          <button type="button" aria-label="Back" disabled={!canGoBack} onClick={() => navigateHistory(-1)}><ArrowLeft /></button>
          <button type="button" aria-label="Forward" disabled={!canGoForward} onClick={() => navigateHistory(1)}><ArrowRight /></button>
          <button type="button" aria-label="Up one folder" disabled={!directory.path || directory.path === '/'} onClick={() => openDirectory(parentRemotePath(directory.path))}><ArrowUp /></button>
          <button type="button" aria-label="Refresh folder" disabled={phase !== 'ready'} onClick={() => openDirectory(directory.path, 'history')}><RefreshCw /></button>
        </div>
        <form className="explorer-path" onSubmit={(event) => { event.preventDefault(); openDirectory(pathInput) }}>
          <Server aria-hidden="true" />
          <input aria-label="Remote path" value={pathInput} onChange={(event) => setPathInput(event.target.value)} spellCheck="false" />
        </form>
        <button className="explorer-download" type="button" disabled={!selectedPaths.length || downloading} onClick={downloadSelection}>
          {downloading ? <LoaderCircle className="spin" /> : <ArrowDownToLine />}
          Download{selectedPaths.length > 1 ? ` (${selectedPaths.length})` : ''}
        </button>
      </header>

      <div className="explorer-workspace">
        <aside className="explorer-sidebar">
          <div className="explorer-server">
            <span>{server?.name?.slice(0, 2).toUpperCase() || 'SH'}</span>
            <div><strong>{server?.name || 'Server'}</strong><small>{server ? `${server.username}@${server.host}` : 'Connecting…'}</small></div>
          </div>
          <nav aria-label="Remote locations">
            <span>Locations</span>
            <button type="button" className={directory.path === homePath ? 'is-current' : ''} disabled={!homePath} onClick={() => openDirectory(homePath)}><Home /> Home</button>
            <button type="button" className={directory.path === '/' ? 'is-current' : ''} onClick={() => openDirectory('/')}><Monitor /> File system</button>
          </nav>
          {favoritePaths.length ? (
            <nav className="explorer-favorites" aria-label="Favorite folders">
              <span>Favorites</span>
              {favoritePaths.map((remotePath) => (
                <div className="explorer-favorite" key={remotePath}>
                  <button type="button" className={directory.path === remotePath ? 'is-current' : ''} aria-label={`Open favorite ${favoriteName(remotePath)}`} title={remotePath} onClick={() => openDirectory(remotePath)}><Star /> <span>{favoriteName(remotePath)}</span></button>
                  <button type="button" className="explorer-favorite__remove" aria-label={`Remove favorite ${favoriteName(remotePath)}`} onClick={() => removeFavoritePath(remotePath)}><X /></button>
                </div>
              ))}
            </nav>
          ) : null}
          <div className="explorer-connection"><span className={phase === 'ready' ? 'is-online' : ''} />{phase === 'ready' ? 'Connected over SFTP' : 'SSH connection'}</div>
        </aside>

        <main className="explorer-browser" aria-busy={phase === 'loading' || phase === 'connecting'}>
          <div className="explorer-location-title">
            <FolderOpen />
            <strong>{directory.path ? directory.path.split('/').filter(Boolean).at(-1) || '/' : 'Files'}</strong>
            <span>{directory.entries.length} items</span>
            <button
              type="button"
              className={currentFolderIsFavorite ? 'is-favorite' : ''}
              aria-label={currentFolderIsFavorite ? 'Remove current folder from favorites' : 'Favorite current folder'}
              title={currentFolderIsFavorite ? 'Remove from favorites' : 'Add to favorites'}
              disabled={!directory.path || phase !== 'ready'}
              onClick={toggleCurrentFavorite}
            ><Star fill={currentFolderIsFavorite ? 'currentColor' : 'none'} /></button>
          </div>
          <div className="explorer-columns" aria-hidden="true"><span>Name</span><span>Size</span><span>Modified</span></div>
          {phase === 'loading' || phase === 'connecting' ? <div className="explorer-state"><LoaderCircle className="spin" /><span>{phase === 'connecting' ? 'Connecting over SSH…' : 'Opening folder…'}</span></div> : null}
          {phase === 'error' ? (
            <div className="explorer-state is-error"><CircleAlert /><strong>Couldn’t open this location</strong><span>{error}</span><button type="button" onClick={() => connect('')}>Reconnect</button></div>
          ) : null}
          {phase === 'ready' && !directory.entries.length ? <div className="explorer-state"><Folder /><strong>This folder is empty</strong></div> : null}
          {phase === 'ready' && directory.entries.length ? (
            <div className="explorer-list" role="listbox" aria-label={`Files in ${directory.path}`} aria-multiselectable="true">
              {directory.entries.map((entry) => {
                const Icon = itemIcon(entry)
                const selected = selectedPaths.includes(entry.path)
                return (
                  <button
                    type="button"
                    role="option"
                    aria-selected={selected}
                    className={`explorer-row${selected ? ' is-selected' : ''}`}
                    key={entry.path}
                    onClick={(event) => selectEntry(event, entry)}
                    onDoubleClick={() => entry.kind === 'directory' && openDirectory(entry.path)}
                  >
                    <span className={`explorer-file-icon is-${entry.kind}`}><Icon /></span>
                    <span className="explorer-file-name">{entry.name}{entry.kind === 'directory' ? <ChevronRight /> : null}</span>
                    <span>{entry.kind === 'directory' ? '—' : formatFileSize(entry.size)}</span>
                    <span>{formatDate(entry.modifiedAt)}</span>
                  </button>
                )
              })}
            </div>
          ) : null}
        </main>

        <aside className="explorer-preview">
          <div className="explorer-preview__header">
            <div><Eye /><strong>{previewTitle(preview)}</strong>{dirty ? <span className="explorer-edited">Edited</span> : null}</div>
            <div className="explorer-preview-actions">
              {preview?.kind === 'browser' && previewMode === 'rendered' ? <span className="explorer-preview-safety">Scripts disabled</span> : null}
              {sourceEditorVisible && editablePreview ? <label className="explorer-vim-toggle"><input type="checkbox" checked={vimMode} onChange={toggleVimMode} />Vim controls</label> : null}
              {preview?.content != null && ['markdown', 'browser'].includes(preview.kind) ? (
                <div className="explorer-preview-toggle">
                  <button type="button" className={previewMode === 'rendered' ? 'is-current' : ''} onClick={() => setPreviewMode('rendered')}>Rendered</button>
                  <button type="button" className={previewMode === 'source' ? 'is-current' : ''} onClick={() => setPreviewMode('source')}>Source</button>
                </div>
              ) : null}
              {editablePreview ? <button type="button" className="explorer-save" aria-label="Save" title="Save (Command/Ctrl+S or :w in Vim mode)" disabled={!dirty || saving} onClick={() => saveCurrentFile()}>{saving ? <LoaderCircle className="spin" /> : <Save />}<span>{saving ? 'Saving' : 'Save'}</span></button> : null}
            </div>
          </div>
          {previewLoading ? <div className="explorer-preview-empty"><LoaderCircle className="spin" /> Loading preview…</div> : null}
          {!previewLoading && !selectedEntry ? <div className="explorer-preview-empty"><File /><span>Select one file to preview it.</span></div> : null}
          {!previewLoading && selectedEntry?.kind === 'directory' ? <div className="explorer-preview-empty"><Folder /><strong>{selectedEntry.name}</strong><span>Double-click to open this folder.</span></div> : null}
          {!previewLoading && preview ? (
            <div className="explorer-preview__body">
              {preview.truncated ? <div className="explorer-preview-note">Preview limited to the first 2 MB. Download for the full file.</div> : null}
              {previewMode === 'source' && preview.content != null && ['markdown', 'browser'].includes(preview.kind) ? <MonacoPreview content={draftContent} language={editorLanguage(preview.name)} label={`${preview.name} source`} modelKey={preview.path} vimMode={vimMode} onChange={(content) => { setDraftContent(content); setDirty(content !== preview.content) }} onSave={saveCurrentFile} /> : null}
              {previewMode === 'rendered' && preview.kind === 'markdown' ? (
                <article className="explorer-markdown">
                  <ReactMarkdown
                    remarkPlugins={[remarkGfm]}
                    urlTransform={(url) => resolveRemoteReference(preview.path, url)}
                    components={{ a: ({ node: _node, ...props }) => <a {...props} target="_blank" rel="noreferrer" /> }}
                  >{draftContent}</ReactMarkdown>
                </article>
              ) : null}
              {previewMode === 'rendered' && preview.kind === 'browser' ? <iframe key={preview.revision} title={`${preview.name} browser preview`} src={remoteContentURL(preview.path)} sandbox="allow-forms" /> : null}
              {previewMode === 'rendered' && preview.kind === 'image' ? <div className="explorer-image"><img src={remoteContentURL(preview.path)} alt={preview.name} /></div> : null}
              {previewMode === 'rendered' && preview.kind === 'unsupported' ? <div className="explorer-preview-empty"><File /><strong>No preview available</strong><span>{formatFileSize(preview.size)} · Download to open locally.</span></div> : null}
              {preview.kind === 'text' ? <MonacoPreview content={draftContent} language={editorLanguage(preview.name)} label={`${preview.name} source`} modelKey={preview.path} vimMode={vimMode} onChange={(content) => { setDraftContent(content); setDirty(content !== preview.content) }} onSave={saveCurrentFile} /> : null}
            </div>
          ) : null}
        </aside>
      </div>

      {phase === 'locked' ? (
        <div className="explorer-unlock-backdrop">
          <form className="explorer-unlock" onSubmit={(event) => { event.preventDefault(); connect(passphrase) }}>
            <div className="explorer-unlock__icon"><Server /></div>
            <span>Encrypted SSH key</span>
            <h1>Unlock {server?.name || 'server'}</h1>
            <p>Enter the passphrase for the saved private key. It stays in this explorer process only.</p>
            <label>Key passphrase<input autoFocus type="password" value={passphrase} onChange={(event) => setPassphrase(event.target.value)} /></label>
            <button type="submit" disabled={!passphrase}>Connect and explore</button>
          </form>
        </div>
      ) : null}

      {error && phase !== 'error' ? <div className="explorer-toast is-error" role="alert">{error}</div> : null}
      {notice ? <div className="explorer-toast" role="status">{notice}</div> : null}
    </div>
  )
}
