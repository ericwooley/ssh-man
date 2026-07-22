import React, { lazy, Suspense } from 'react'
import { createRoot } from 'react-dom/client'
import App from './app/App'
import { hasExplorerRuntime } from './lib/explorerApi'
import './app.css'

const ExplorerApp = lazy(() => import('./explorer/ExplorerApp'))

const target = document.getElementById('app')

if (!target) {
  throw new Error('The application root element could not be found.')
}

try {
  const explorerRuntime = hasExplorerRuntime()
  const RootApp = explorerRuntime ? ExplorerApp : App
  createRoot(target).render(
    <React.StrictMode>
      {explorerRuntime ? <Suspense fallback={<div className="startup-error">Opening explorer…</div>}><RootApp /></Suspense> : <RootApp />}
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
