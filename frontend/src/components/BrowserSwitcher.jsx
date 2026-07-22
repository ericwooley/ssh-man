import { useEffect, useRef, useState } from 'react'
import {
  ArrowLeft,
  Briefcase,
  Check,
  Code2,
  Globe2,
  LoaderCircle,
  Network,
  Palette,
  RefreshCw,
  RotateCcw,
  Save,
  Shield,
  Star,
  Terminal,
  X,
} from 'lucide-react'
import {
  browserAppearanceForTarget,
  browserAppearanceForeground,
  browserAppearanceKey,
  moveBrowserSelectionIndex,
  normalizeBrowserAppearance,
  validateBrowserAppearance,
} from '../model/appModel'
import { IconButton } from './AppChrome'

const ICON_OPTIONS = [
  { value: '', label: 'Default' },
  { value: 'icon:x', label: 'X', Icon: X },
  { value: 'icon:shield', label: 'Shield', Icon: Shield },
  { value: 'icon:terminal', label: 'Terminal', Icon: Terminal },
  { value: 'icon:globe', label: 'Globe', Icon: Globe2 },
  { value: 'icon:network', label: 'Network', Icon: Network },
  { value: 'icon:star', label: 'Star', Icon: Star },
  { value: 'icon:briefcase', label: 'Briefcase', Icon: Briefcase },
  { value: 'icon:code', label: 'Code', Icon: Code2 },
]

const ICON_COMPONENTS = Object.fromEntries(
  ICON_OPTIONS.filter(({ Icon }) => Icon).map(({ value, Icon }) => [value, Icon]),
)

const COLOR_OPTIONS = [
  { value: '#22C55E', label: 'Green' },
  { value: '#3B82F6', label: 'Blue' },
  { value: '#8B5CF6', label: 'Purple' },
  { value: '#F97316', label: 'Orange' },
  { value: '#EF4444', label: 'Red' },
  { value: '#14B8A6', label: 'Teal' },
  { value: '#EC4899', label: 'Pink' },
]

function BrowserMark({ item, appearance }) {
  const Icon = ICON_COMPONENTS[appearance.icon]
  if (Icon) return <Icon />
  if (appearance.icon) return <span className="browser-target__mark">{appearance.icon}</span>
  return item.kind === 'proxy' ? <Network /> : <Globe2 />
}

function appearanceStyle(appearance) {
  if (!appearance.primaryColor) return undefined
  return {
    '--browser-primary': appearance.primaryColor,
    '--browser-on-primary': browserAppearanceForeground(appearance.primaryColor),
  }
}

function BrowserAppearanceEditor({ item, draft, errors, onChange }) {
  const appearance = normalizeBrowserAppearance(draft)
  const customMark = appearance.icon && !appearance.icon.startsWith('icon:') ? appearance.icon : ''
  const previewClass = appearance.icon || appearance.primaryColor ? 'has-custom-appearance' : ''
  const proxyLabel = item.kind === 'proxy' ? `${item.serverName || 'Unknown server'} proxy` : 'Regular browser'

  return (
    <div className="browser-appearance-editor">
      <div className="browser-appearance-preview">
        <span
          className={`browser-target__icon ${item.kind === 'proxy' ? 'is-proxy' : ''} ${previewClass}`}
          style={appearanceStyle(appearance)}
          aria-hidden="true"
        >
          <BrowserMark item={item} appearance={appearance} />
        </span>
        <strong>{item.browserName}</strong>
        <span>{proxyLabel}</span>
      </div>

      <fieldset className="browser-appearance-group">
        <legend>Icon</legend>
        <div className="browser-appearance-icons" role="radiogroup" aria-label="Built-in browser icon">
          {ICON_OPTIONS.map(({ value, label, Icon }) => (
            <button
              key={label}
              type="button"
              role="radio"
              aria-checked={appearance.icon === value}
              aria-label={`${label} icon`}
              className={appearance.icon === value ? 'is-selected' : ''}
              onClick={() => onChange({ ...appearance, icon: value })}
            >
              {Icon ? <Icon aria-hidden="true" /> : <RotateCcw aria-hidden="true" />}
            </button>
          ))}
        </div>
        <label className={errors.icon ? 'browser-appearance-custom has-error' : 'browser-appearance-custom'}>
          <span>Emoji or mark</span>
          <input
            aria-label="Custom emoji or mark"
            value={customMark}
            placeholder="X or 🟢"
            onChange={(event) => onChange({ ...appearance, icon: event.target.value })}
            aria-invalid={Boolean(errors.icon)}
            aria-describedby={errors.icon ? 'browser-appearance-icon-error' : undefined}
          />
        </label>
        {errors.icon ? <small id="browser-appearance-icon-error" className="field-message field-message--error">{errors.icon}</small> : null}
      </fieldset>

      <fieldset className="browser-appearance-group">
        <legend>Primary color</legend>
        <div className="browser-appearance-colors" role="radiogroup" aria-label="Browser primary color">
          <button
            type="button"
            role="radio"
            aria-checked={!appearance.primaryColor}
            aria-label="Default color"
            className={`browser-color-swatch is-default ${!appearance.primaryColor ? 'is-selected' : ''}`}
            onClick={() => onChange({ ...appearance, primaryColor: '' })}
          >
            <RotateCcw aria-hidden="true" />
          </button>
          {COLOR_OPTIONS.map(({ value, label }) => (
            <button
              key={value}
              type="button"
              role="radio"
              aria-checked={appearance.primaryColor === value}
              aria-label={`${label} color`}
              className={`browser-color-swatch ${appearance.primaryColor === value ? 'is-selected' : ''}`}
              style={{ '--swatch': value }}
              onClick={() => onChange({ ...appearance, primaryColor: value })}
            />
          ))}
        </div>
        <label className="browser-appearance-color-input">
          <span>Custom</span>
          <input
            type="color"
            aria-label="Custom primary color"
            value={appearance.primaryColor || '#3B82F6'}
            onChange={(event) => onChange({ ...appearance, primaryColor: event.target.value.toUpperCase() })}
          />
          <code>{appearance.primaryColor || 'Default'}</code>
        </label>
        {errors.primaryColor ? <small className="field-message field-message--error">{errors.primaryColor}</small> : null}
      </fieldset>
    </div>
  )
}

