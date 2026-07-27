import { useEffect, useLayoutEffect, useRef } from 'react'

const menuMargin = 8

export default function ExplorerContextMenu({ items, onClose, position }) {
  const menuRef = useRef(null)

  useLayoutEffect(() => {
    const menu = menuRef.current
    if (!menu) return
    const bounds = menu.getBoundingClientRect()
    const left = Math.max(menuMargin, Math.min(position.x, window.innerWidth - bounds.width - menuMargin))
    const top = Math.max(menuMargin, Math.min(position.y, window.innerHeight - bounds.height - menuMargin))
    menu.style.left = `${left}px`
    menu.style.top = `${top}px`
  }, [items, position])

  useEffect(() => {
    const menu = menuRef.current
    menu?.querySelector('button:not(:disabled)')?.focus()

    function closeForOutsidePointer(event) {
      if (!menu?.contains(event.target)) onClose()
    }

    function closeForWindowChange() {
      onClose()
    }

    window.addEventListener('pointerdown', closeForOutsidePointer)
    window.addEventListener('blur', closeForWindowChange)
    window.addEventListener('resize', closeForWindowChange)
    window.addEventListener('scroll', closeForWindowChange, true)
    return () => {
      window.removeEventListener('pointerdown', closeForOutsidePointer)
      window.removeEventListener('blur', closeForWindowChange)
      window.removeEventListener('resize', closeForWindowChange)
      window.removeEventListener('scroll', closeForWindowChange, true)
    }
  }, [onClose])

  function handleKeyDown(event) {
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
      return
    }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const buttons = Array.from(menuRef.current?.querySelectorAll('button:not(:disabled)') || [])
    if (!buttons.length) return
    const currentIndex = buttons.indexOf(document.activeElement)
    let nextIndex
    if (event.key === 'Home') nextIndex = 0
    else if (event.key === 'End') nextIndex = buttons.length - 1
    else if (event.key === 'ArrowDown') nextIndex = (currentIndex + 1 + buttons.length) % buttons.length
    else nextIndex = (currentIndex - 1 + buttons.length) % buttons.length
    buttons[nextIndex]?.focus()
  }

  return (
    <div
      ref={menuRef}
      className="explorer-context-menu"
      role="menu"
      aria-label="File actions"
      style={{ left: position.x, top: position.y }}
      onContextMenu={(event) => event.preventDefault()}
      onKeyDown={handleKeyDown}
    >
      {items.map((item, index) => {
        if (item.separator) {
          return <div className="explorer-context-menu__separator" role="separator" key={`separator-${index}`} />
        }
        const Icon = item.icon
        return (
          <button
            type="button"
            role="menuitem"
            aria-label={item.label}
            className={item.danger ? 'is-danger' : ''}
            disabled={item.disabled}
            key={item.label}
            onClick={() => {
              onClose()
              item.action()
            }}
          >
            <span>{Icon ? <Icon aria-hidden="true" /> : null}{item.label}</span>
            {item.shortcut ? <kbd>{item.shortcut}</kbd> : null}
          </button>
        )
      })}
    </div>
  )
}
