export function hasSettingsRuntime() {
  if (typeof window === 'undefined') return false
  return Boolean(window.go?.bindings?.SettingsWindowBindings)
}