export function BrowserSwitcher({
  items,
  selectedIndex,
  loading,
  error,
  activating,
  mode,
  forwardShortcut,
  backwardShortcut,
  appearances = {},
  onSelect,
  onActivate,
  onSaveAppearance,
  onRefresh,
  onClose,
}) {
  const itemRefs = useRef([])
  const [editingKey, setEditingKey] = useState('')
  const [appearanceDraft, setAppearanceDraft] = useState({ icon: '', primaryColor: '' })
  const [appearanceErrors, setAppearanceErrors] = useState({})
  const [savingAppearance, setSavingAppearance] = useState(false)

  const editingItem = editingKey
    ? items.find((item) => browserAppearanceKey(item) === editingKey)
    : null
  const selectedItem = items[selectedIndex]

  useEffect(() => {
    if (editingItem) return
    const selected = itemRefs.current[selectedIndex]
    selected?.focus()
    selected?.scrollIntoView?.({ block: 'nearest', inline: 'center' })
  }, [editingItem, items, selectedIndex])

  useEffect(() => {
    if (editingKey && !editingItem) {
      setEditingKey('')
      setAppearanceErrors({})
    }
  }, [editingItem, editingKey])

  function closeEditor() {
    setEditingKey('')
    setAppearanceErrors({})
    setSavingAppearance(false)
  }

  function openEditor(item) {
    const key = browserAppearanceKey(item)
    if (!key || !onSaveAppearance) return
    setAppearanceDraft(browserAppearanceForTarget(item, appearances))
    setAppearanceErrors({})
    setEditingKey(key)
  }

  async function saveAppearance(nextAppearance = appearanceDraft) {
    const nextErrors = validateBrowserAppearance(nextAppearance)
    setAppearanceErrors(nextErrors)
    if (Object.keys(nextErrors).length || !editingKey) return
    setSavingAppearance(true)
    try {
      const saved = await onSaveAppearance(editingKey, normalizeBrowserAppearance(nextAppearance))
      if (saved) closeEditor()
    } finally {
      setSavingAppearance(false)
    }
  }

  useEffect(() => {
    function handleKeyDown(event) {
      if (event.key === 'Escape') {
        event.preventDefault()
        if (editingItem) closeEditor()
        else onClose()
        return
      }
      if (editingItem || !items.length || activating || error) return
      if (event.key === 'Enter' && event.target instanceof Element && event.target.closest('button, input, select, textarea, a')) {
        return
      }
      if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
        event.preventDefault()
        onSelect(moveBrowserSelectionIndex(selectedIndex, items.length, 'forward'))
      } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
        event.preventDefault()
        onSelect(moveBrowserSelectionIndex(selectedIndex, items.length, 'backward'))
      } else if (event.key === 'Enter') {
        event.preventDefault()
        onActivate(items[selectedIndex])
      } else if (/^[1-9]$/.test(event.key)) {
        const index = Number(event.key) - 1
        if (items[index]) {
          event.preventDefault()
          onSelect(index)
          onActivate(items[index])
        }
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [activating, editingItem, error, items, onActivate, onClose, onSelect, selectedIndex])

  const manual = mode === 'manual'

  return (
    <section className="browser-switcher" role="dialog" aria-modal="true" aria-labelledby="browser-switcher-title">
      <header className="browser-switcher__header">
        <div>
          <span className="eyebrow">{editingItem ? 'Browser appearance' : 'Quick switch'}</span>
          <h1 id="browser-switcher-title">{editingItem ? `Customize ${editingItem.browserName}` : 'Switch browser'}</h1>
          <p>{editingItem
            ? 'Choose a recognizable icon and color for this switcher tile.'
            : manual
              ? 'Click a browser or select it with the arrow keys and press Enter.'
              : 'Cycle while holding the shortcut modifier, then release to activate.'}</p>
        </div>
        <div className="browser-switcher__actions">
          {editingItem ? (
            <IconButton label="Back to running browsers" disabled={savingAppearance} onClick={closeEditor}>
              <ArrowLeft aria-hidden="true" />
            </IconButton>
          ) : (
            <IconButton label="Refresh running browsers" disabled={loading || activating} onClick={onRefresh}>
              <RefreshCw className={loading ? 'spin' : ''} aria-hidden="true" />
            </IconButton>
          )}
          <IconButton label="Close browser switcher" onClick={onClose}>
            <X aria-hidden="true" />
          </IconButton>
        </div>
      </header>

      <div className="browser-switcher__content">
        {editingItem ? (
          <BrowserAppearanceEditor
            item={editingItem}
            draft={appearanceDraft}
            errors={appearanceErrors}
            onChange={(next) => {
              setAppearanceDraft(next)
              setAppearanceErrors({})
            }}
          />
        ) : loading ? (
          <div className="browser-switcher__state" role="status">
            <LoaderCircle className="spin" aria-hidden="true" />
            <strong>Finding browser instances…</strong>
          </div>
        ) : error ? (
          <div className="browser-switcher__state" role="alert">
            <Globe2 aria-hidden="true" />
            <strong>Browsers could not be listed</strong>
            <span>{error}</span>
          </div>
        ) : items.length ? (
          <div className="browser-switcher__grid" role="listbox" aria-label="Running browsers" aria-orientation="horizontal">
            {items.map((item, index) => {
              const selected = index === selectedIndex
              const proxy = item.kind === 'proxy'
              const appearance = browserAppearanceForTarget(item, appearances)
              const customized = appearance.icon || appearance.primaryColor
              return (
                <button
                  key={item.id}
                  ref={(node) => { itemRefs.current[index] = node }}
                  className={`browser-target ${selected ? 'is-selected' : ''} ${customized ? 'has-custom-appearance' : ''}`}
                  style={appearanceStyle(appearance)}
                  type="button"
                  role="option"
                  aria-selected={selected}
                  disabled={activating}
                  onFocus={() => onSelect(index)}
                  onMouseEnter={() => onSelect(index)}
                  onClick={() => onActivate(item)}
                >
                  <span className={`browser-target__icon ${proxy ? 'is-proxy' : ''}`} aria-hidden="true">
                    <BrowserMark item={item} appearance={appearance} />
                  </span>
                  {selected ? <span className="browser-target__selection" aria-hidden="true"><Check /></span> : null}
                  <span className="browser-target__copy">
                    <strong>{item.browserName}</strong>
                    <span>{proxy ? `${item.serverName || 'Unknown server'} proxy` : 'Regular browser'}</span>
                  </span>
                  <span className={`browser-target__badge ${proxy ? 'is-proxy' : ''}`}>{proxy ? 'SOCKS' : 'Regular'}</span>
                </button>
              )
            })}
          </div>
        ) : (
          <div className="browser-switcher__state" role="status">
            <Globe2 aria-hidden="true" />
            <strong>No supported browsers are running</strong>
            <span>Launch a browser normally or through a connected SOCKS tunnel, then refresh.</span>
          </div>
        )}
      </div>

      <footer className="browser-switcher__footer">
        {editingItem ? (
          <>
            <button className="browser-switcher__footer-button" type="button" disabled={savingAppearance} onClick={() => saveAppearance({ icon: '', primaryColor: '' })}>
              <RotateCcw aria-hidden="true" /> Use default
            </button>
            <span className="browser-switcher__commit-hint">Saved for this browser across restarts</span>
            <button className="browser-switcher__footer-button" type="button" disabled={savingAppearance} onClick={closeEditor}>Cancel</button>
            <button className="browser-switcher__footer-button is-primary" type="button" disabled={savingAppearance} onClick={() => saveAppearance()}>
              {savingAppearance ? <LoaderCircle className="spin" aria-hidden="true" /> : <Save aria-hidden="true" />} Save
            </button>
          </>
        ) : (
          <>
            <span><kbd>←</kbd><kbd>→</kbd> select</span>
            {manual ? (
              <>
                <button className="browser-switcher__footer-button" type="button" disabled={!selectedItem || !onSaveAppearance} onClick={() => openEditor(selectedItem)}>
                  <Palette aria-hidden="true" /> Customize selected
                </button>
                <span className="browser-switcher__commit-hint">Click a tile or press <kbd>Enter</kbd> to activate</span>
              </>
            ) : (
              <>
                <span><kbd>{backwardShortcut || 'Alt+Z'}</kbd><kbd>{forwardShortcut || 'Alt+X'}</kbd> cycle</span>
                <span className="browser-switcher__commit-hint">Release the shortcut modifier to activate</span>
              </>
            )}
            <span><kbd>Esc</kbd> cancel</span>
            {activating ? <span className="browser-switcher__activating"><LoaderCircle className="spin" aria-hidden="true" /> Activating…</span> : null}
          </>
        )}
      </footer>
    </section>
  )
}
