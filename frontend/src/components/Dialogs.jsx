import { useEffect, useRef, useState } from 'react'
import { KeyRound, LoaderCircle, Trash2, X } from 'lucide-react'
import { IconButton } from './AppChrome'

function useNativeDialog(open, onClose) {
  const ref = useRef(null)

  useEffect(() => {
    const dialog = ref.current
    if (!dialog) return

    if (open && !dialog.open) {
      if (typeof dialog.showModal === 'function') dialog.showModal()
      else dialog.setAttribute('open', '')
    } else if (!open && dialog.open) {
      if (typeof dialog.close === 'function') dialog.close()
      else dialog.removeAttribute('open')
    }
  }, [open])

  function handleCancel(event) {
    event.preventDefault()
    onClose()
  }

  return { ref, handleCancel }
}

export function ConfirmDialog({ request, pending, onClose, onConfirm }) {
  const { ref, handleCancel } = useNativeDialog(Boolean(request), onClose)
  if (!request) return null

  return (
    <dialog ref={ref} className="native-dialog" aria-labelledby="confirm-title" aria-describedby="confirm-description" onCancel={handleCancel}>
      <div className="dialog-icon dialog-icon--danger" aria-hidden="true"><Trash2 /></div>
      <div className="dialog-copy">
        <h2 id="confirm-title">{request.title}</h2>
        <p id="confirm-description">{request.description}</p>
      </div>
      <div className="dialog-actions">
        <button className="secondary-button" type="button" disabled={pending} onClick={onClose}>Cancel</button>
        <button className="danger-button" type="button" disabled={pending} onClick={onConfirm}>
          {pending ? <LoaderCircle className="spin" aria-hidden="true" /> : <Trash2 aria-hidden="true" />}
          {pending ? 'Deleting…' : request.confirmLabel || 'Delete'}
        </button>
      </div>
    </dialog>
  )
}

export function UnlockDialog({ request, pending, onClose, onSubmit }) {
  const [secret, setSecret] = useState('')
  const inputRef = useRef(null)
  const { ref, handleCancel } = useNativeDialog(Boolean(request), onClose)

  useEffect(() => {
    if (request) {
      setSecret('')
      requestAnimationFrame(() => inputRef.current?.focus())
    }
  }, [request])

  if (!request) return null

  function handleSubmit(event) {
    event.preventDefault()
    if (secret.trim()) onSubmit(secret)
  }

  return (
    <dialog ref={ref} className="native-dialog" aria-labelledby="unlock-title" aria-describedby="unlock-description" onCancel={handleCancel}>
      <form onSubmit={handleSubmit}>
        <div className="dialog-topline">
          <div className="dialog-icon dialog-icon--warning" aria-hidden="true"><KeyRound /></div>
          <IconButton label="Close unlock prompt" onClick={onClose}><X aria-hidden="true" /></IconButton>
        </div>
        <div className="dialog-copy">
          <span className="eyebrow">Key required</span>
          <h2 id="unlock-title">Unlock SSH key</h2>
          <p id="unlock-description">Enter the passphrase for {request.configurationLabel}. It is used for this connection only and is never saved.</p>
          {request.detail ? <div className="inline-alert inline-alert--warning"><KeyRound aria-hidden="true" /><div><span>{request.detail}</span></div></div> : null}
        </div>
        <label className="field-group" htmlFor="key-passphrase">
          <span>Passphrase</span>
          <input id="key-passphrase" ref={inputRef} type="password" autoComplete="current-password" value={secret} onChange={(event) => setSecret(event.target.value)} />
        </label>
        <div className="dialog-actions">
          <button className="secondary-button" type="button" disabled={pending} onClick={onClose}>Not now</button>
          <button className="primary-button" type="submit" disabled={pending || !secret.trim()}>
            {pending ? <LoaderCircle className="spin" aria-hidden="true" /> : <KeyRound aria-hidden="true" />}
            {pending ? 'Unlocking…' : 'Unlock and connect'}
          </button>
        </div>
      </form>
    </dialog>
  )
}
