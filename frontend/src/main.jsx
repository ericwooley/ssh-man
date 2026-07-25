import React, { lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import App from './app/App'
import { hasCommandRuntime } from './lib/commandApi'
import { hasExplorerRuntime } from './lib/explorerApi'
import { hasPreviewRuntime } from './lib/previewApi'
import { hasSettingsRuntime } from './lib/settingsApi'
import './app.css'

const ExplorerApp = lazy(() => import('./explorer/ExplorerApp'))
const CommandApp = lazy(() => import('./command/CommandApp'))
const PreviewApp = lazy(() => import('./explorer/PreviewApp'))

const target = document.getElementById('app')

if (!target) {
  throw new Error('The application root element could not be found.')
}

try {
  const commandRuntime = hasCommandRuntime()
  const previewRuntime = !commandRuntime && hasPreviewRuntime()
  const explorerRuntime = !commandRuntime && !previewRuntime && hasExplorerRuntime()
  const settingsRuntime = !commandRuntime && !previewRuntime && !explorerRuntime && hasSettingsRuntime()
  const RootApp = commandRuntime ? CommandApp : previewRuntime ? PreviewApp : explorerRuntime ? ExplorerApp : App
  createRoot(target).render(
    <React.StrictMode>
      {commandRuntime || previewRuntime || explorerRuntime
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
