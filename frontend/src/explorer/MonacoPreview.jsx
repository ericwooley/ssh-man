import { useEffect, useRef } from 'react'
import * as monaco from 'monaco-editor/editor/editor.api.js'
import EditorWorker from 'monaco-editor/editor/editor.worker.js?worker'
import 'monaco-editor/basic-languages/monaco.contribution.js'
import { initVimMode, VimMode } from 'monaco-vim'

if (typeof self !== 'undefined') {
  self.MonacoEnvironment = {
    getWorker() {
      return new EditorWorker()
    },
  }
}

export default function MonacoPreview({ content, language, label, modelKey, onChange, onSave, vimMode = false }) {
  const hostRef = useRef(null)
  const statusRef = useRef(null)
  const editorRef = useRef(null)
  const onChangeRef = useRef(onChange)
  const onSaveRef = useRef(onSave)
  onChangeRef.current = onChange
  onSaveRef.current = onSave

  useEffect(() => {
    if (!hostRef.current) return undefined
    const model = monaco.editor.createModel(content || '', language || 'plaintext')
    const editor = monaco.editor.create(hostRef.current, {
      model,
      readOnly: false,
      domReadOnly: false,
      automaticLayout: true,
      minimap: { enabled: false },
      fontSize: 13,
      lineHeight: 20,
      padding: { top: 14, bottom: 14 },
      renderLineHighlight: 'none',
      scrollBeyondLastLine: false,
      wordWrap: 'on',
      theme: 'vs-dark',
      ariaLabel: label || 'Remote file editor',
    })
    editorRef.current = editor
    const changeSubscription = model.onDidChangeContent(() => onChangeRef.current?.(model.getValue()))
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => onSaveRef.current?.(model.getValue()))
    return () => {
      changeSubscription.dispose()
      editorRef.current = null
      editor.dispose()
      model.dispose()
    }
  }, [label, language, modelKey])

  useEffect(() => {
    const editor = editorRef.current
    if (!editor || !vimMode) return undefined
    VimMode.Vim.defineEx('write', 'w', () => onSaveRef.current?.(editor.getValue()))
    const mode = initVimMode(editor, statusRef.current)
    return () => mode.dispose()
  }, [language, modelKey, vimMode])

  return (
    <div className="explorer-editor">
      <div className="explorer-monaco" ref={hostRef} />
      <div className={`explorer-vim-status${vimMode ? ' is-visible' : ''}`} ref={statusRef} />
    </div>
  )
}
