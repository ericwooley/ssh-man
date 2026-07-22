import {
  Activity,
  ChevronLeft,
  CircleAlert,
  CircleCheck,
  Info,
  LoaderCircle,
  PanelTopClose,
  Server,
  Settings,
  X,
} from 'lucide-react'
import { statusLabel, statusTone } from '../model/appModel'

export function IconButton({ label, children, className = '', ...props }) {
  return (
    <button className={`icon-button ${className}`.trim()} type="button" aria-label={label} title={label} {...props}>
      {children}
    </button>
  )
}

export function BrandMark(props) {
  return (
    <svg viewBox="0 0 100 100" fill="none" focusable="false" {...props}>
      <g stroke="currentColor" strokeWidth="7" strokeLinecap="round" strokeLinejoin="round">
        <rect x="12" y="21" width="76" height="58" rx="11" />
        <path d="m30 40 12 10-12 10" />
        <path d="M53 60h17" />
      </g>
    </svg>
  )
}

export function AppHeader({ title, subtitle, onBack, onHide, children }) {
  return (
    <header className="app-header">
      <div className="app-header__side">
        {onBack ? (
          <IconButton label="Go back" onClick={onBack}>
            <ChevronLeft aria-hidden="true" />
          </IconButton>
        ) : (
          <div className="app-mark" aria-hidden="true"><BrandMark /></div>
        )}
      </div>
      <div className="app-header__copy">
        <h1>{title}</h1>
        {subtitle ? <p>{subtitle}</p> : null}
      </div>
      <div className="app-header__actions">
        {children}
        {onHide ? (
          <IconButton label="Hide SSH Man" onClick={onHide}>
            <PanelTopClose aria-hidden="true" />
          </IconButton>
        ) : null}
      </div>
    </header>
  )
}

const navigationItems = [
  { id: 'servers', label: 'Servers', Icon: Server },
  { id: 'activity', label: 'Active', Icon: Activity },
  { id: 'settings', label: 'Settings', Icon: Settings },
]

export function BottomNav({ current, activeCount, onChange }) {
  return (
    <nav className="bottom-nav" aria-label="Main navigation">
      {navigationItems.map(({ id, label, Icon }) => (
        <button
          key={id}
          type="button"
          className={current === id ? 'bottom-nav__item is-current' : 'bottom-nav__item'}
          aria-current={current === id ? 'page' : undefined}
          onClick={() => onChange(id)}
        >
          <span className="bottom-nav__icon">
            <Icon aria-hidden="true" />
            {id === 'activity' && activeCount > 0 ? <span className="bottom-nav__badge">{activeCount}</span> : null}
          </span>
          <span>{label}</span>
        </button>
      ))}
    </nav>
  )
}

export function StatusPill({ status = 'stopped', compact = false }) {
  return (
    <span className={`status-pill tone-${statusTone(status)} ${compact ? 'is-compact' : ''}`} aria-label={`Tunnel status: ${statusLabel(status)}`}>
      <span className="status-dot" aria-hidden="true" />
      {statusLabel(status)}
    </span>
  )
}

export function EmptyState({ icon: Icon = Server, title, description, action }) {
  return (
    <div className="empty-state">
      <span className="empty-state__icon" aria-hidden="true"><Icon /></span>
      <h2>{title}</h2>
      <p>{description}</p>
      {action}
    </div>
  )
}

export function LoadingScreen() {
  return (
    <div className="loading-screen" role="status" aria-live="polite">
      <LoaderCircle className="spin" aria-hidden="true" />
      <strong>Loading your servers</strong>
      <span>Checking saved tunnels and live connections…</span>
    </div>
  )
}

const toastIcons = {
  success: CircleCheck,
  warning: CircleAlert,
  danger: CircleAlert,
  info: Info,
}

export function ToastRegion({ notification, onDismiss }) {
  if (!notification) return <div className="toast-region" aria-live="polite" aria-atomic="true" />
  const Icon = toastIcons[notification.kind] || Info

  return (
    <div className="toast-region" aria-live={notification.kind === 'danger' ? 'assertive' : 'polite'} aria-atomic="true">
      <div className={`toast toast--${notification.kind}`} role={notification.kind === 'danger' ? 'alert' : 'status'}>
        <Icon aria-hidden="true" />
        <div className="toast__copy">
          <strong>{notification.message}</strong>
          {notification.detail ? <span>{notification.detail}</span> : null}
        </div>
        <IconButton label="Dismiss message" className="toast__close" onClick={onDismiss}>
          <X aria-hidden="true" />
        </IconButton>
      </div>
    </div>
  )
}
