function bindings() {
  if (typeof window === 'undefined') return null
  return window.go?.bindings?.PreviewBindings || null
}

function requireBindings() {
  const value = bindings()
  if (!value) throw new Error('The preview window runtime is unavailable.')
  return value
}

export function hasPreviewRuntime() {
  return bindings() !== null
}

export async function initialState() {
  return requireBindings().InitialState()
}

export async function connect(passphrase = '') {
  return requireBindings().Connect(passphrase)
}

export async function previewFile() {
  return requireBindings().PreviewFile()
}

function requireFullscreenRuntime() {
  const runtime = window.runtime
  if (
    typeof runtime?.WindowIsFullscreen !== 'function'
    || typeof runtime.WindowFullscreen !== 'function'
    || typeof runtime.WindowUnfullscreen !== 'function'
  ) {
    throw new Error('Fullscreen is unavailable.')
  }
  return runtime
}

export async function fullscreenState() {
  return requireFullscreenRuntime().WindowIsFullscreen()
}

export async function toggleFullscreen() {
  const runtime = requireFullscreenRuntime()
  const isFullscreen = await runtime.WindowIsFullscreen()
  if (isFullscreen) runtime.WindowUnfullscreen()
  else runtime.WindowFullscreen()
  return !isFullscreen
}
