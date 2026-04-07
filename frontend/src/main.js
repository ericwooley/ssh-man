import './app.css'
import { mount } from 'svelte'
import App from './App.svelte'

const target = document.getElementById('app')

if (!target) {
  throw new Error('The application root element could not be found.')
}

let app

try {
  app = mount(App, {
    target,
  })
} catch (error) {
  console.error('Failed to mount the frontend application.', error)
  target.innerHTML = `
    <main class="app-shell">
      <div class="banner banner-danger" role="alert">
        The SSH Man UI failed to start. Launch the app from a terminal to inspect startup errors.
      </div>
    </main>
  `
}

export default app
