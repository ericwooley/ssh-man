import { useEffect, useMemo, useRef, useState } from 'react'
import { Check, ChevronLeft, LoaderCircle, Save, ShieldCheck } from 'lucide-react'
import { emptyServer, emptyTunnel, validateServer, validateTunnel } from '../model/appModel'
import { IconButton } from '../components/AppChrome'

function focusFirstError(errors, refs) {
  const firstKey = Object.keys(errors)[0]
  refs.current[firstKey]?.focus()
}

function Field({ id, label, error, hint, children }) {
  const descriptionIds = [error ? `${id}-error` : '', hint ? `${id}-hint` : ''].filter(Boolean).join(' ') || undefined
  return (
    <label className={error ? 'field-group has-error' : 'field-group'} htmlFor={id}>
      <span>{label}</span>
      {typeof children === 'function' ? children({ describedBy: descriptionIds }) : children}
      {hint ? <small id={`${id}-hint`} className="field-hint">{hint}</small> : null}
      {error ? <small id={`${id}-error`} className="field-message field-message--error">{error}</small> : null}
    </label>
  )
}

function FormHeader({ title, subtitle, onCancel }) {
  return (
    <header className="form-header">
      <IconButton label="Cancel and go back" onClick={onCancel}>
        <ChevronLeft aria-hidden="true" />
      </IconButton>
      <div>
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>
      <span className="form-header__spacer" aria-hidden="true" />
    </header>
  )
}

const CUSTOM_KEY_VALUE = '__custom__'

function initialKeyChoice(value, sshKeys) {
  if (!value?.keyReference) return ''
  return sshKeys.some((key) => key.path === value.keyReference) ? value.keyReference : CUSTOM_KEY_VALUE
}

