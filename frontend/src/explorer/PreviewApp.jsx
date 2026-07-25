import { useCallback, useEffect, useRef, useState } from 'react'
import {
  CircleAlert,
  Eye,
  LoaderCircle,
  Maximize2,
  Minimize2,
  RefreshCw,
  Server,
} from 'lucide-react'
import * as defaultApi from '../lib/previewApi'
import PreviewContent from './PreviewContent'

export default function PreviewApp({ api = defaultApi }) {
  const [initialState, setInitialState] = useState(null)
  const [phase, setPhase] = useState('connecting')
  const [preview, setPreview] = useState(null)
  const [previewMode, setPreviewMode] = useState('rendered')
  const [contentVersion, setContentVersion] = useState(0)
  const [passphrase, setPassphrase] = useState('')
  const [error, setError] = useState('')
  const [fullscreen, setFullscreen] = useState(false)
  const startedRef = useRef(false)

  const loadPreview = useCallback(async () => {
    setError('')
    setPhase('loading')
    try {
      const next = await api.previewFile()
      setPreview(next)
      setContentVersion((version) => version + 1)
      setPreviewMode((current) => {
        if (next.kind === 'text') return 'source'
        if (current === 'source' && ['markdown', 'browser'].includes(next.kind)) return 'source'
        return 'rendered'
      })
      setPhase('ready')
    } catch (nextError) {
      setError(nextError.message || 'The remote file preview could not be loaded.')
      setPhase('error')
    }
  }, [api])

  const connect = useCallback(async (secret = '') => {
    setError('')
    setPhase('connecting')
    try {
      const result = await api.connect(secret)
      if (result.needsPassphrase) {
        setPhase('locked')
        return
      }
      setPassphrase('')
      await loadPreview()
    } catch (nextError) {
      setError(nextError.message || 'The SSH connection could not be opened.')
      setPhase('error')
    }
  }, [api, loadPreview])

  useEffect(() => {
    if (startedRef.current) return
    startedRef.current = true
    let active = true
    async function start() {
      try {
        const state = await api.initialState()
        if (!active) return
        setInitialState(state)
        const fileName = state.remotePath.split('/').filter(Boolean).at(-1) || state.remotePath
        document.title = `${fileName} — ${state.server.name} — SSH Man Preview`
        await connect('')
      } catch (nextError) {
        if (!active) return
        setError(nextError.message || 'The preview window could not start.')
        setPhase('error')
      }
    }
    start()
    return () => { active = false }
  }, [api, connect])

  useEffect(() => {
    if (!api.fullscreenState) return undefined
    let active = true
    let resizeTimer
    const syncFullscreen = () => {
      api.fullscreenState()
        .then((next) => {
          if (active) setFullscreen(Boolean(next))
        })
        .catch(() => {})
    }
    const scheduleFullscreenSync = () => {
      window.clearTimeout(resizeTimer)
      resizeTimer = window.setTimeout(syncFullscreen, 100)
    }
    syncFullscreen()
    window.addEventListener('focus', syncFullscreen)
    window.addEventListener('resize', scheduleFullscreenSync)
    return () => {
      active = false
      window.clearTimeout(resizeTimer)
      window.removeEventListener('focus', syncFullscreen)
      window.removeEventListener('resize', scheduleFullscreenSync)
    }
  }, [api])

  async function toggleFullscreen() {
    setError('')
    try {
      setFullscreen(await api.toggleFullscreen())
    } catch (nextError) {
      setError(nextError.message || 'Fullscreen could not be changed.')
    }
  }

  const supportsModes = preview?.content != null && ['markdown', 'browser'].includes(preview.kind)
  const showsReadOnlySource = preview?.content != null && ['text', 'markdown', 'browser'].includes(preview.kind)

  return (
    <div className="preview-window-shell">
      <header className="preview-window__header">
        <div className="preview-window__identity">
          <Eye />
          <div>
            <strong>{preview?.name || initialState?.remotePath?.split('/').filter(Boolean).at(-1) || 'Remote preview'}</strong>
            <span>{initialState ? `${initialState.server.name} · ${initialState.remotePath}` : 'Connecting to server…'}</span>
          </div>
        </div>
        <div className="preview-window__actions">
          {showsReadOnlySource ? <span className="preview-window__readonly">Read only</span> : null}
          {preview?.kind === 'browser' && previewMode === 'rendered' ? <span className="explorer-preview-safety">Scripts disabled</span> : null}
          {supportsModes ? (
            <div className="explorer-preview-toggle">
              <button type="button" className={previewMode === 'rendered' ? 'is-current' : ''} onClick={() => setPreviewMode('rendered')}>Rendered</button>
              <button type="button" className={previewMode === 'source' ? 'is-current' : ''} onClick={() => setPreviewMode('source')}>Source</button>
            </div>
          ) : null}
          <button type="button" aria-label="Reload preview" title="Reload preview" disabled={phase !== 'ready'} onClick={loadPreview}><RefreshCw /></button>
          <button
            type="button"
            aria-label={fullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
            title={fullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
            onClick={toggleFullscreen}
          >{fullscreen ? <Minimize2 /> : <Maximize2 />}</button>
        </div>
      </header>

      <main className="preview-window__content">
        {phase === 'connecting' || phase === 'loading' ? <div className="explorer-preview-empty"><LoaderCircle className="spin" /><span>{phase === 'connecting' ? 'Connecting over SSH…' : 'Loading preview…'}</span></div> : null}
        {phase === 'error' ? <div className="explorer-state is-error"><CircleAlert /><strong>Couldn’t open this preview</strong><span>{error}</span><button type="button" onClick={() => connect('')}>Retry</button></div> : null}
        {phase === 'ready' && preview ? <PreviewContent content={preview.content ?? ''} contentKey={contentVersion} mode={previewMode} preview={preview} readOnly /> : null}
      </main>

      {phase === 'locked' ? (
        <div className="explorer-unlock-backdrop">
          <form className="explorer-unlock" onSubmit={(event) => { event.preventDefault(); connect(passphrase) }}>
            <div className="explorer-unlock__icon"><Server /></div>
            <span>Encrypted SSH key</span>
            <h1>Unlock {initialState?.server.name || 'server'}</h1>
            <p>Enter the passphrase for the saved private key. It stays in this preview process only.</p>
            <label>Key passphrase<input autoFocus type="password" value={passphrase} onChange={(event) => setPassphrase(event.target.value)} /></label>
            <button type="submit" disabled={!passphrase}>Connect and preview</button>
          </form>
        </div>
      ) : null}

      {error && phase !== 'error' ? <div className="explorer-toast is-error" role="alert">{error}</div> : null}
    </div>
  )
}
