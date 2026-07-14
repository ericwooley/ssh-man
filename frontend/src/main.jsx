import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './app/App'
import './app.css'

const target = document.getElementById('app')

if (!target) {
  throw new Error('The application root element could not be found.')
}

try {
  createRoot(target).render(
    <React.StrictMode>
      <App />
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