export function ServerFormScreen({ initialValue, currentUsername, sshKeys = [], pending, onCancel, onSave }) {
  const [draft, setDraft] = useState(() => ({ ...emptyServer(currentUsername), ...(initialValue || {}) }))
  const [keyChoice, setKeyChoice] = useState(() => initialKeyChoice(initialValue, sshKeys))
  const [errors, setErrors] = useState({})
  const refs = useRef({})
  const isEditing = Boolean(draft.id)

  useEffect(() => {
    refs.current.name?.focus()
  }, [])

  function update(field, value) {
    setDraft((current) => ({ ...current, [field]: value }))
    setErrors((current) => {
      if (!current[field]) return current
      const next = { ...current }
      delete next[field]
      return next
    })
  }

  function changeKeyChoice(value) {
    setKeyChoice(value)
    update('keyReference', value === CUSTOM_KEY_VALUE ? '' : value)
  }

  async function handleSubmit(event) {
    event.preventDefault()
    const nextErrors = validateServer(draft)
    setErrors(nextErrors)
    if (Object.keys(nextErrors).length) {
      requestAnimationFrame(() => focusFirstError(nextErrors, refs))
      return
    }
    const saved = await onSave(draft)
    if (saved) onCancel(saved)
  }

  return (
    <form className="form-screen" onSubmit={handleSubmit} noValidate aria-labelledby="server-form-title">
      <FormHeader
        title={isEditing ? 'Edit server' : 'New server'}
        subtitle={isEditing ? 'Update this saved SSH connection.' : 'Save an SSH connection for repeat use.'}
        onCancel={() => onCancel(null)}
      />

      <div className="form-scroll">
        <section className="form-section" aria-labelledby="server-form-title">
          <div className="form-section__heading">
            <span className="step-number">1</span>
            <div><h2 id="server-form-title">Connection</h2><p>The name is just for you.</p></div>
          </div>

          <Field id="server-name" label="Name" error={errors.name}>
            {({ describedBy }) => (
              <input id="server-name" ref={(node) => { refs.current.name = node }} value={draft.name} onChange={(event) => update('name', event.target.value)} placeholder="Production bastion" autoComplete="off" aria-invalid={Boolean(errors.name)} aria-describedby={describedBy} />
            )}
          </Field>

          <Field id="server-host" label="Host" error={errors.host}>
            {({ describedBy }) => (
              <input id="server-host" ref={(node) => { refs.current.host = node }} value={draft.host} onChange={(event) => update('host', event.target.value)} placeholder="example.com" autoCapitalize="none" spellCheck="false" aria-invalid={Boolean(errors.host)} aria-describedby={describedBy} />
            )}
          </Field>

          <div className="field-row field-row--port">
            <Field id="server-username" label="Username" error={errors.username}>
              {({ describedBy }) => (
                <input id="server-username" ref={(node) => { refs.current.username = node }} value={draft.username} onChange={(event) => update('username', event.target.value)} placeholder="deploy" autoCapitalize="none" spellCheck="false" aria-invalid={Boolean(errors.username)} aria-describedby={describedBy} />
              )}
            </Field>
            <Field id="server-port" label="Port" error={errors.port}>
              {({ describedBy }) => (
                <input id="server-port" ref={(node) => { refs.current.port = node }} type="number" inputMode="numeric" min="1" max="65535" value={draft.port} onChange={(event) => update('port', event.target.value)} aria-invalid={Boolean(errors.port)} aria-describedby={describedBy} />
              )}
            </Field>
          </div>
        </section>

        <section className="form-section" aria-labelledby="authentication-heading">
          <div className="form-section__heading">
            <span className="step-number">2</span>
            <div><h2 id="authentication-heading">Authentication</h2><p>SSH agent is the simplest default.</p></div>
          </div>

          <div className="choice-cards" role="radiogroup" aria-label="Authentication method">
            <button type="button" role="radio" aria-checked={draft.authMode === 'agent'} className={draft.authMode === 'agent' ? 'choice-card is-selected' : 'choice-card'} onClick={() => update('authMode', 'agent')}>
              <ShieldCheck aria-hidden="true" /><span><strong>SSH agent</strong><small>Use keys already loaded on this Mac</small></span>{draft.authMode === 'agent' ? <Check aria-hidden="true" /> : null}
            </button>
            <button type="button" role="radio" aria-checked={draft.authMode === 'private_key'} className={draft.authMode === 'private_key' ? 'choice-card is-selected' : 'choice-card'} onClick={() => update('authMode', 'private_key')}>
              <Save aria-hidden="true" /><span><strong>Private key</strong><small>Use a key file from disk</small></span>{draft.authMode === 'private_key' ? <Check aria-hidden="true" /> : null}
            </button>
          </div>

          {draft.authMode === 'private_key' ? (
            <>
              <Field id="server-key-choice" label="Private key" error={keyChoice === CUSTOM_KEY_VALUE ? undefined : errors.keyReference} hint="Choose a key from ~/.ssh or use a custom path.">
                {({ describedBy }) => (
                  <select id="server-key-choice" ref={(node) => { refs.current.keyReference = node }} value={keyChoice} onChange={(event) => changeKeyChoice(event.target.value)} aria-invalid={Boolean(errors.keyReference && keyChoice !== CUSTOM_KEY_VALUE)} aria-describedby={describedBy}>
                    <option value="">Choose a private key…</option>
                    {sshKeys.map((key) => <option key={key.path} value={key.path}>{key.name}</option>)}
                    <option value={CUSTOM_KEY_VALUE}>Custom path…</option>
                  </select>
                )}
              </Field>
              {keyChoice === CUSTOM_KEY_VALUE ? (
                <Field id="server-key-reference" label="Private key path" error={errors.keyReference} hint="The passphrase is requested only when a tunnel starts.">
                  {({ describedBy }) => (
                    <input id="server-key-reference" ref={(node) => { refs.current.keyReference = node }} value={draft.keyReference} onChange={(event) => update('keyReference', event.target.value)} placeholder="/Users/you/.ssh/id_ed25519" autoCapitalize="none" spellCheck="false" aria-invalid={Boolean(errors.keyReference)} aria-describedby={describedBy} />
                  )}
                </Field>
              ) : null}
            </>
          ) : null}
        </section>
      </div>

      <footer className="form-footer">
        <button className="secondary-button" type="button" onClick={() => onCancel(null)}>Cancel</button>
        <button className="primary-button" type="submit" disabled={pending}>
          {pending ? <LoaderCircle className="spin" aria-hidden="true" /> : <Save aria-hidden="true" />}
          {pending ? 'Saving…' : 'Save server'}
        </button>
      </footer>
    </form>
  )
}

function initialSocksMode(value) {
  return value?.socksPort && Number(value.socksPort) > 0 ? 'manual' : 'auto'
}

