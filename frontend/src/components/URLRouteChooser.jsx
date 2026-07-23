import { useEffect, useRef, useState } from 'react'
import { ArrowRight, LoaderCircle, Network, X } from 'lucide-react'
import { IconButton } from './AppChrome'

export function URLRouteChooser({ request, onChoose, onDismiss }) {
  const firstChoice = useRef(null)
  const [openingID, setOpeningID] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    firstChoice.current?.focus()
  }, [request.id])

  useEffect(() => {
    function handleKeyDown(event) {
      if (event.key !== 'Escape' || openingID) return
      event.preventDefault()
      onDismiss()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onDismiss, openingID])

  async function choose(choice) {
    if (openingID) return
    setOpeningID(choice.id)
    setError('')
    try {
      await onChoose(choice.id)
    } catch (nextError) {
      setError(nextError.message || 'The URL could not be opened through that server.')
      setOpeningID('')
    }
  }

  return (
    <div className="url-route-chooser" role="dialog" aria-modal="true" aria-labelledby="url-route-title">
      <div className="url-route-chooser__header">
        <div>
          <span className="eyebrow">Localhost route</span>
          <h1 id="url-route-title">Choose a server for this URL</h1>
        </div>
        <IconButton label="Cancel URL routing" disabled={Boolean(openingID)} onClick={onDismiss}>
          <X aria-hidden="true" />
        </IconButton>
      </div>
      <code className="url-route-chooser__url">{request.url}</code>
      <p>More than one connected server is accepting this port. Pick the SOCKS proxy to use.</p>

      {error ? <div className="inline-alert inline-alert--danger" role="alert">{error}</div> : null}

      <div className="url-route-choice-list">
        {request.choices.map((choice, index) => (
          <button
            key={choice.id}
            ref={index === 0 ? firstChoice : undefined}
            type="button"
            disabled={Boolean(openingID)}
            aria-label={`Open through ${choice.serverName}`}
            onClick={() => choose(choice)}
          >
            <span className="url-route-choice-list__icon"><Network aria-hidden="true" /></span>
            <span><strong>{choice.serverName}</strong><small>SOCKS5 proxy</small></span>
            {openingID === choice.id ? <LoaderCircle className="spin" aria-hidden="true" /> : <ArrowRight aria-hidden="true" />}
          </button>
        ))}
      </div>
    </div>
  )
}
