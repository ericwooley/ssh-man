import React, { lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import App from './app/App'
import { hasCommandRuntime } from './lib/commandApi'
import { hasExplorerRuntime } from './lib/explorerApi'
import { hasHostRuntime } from './lib/hostApi'
import { hasPreviewRuntime } from './lib/previewApi'
import { hasSettingsRuntime } from './lib/settingsApi'
import './app.css'

const ExplorerApp = lazy(() => import('./explorer/ExplorerApp'))
const CommandApp = lazy(() => import('./command/CommandApp'))
const HostApp = lazy(() => import('./host/HostApp'))
const PreviewApp = lazy(() => import('./explorer/PreviewApp'))

const target = document.getElementById('app')

if (!target) {
  throw new Error('The application root element could not be found.')
}

try {
  const commandRuntime = hasCommandRuntime()
  const previewRuntime = !commandRuntime && hasPreviewRuntime()
  const explorerRuntime = !commandRuntime && !previewRuntime && hasExplorerRuntime()
  const hostRuntime = !commandRuntime && !previewRuntime && !explorerRuntime && hasHostRuntime()
  const settingsRuntime = !commandRuntime && !previewRuntime && !explorerRuntime && !hostRuntime && hasSettingsRuntime()
  const RootApp = commandRuntime
    ? CommandApp
    : previewRuntime
      ? PreviewApp
      : explorerRuntime
        ? ExplorerApp
        : hostRuntime
          ? HostApp
          : App
  const companionRuntime = commandRuntime || previewRuntime || explorerRuntime || hostRuntime
  createRoot(target).render(
    <React.StrictMode>
      {companionRuntime
        ? <Suspense fallback={<div className="startup-error">Opening window…</div>}><RootApp /></Suspense>
        : <RootApp settingsWindow={settingsRuntime} />}
    </React.StrictMode>,
  )
} catch (error) {
  console.error('Failed to mount the frontend application.', error)
  target.innerHTML = `
    <main class="startup-error" role="alert">
      <h1>SSH Man could not start</h1>
      <p>Quit and reopen the app. If the problem continues, launch it from a terminal to inspect the startup error.</p>
    </main>
  `
}