export function TunnelFormScreen({ server, initialValue, pending, onCancel, onSave }) {
  const startingValue = useMemo(() => ({ ...emptyTunnel(server.id), ...(initialValue || {}) }), [initialValue, server.id])
  const [draft, setDraft] = useState(startingValue)
  const [socksMode, setSocksMode] = useState(() => initialSocksMode(startingValue))
  const [errors, setErrors] = useState({})
  const refs = useRef({})
  const isEditing = Boolean(draft.id)

  useEffect(() => {
    refs.current.label?.focus()
  }, [])

  function update(field, value) {
    setDraft((current) => ({ ...current, [field]: value }))
    setErrors((current) => {
      if (!current[field]) return current
      const next = { ...current }
      delete next[field]
      return next
    })
  }

  function changeType(connectionType) {
    setDraft((current) => ({ ...current, connectionType }))
    setErrors({})
  }

  function changeSocksMode(mode) {
    setSocksMode(mode)
    update('socksPort', mode === 'auto' ? 'auto' : '')
  }

  async function handleSubmit(event) {
    event.preventDefault()
    const nextErrors = validateTunnel(draft, { socksPortMode: socksMode })
    setErrors(nextErrors)
    if (Object.keys(nextErrors).length) {
      requestAnimationFrame(() => focusFirstError(nextErrors, refs))
      return
    }
    const saved = await onSave(draft)
    if (saved) onCancel(saved)
  }

  return (
    <form className="form-screen" onSubmit={handleSubmit} noValidate aria-labelledby="tunnel-form-title">
      <FormHeader
        title={isEditing ? 'Edit tunnel' : 'New tunnel'}
        subtitle={`${server.name} · ${server.username}@${server.host}`}
        onCancel={() => onCancel(null)}
      />

      <div className="form-scroll">
        <section className="form-section" aria-labelledby="tunnel-form-title">
          <div className="form-section__heading">
            <span className="step-number">1</span>
            <div><h2 id="tunnel-form-title">Tunnel</h2><p>Name it and choose how traffic should flow.</p></div>
          </div>

          <Field id="tunnel-label" label="Label" error={errors.label}>
            {({ describedBy }) => (
              <input id="tunnel-label" ref={(node) => { refs.current.label = node }} value={draft.label} onChange={(event) => update('label', event.target.value)} placeholder="Admin dashboard" autoComplete="off" aria-invalid={Boolean(errors.label)} aria-describedby={describedBy} />
            )}
          </Field>

          <div className="choice-cards choice-cards--horizontal" role="radiogroup" aria-label="Tunnel type">
            <button type="button" role="radio" aria-checked={draft.connectionType === 'local_forward'} className={draft.connectionType === 'local_forward' ? 'choice-card is-selected' : 'choice-card'} onClick={() => changeType('local_forward')}>
              <span><strong>Local forward</strong><small>Local app to a remote port</small></span>{draft.connectionType === 'local_forward' ? <Check aria-hidden="true" /> : null}
            </button>
            <button type="button" role="radio" aria-checked={draft.connectionType === 'socks_proxy'} className={draft.connectionType === 'socks_proxy' ? 'choice-card is-selected' : 'choice-card'} onClick={() => changeType('socks_proxy')}>
              <span><strong>SOCKS proxy</strong><small>Browser through this server</small></span>{draft.connectionType === 'socks_proxy' ? <Check aria-hidden="true" /> : null}
            </button>
          </div>
        </section>

        <section className="form-section" aria-labelledby="routing-heading">
          <div className="form-section__heading">
            <span className="step-number">2</span>
            <div><h2 id="routing-heading">Routing</h2><p>{draft.connectionType === 'socks_proxy' ? 'Choose a local proxy port.' : 'Map a local port to the remote service.'}</p></div>
          </div>

          {draft.connectionType === 'local_forward' ? (
            <>
              <Field id="tunnel-local-port" label="Local port" error={errors.localPort}>
                {({ describedBy }) => (
                  <input id="tunnel-local-port" ref={(node) => { refs.current.localPort = node }} type="number" inputMode="numeric" min="1" max="65535" value={draft.localPort} onChange={(event) => update('localPort', event.target.value)} placeholder="3000" aria-invalid={Boolean(errors.localPort)} aria-describedby={describedBy} />
                )}
              </Field>
              <div className="field-row field-row--host-port">
                <Field id="tunnel-remote-host" label="Remote host" error={errors.remoteHost}>
                  {({ describedBy }) => (
                    <input id="tunnel-remote-host" ref={(node) => { refs.current.remoteHost = node }} value={draft.remoteHost} onChange={(event) => update('remoteHost', event.target.value)} placeholder="127.0.0.1" autoCapitalize="none" spellCheck="false" aria-invalid={Boolean(errors.remoteHost)} aria-describedby={describedBy} />
                  )}
                </Field>
                <Field id="tunnel-remote-port" label="Remote port" error={errors.remotePort}>
                  {({ describedBy }) => (
                    <input id="tunnel-remote-port" ref={(node) => { refs.current.remotePort = node }} type="number" inputMode="numeric" min="1" max="65535" value={draft.remotePort} onChange={(event) => update('remotePort', event.target.value)} placeholder="3000" aria-invalid={Boolean(errors.remotePort)} aria-describedby={describedBy} />
                  )}
                </Field>
              </div>
            </>
          ) : (
            <>
              <div className="segmented-control" role="group" aria-label="SOCKS port mode">
                <button type="button" className={socksMode === 'auto' ? 'is-selected' : ''} aria-pressed={socksMode === 'auto'} onClick={() => changeSocksMode('auto')}>Auto {socksMode === 'auto' ? <Check aria-hidden="true" /> : null}</button>
                <button type="button" className={socksMode === 'manual' ? 'is-selected' : ''} aria-pressed={socksMode === 'manual'} onClick={() => changeSocksMode('manual')}>Manual {socksMode === 'manual' ? <Check aria-hidden="true" /> : null}</button>
              </div>
              {socksMode === 'manual' ? (
                <Field id="tunnel-socks-port" label="SOCKS port" error={errors.socksPort} hint="Use a port from 1 to 65535.">
                  {({ describedBy }) => (
                    <input id="tunnel-socks-port" ref={(node) => { refs.current.socksPort = node }} type="number" inputMode="numeric" min="1" max="65535" value={draft.socksPort === 'auto' ? '' : draft.socksPort} onChange={(event) => update('socksPort', event.target.value)} placeholder="1080" aria-invalid={Boolean(errors.socksPort)} aria-describedby={describedBy} />
                  )}
                </Field>
              ) : (
                <div className="inline-alert inline-alert--info"><Check aria-hidden="true" /><div><strong>Automatic port</strong><span>SSH Man will choose an open localhost port each time this tunnel starts.</span></div></div>
              )}
            </>
          )}
        </section>

        <section className="form-section" aria-labelledby="behavior-heading">
          <div className="form-section__heading">
            <span className="step-number">3</span>
            <div><h2 id="behavior-heading">Behavior</h2><p>Keep the tunnel resilient and recognizable.</p></div>
          </div>

          <label className="toggle-row">
            <input type="checkbox" checked={Boolean(draft.autoReconnectEnabled)} onChange={(event) => update('autoReconnectEnabled', event.target.checked)} />
            <span className="toggle" aria-hidden="true"><span /></span>
            <span><strong>Reconnect automatically</strong><small>Recover after sleep or a transient network change.</small></span>
          </label>

          <label className="toggle-row">
            <input type="checkbox" checked={Boolean(draft.startOnLaunch)} onChange={(event) => update('startOnLaunch', event.target.checked)} />
            <span className="toggle" aria-hidden="true"><span /></span>
            <span><strong>Connect when SSH Man starts</strong><small>Start this tunnel automatically when the app launches.</small></span>
          </label>

          <Field id="tunnel-notes" label="Notes" hint="Optional context for the next time you use this tunnel.">
            {({ describedBy }) => (
              <textarea id="tunnel-notes" rows="3" value={draft.notes} onChange={(event) => update('notes', event.target.value)} placeholder="Production admin UI" aria-describedby={describedBy} />
            )}
          </Field>
        </section>
      </div>

      <footer className="form-footer">
        <button className="secondary-button" type="button" onClick={() => onCancel(null)}>Cancel</button>
        <button className="primary-button" type="submit" disabled={pending}>
          {pending ? <LoaderCircle className="spin" aria-hidden="true" /> : <Save aria-hidden="true" />}
          {pending ? 'Saving…' : 'Save tunnel'}
        </button>
      </footer>
    </form>
  )
}
