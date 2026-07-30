import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowRight, Globe2, LoaderCircle, Network, TerminalSquare, X } from 'lucide-react'
import { browserAppearanceForTarget } from '../model/appModel'
import { IconButton } from './AppChrome'
import { BrowserAppearanceMark, browserAppearanceStyle } from './BrowserAppearance'

const fallbackTimeoutMilliseconds = 5000
const timerTickMilliseconds = 100

function choiceIcon(choice) {
  if (choice.kind === 'proxy') return Network
  if (choice.kind === 'command') return TerminalSquare
  return Globe2
}

export function URLRouteChooser({ request, appearances = {}, onChoose, onDismiss }) {
  const dialogRef = useRef(null)
  const autoOpenedRequestRef = useRef('')
  const timeoutMilliseconds = Math.max(Number(request.timeoutMilliseconds) || fallbackTimeoutMilliseconds, 0)
  const defaultChoice = request.choices.find((choice) => choice.id === request.defaultChoiceId) || request.choices[0]
  const [selectedID, setSelectedID] = useState(defaultChoice?.id || '')
  const [remainingMilliseconds, setRemainingMilliseconds] = useState(timeoutMilliseconds)
  const [paused, setPaused] = useState(false)
  const [openingID, setOpeningID] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    const nextDefault = request.choices.find((choice) => choice.id === request.defaultChoiceId) || request.choices[0]
    setSelectedID(nextDefault?.id || '')
    setRemainingMilliseconds(Math.max(Number(request.timeoutMilliseconds) || fallbackTimeoutMilliseconds, 0))
    setPaused(false)
    setOpeningID('')
    setError('')
    autoOpenedRequestRef.current = ''
    dialogRef.current?.focus({ preventScroll: true })
  }, [request])

  useEffect(() => {
    if (paused || openingID || remainingMilliseconds <= 0) return undefined
    const timer = window.setInterval(() => {
      setRemainingMilliseconds((current) => Math.max(0, current - timerTickMilliseconds))
    }, timerTickMilliseconds)
    return () => window.clearInterval(timer)
  }, [openingID, paused, remainingMilliseconds])

  const choose = useCallback(async (choiceID) => {
    if (!choiceID || openingID) return
    setOpeningID(choiceID)
    setError('')
    try {
      await onChoose(choiceID)
    } catch (nextError) {
      setError(nextError.message || 'The URL could not be opened in that destination.')
      setOpeningID('')
      setPaused(true)
    }
  }, [onChoose, openingID])

  useEffect(() => {
    if (
      paused ||
      openingID ||
      remainingMilliseconds > 0 ||
      !defaultChoice?.id ||
      autoOpenedRequestRef.current === request.id
    ) {
      return
    }
    autoOpenedRequestRef.current = request.id
    void choose(defaultChoice.id)
  }, [choose, defaultChoice?.id, openingID, paused, remainingMilliseconds, request.id])

  const selectedIndex = useMemo(
    () => Math.max(request.choices.findIndex((choice) => choice.id === selectedID), 0),
    [request.choices, selectedID],
  )

  function handleKeyDown(event) {
    if (openingID) return
    setPaused(true)
    if (event.key === 'Escape') {
      event.preventDefault()
      onDismiss()
      return
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const offset = event.key === 'ArrowDown' ? 1 : -1
      const nextIndex = (selectedIndex + offset + request.choices.length) % request.choices.length
      setSelectedID(request.choices[nextIndex]?.id || '')
      return
    }
    if (event.key === 'Enter') {
      event.preventDefault()
      void choose(selectedID)
    }
  }

  const countdownSeconds = Math.max(1, Math.ceil(remainingMilliseconds / 1000))
  const progress = timeoutMilliseconds > 0
    ? Math.max(0, Math.min(1, remainingMilliseconds / timeoutMilliseconds))
    : 0

  return (
    <div
      ref={dialogRef}
      className="url-route-chooser"
      role="dialog"
      aria-modal="true"
      aria-labelledby="url-route-title"
      tabIndex={-1}
      onKeyDown={handleKeyDown}
      onPointerDown={() => setPaused(true)}
      onWheel={() => setPaused(true)}
      onTouchStart={() => setPaused(true)}
    >
      <div className="url-route-chooser__header">
        <div>
          <span className="eyebrow">Open link</span>
          <h1 id="url-route-title">Choose where to open this link</h1>
        </div>
        <IconButton label="Cancel URL routing" disabled={Boolean(openingID)} onClick={onDismiss}>
          <X aria-hidden="true" />
        </IconButton>
      </div>
      <code className="url-route-chooser__url" title={request.url}>{request.url}</code>

      <div className={`url-route-countdown ${paused ? 'is-paused' : ''}`} role="status" aria-live="polite">
        <span>
          {paused
            ? 'Timer paused'
            : `Opening ${defaultChoice?.label || 'the default'} in ${countdownSeconds}s`}
        </span>
        <span className="url-route-countdown__track" aria-hidden="true">
          <span style={{ transform: `scaleX(${progress})` }} />
        </span>
      </div>

      <div className="url-route-error-slot">
        {error ? <div className="inline-alert inline-alert--danger" role="alert">{error}</div> : null}
      </div>

      <div className="url-route-choice-list" role="listbox" aria-label="Link destination">
        {request.choices.map((choice) => {
          const ChoiceIcon = choiceIcon(choice)
          const appearance = browserAppearanceForTarget(choice, appearances)
          const displayedAppearance = {
            ...appearance,
            icon: appearance.icon || choice.icon || '',
          }
          const customized = Boolean(displayedAppearance.icon || displayedAppearance.primaryColor)
          const selected = choice.id === selectedID
          return (
            <button
              key={choice.id}
              className={`${selected ? 'is-selected' : ''} ${customized ? 'has-custom-appearance' : ''}`}
              style={browserAppearanceStyle(displayedAppearance)}
              type="button"
              role="option"
              aria-selected={selected}
              disabled={Boolean(openingID)}
              aria-label={`Open in ${choice.label}`}
              onFocus={() => setSelectedID(choice.id)}
              onClick={() => choose(choice.id)}
            >
              <span className="url-route-choice-list__icon" aria-hidden="true">
                <BrowserAppearanceMark
                  target={choice}
                  appearance={displayedAppearance}
                  fallbackIcon={ChoiceIcon}
                />
              </span>
              <span>
                <strong>{choice.label}</strong>
                <small>{choice.id === request.defaultChoiceId ? `${choice.detail} · Default` : choice.detail}</small>
              </span>
              {openingID === choice.id ? <LoaderCircle className="spin" aria-hidden="true" /> : <ArrowRight aria-hidden="true" />}
            </button>
          )
        })}
      </div>
    </div>
  )
}
