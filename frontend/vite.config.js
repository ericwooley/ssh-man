import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath } from 'node:url'

const monacoEditorAPI = fileURLToPath(new URL('./node_modules/monaco-editor/esm/vs/editor/editor.api.js', import.meta.url))

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      'monaco-editor/esm/vs/editor/editor.api': monacoEditorAPI,
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./vitest.setup.js'],
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
