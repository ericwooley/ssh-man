import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowDownToLine,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  CheckSquare2,
  ChevronRight,
  CircleAlert,
  CloudUpload,
  ClipboardPaste,
  Copy,
  Eye,
  ExternalLink,
  File,
  FileCode2,
  FileText,
  Files,
  Folder,
  FolderPlus,
  FolderOpen,
  Home,
  Link2,
  LoaderCircle,
  Monitor,
  Pencil,
  RefreshCw,
  Save,
  Scissors,
  Server,
  Star,
  Trash2,
  X,
} from 'lucide-react'
import { MoonPixelsButton } from '../components/AppChrome'
import { ConfirmDialog } from '../components/Dialogs'
import * as defaultApi from '../lib/explorerApi'
import ExplorerContextMenu from './ExplorerContextMenu'
import PreviewContent from './PreviewContent'
import {
  addFavorite,
  readFavoritePaths,
  readVimMode,
  removeFavorite,
  writeFavoritePaths,
  writeVimMode,
} from './explorerPreferences'
import {
  formatFileSize,
  parentRemotePath,
} from './pathUtils'
import UploadTransfers from './UploadTransfers'
import {
  applyUploadProgress,
  completeUploadQueue,
  createUploadQueue,
  failUploadQueue,
  summarizeUploadQueue,
} from './uploadQueue'

function itemIcon(entry) {
  if (entry.kind === 'directory') return Folder
  if (/\.(md|markdown)$/i.test(entry.name)) return FileText
  if (/\.(html?|css|js|jsx|ts|tsx|go|py|rs|json|ya?ml)$/i.test(entry.name)) return FileCode2
  return File
}

function formatDate(value) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function previewTitle(preview) {
  if (!preview) return 'Preview'
  if (preview.kind === 'markdown') return 'Markdown preview'
  if (preview.kind === 'browser') return 'Browser preview'
  if (preview.kind === 'image') return 'Image preview'
  if (preview.kind === 'video') return 'Video preview'
  return 'Source preview'
}

function favoriteName(remotePath) {
  return remotePath.split('/').filter(Boolean).at(-1) || '/'
}

function uploadedFilesMessage(uploadedPaths, remoteDirectory) {
  if (uploadedPaths.length === 1) {
    const name = uploadedPaths[0].split('/').filter(Boolean).at(-1) || 'file'
    return `Uploaded ${name} to ${remoteDirectory}.`
  }
  return `Uploaded ${uploadedPaths.length} files to ${remoteDirectory}.`
}

function itemNames(failures) {
  const names = [...new Set(failures.map((failure) => failure?.name || 'this item'))]
  if (names.length === 1) return names[0]
  if (names.length === 2) return `${names[0]} and ${names[1]}`
  if (names.length === 3) return `${names[0]}, ${names[1]}, and ${names[2]}`
  const remaining = names.length - 3
  return `${names.slice(0, 3).join(', ')}, and ${remaining} ${remaining === 1 ? 'other' : 'others'}`
}

function uploadFailureGuidance(failures, remoteDirectory) {
  const names = itemNames(failures)
  const uniqueNameCount = new Set(failures.map((failure) => failure?.name || 'this item')).size
  const plural = uniqueNameCount > 1
  if (failures[0]?.code === 'exists') {
    return plural
      ? `Rename ${names} locally and drop them again; those names are already in ${remoteDirectory}.`
      : `Rename ${names} locally and drop it again; that name is already in ${remoteDirectory}.`
  }
  if (failures[0]?.code === 'directory') {
    return `Open ${names}, then drop the individual files you want to add.`
  }
  if (failures[0]?.code === 'permission') {
    return `${remoteDirectory} isn’t writable. Open a writable folder, then drop ${names} there.`
  }
  if (failures[0]?.code === 'local-permission') {
    return `Give SSH Man access to ${names}, or check ${plural ? 'their' : 'its'} file permissions, then drop ${plural ? 'them' : 'it'} again.`
  }
  if (failures[0]?.code === 'missing') {
    return `${names} ${plural ? 'aren’t' : 'isn’t'} on this Mac anymore. Save ${plural ? 'them' : 'it'} to a folder, then drop ${plural ? 'them' : 'it'} again.`
  }
  if (failures[0]?.code === 'permissions') {
    return `${names} couldn’t be uploaded safely, so nothing was added to ${remoteDirectory}. Uploads to this server aren’t supported yet.`
  }
  if (failures[0]?.code === 'incomplete') {
    return `${names} may be incomplete in ${remoteDirectory}. Delete ${plural ? 'them' : 'it'} there before trying again.`
  }
  if (failures[0]?.code === 'unsupported') {
    return `${names} can’t be uploaded. Drop a document, image, or other file instead.`
  }
  return `Try uploading ${names} to ${remoteDirectory} again.`
}

function uploadFeedback(result, remoteDirectory) {
  const uploaded = Array.isArray(result?.uploaded) ? result.uploaded : []
  const failures = Array.isArray(result?.failures) ? result.failures : []
  if (!failures.length) {
    return { notice: uploaded.length ? uploadedFilesMessage(uploaded, remoteDirectory) : '' }
  }
  const failureGroups = new Map()
  failures.forEach((failure) => {
    const code = failure?.code || 'failed'
    failureGroups.set(code, [...(failureGroups.get(code) || []), failure])
  })
  const prefix = uploaded.length ? `${uploadedFilesMessage(uploaded, remoteDirectory)} ` : ''
  const attention = failures.length > 1 ? `${failures.length} dropped items need attention. ` : ''
  const guidance = [...failureGroups.values()].map((group) => uploadFailureGuidance(group, remoteDirectory)).join(' ')
  return { error: `${prefix}${attention}${guidance}` }
}

function remoteName(remotePath) {
  return remotePath.split('/').filter(Boolean).at(-1) || remotePath
}

function validRemoteName(value) {
  const name = String(value || '').trim()
  return Boolean(name && name !== '.' && name !== '..' && !name.includes('/'))
}

function remotePathContains(parent, candidate) {
  const prefix = parent === '/' ? '/' : `${parent}/`
  return candidate === parent || candidate.startsWith(prefix)
}

function uploadDestinationOverlapsPaths(uploading, uploadDestination, remotePaths) {
  if (!uploading || !uploadDestination) return false
  return remotePaths.some((remotePath) => (
    remotePathContains(remotePath, uploadDestination)
    || remotePathContains(uploadDestination, remotePath)
  ))
}

export default function ExplorerApp({ api = defaultApi }) {
  const storage = window.localStorage
  const [server, setServer] = useState(null)
  const [phase, setPhase] = useState('connecting')
  const [homePath, setHomePath] = useState('')
  const [directory, setDirectory] = useState({ path: '', entries: [] })
  const [pathInput, setPathInput] = useState('')
  const [selectedPaths, setSelectedPaths] = useState([])
  const [preview, setPreview] = useState(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewMode, setPreviewMode] = useState('rendered')
  const [draftContent, setDraftContent] = useState('')
  const [dirty, setDirty] = useState(false)
  const [saving, setSaving] = useState(false)
  const [vimMode, setVimMode] = useState(() => readVimMode(storage))
  const [favoritePaths, setFavoritePaths] = useState([])
  const [error, setError] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [downloading, setDownloading] = useState(false)
  const [uploadQueue, setUploadQueue] = useState(null)
  const [detailTab, setDetailTab] = useState('preview')
  const [operation, setOperation] = useState('')
  const [notice, setNotice] = useState('')
  const [clipboard, setClipboard] = useState({ mode: '', paths: [] })
  const [contextMenu, setContextMenu] = useState(null)
  const [renaming, setRenaming] = useState(null)
  const [newFolderName, setNewFolderName] = useState(null)
  const [deleteRequest, setDeleteRequest] = useState(null)
  const [previewWindowPaths, setPreviewWindowPaths] = useState(() => new Set())
  const [previewWindowPendingPath, setPreviewWindowPendingPath] = useState('')
  const [historyVersion, setHistoryVersion] = useState(0)
  const historyRef = useRef([])
  const historyIndexRef = useRef(-1)
  const startedRef = useRef(false)
  const directoryPathRef = useRef('')
  const uploadingRef = useRef(false)
  const uploadDestinationRef = useRef('')
  const uploadIDRef = useRef(0)
  const previewWindowStateRequestsRef = useRef(new Map())
  const previewWindowOpenRef = useRef(false)
  const previewPopoutButtonRef = useRef(null)
  const previewFocusButtonRef = useRef(null)
  const previewTabRef = useRef(null)
  const transfersTabRef = useRef(null)

  useEffect(() => {
    directoryPathRef.current = directory.path
  }, [directory.path])

  const setPreviewWindowOpen = useCallback((remotePath, open) => {
    if (!remotePath) return
    previewWindowStateRequestsRef.current.delete(remotePath)
    setPreviewWindowPaths((current) => {
      const alreadyOpen = current.has(remotePath)
      if (alreadyOpen === open) return current
      const next = new Set(current)
      if (open) next.add(remotePath)
      else next.delete(remotePath)
      return next
    })
  }, [])

  useEffect(() => {
    if (!api.subscribePreviewWindowState) return undefined
    return api.subscribePreviewWindowState((state) => {
      if (!state?.remotePath || typeof state.open !== 'boolean') return
      setPreviewWindowOpen(state.remotePath, state.open)
    })
  }, [api, setPreviewWindowOpen])

  const confirmDiscard = useCallback(() => {
    if (!dirty) return true
    return window.confirm('Discard the unsaved changes to this remote file?')
  }, [dirty])

  const commitHistory = useCallback((remotePath, mode) => {
    if (mode === 'history') return
    if (mode === 'replace') {
      historyRef.current = [remotePath]
      historyIndexRef.current = 0
    } else {
      historyRef.current = historyRef.current.slice(0, historyIndexRef.current + 1).concat(remotePath)
      historyIndexRef.current = historyRef.current.length - 1
    }
    setHistoryVersion((value) => value + 1)
  }, [])

  const openDirectory = useCallback(async (remotePath, mode = 'push') => {
    if (operation) return null
    if (!confirmDiscard()) return null
    setError('')
    setNotice('')
    setContextMenu(null)
    setRenaming(null)
    setNewFolderName(null)
    setPhase('loading')
    try {
      const next = await api.listDirectory(remotePath)
      setDirectory(next)
      setPathInput(next.path)
      setSelectedPaths([])
      setPreview(null)
      setDraftContent('')
      setDirty(false)
      setPhase('ready')
      commitHistory(next.path, mode)
      if (server?.id) storage.setItem(`ssh-man:explorer:path:${server.id}`, next.path)
      return next
    } catch (nextError) {
      setError(nextError.message || 'The remote folder could not be opened.')
      setPhase('error')
      return null
    }
  }, [api, commitHistory, confirmDiscard, operation, server?.id, storage])

  const connect = useCallback(async (secret = '') => {
    setError('')
    setPhase('connecting')
    try {
      const result = await api.connect(secret)
      if (result.needsPassphrase) {
        setPhase('locked')
        return
      }
      setHomePath(result.homePath)
      setPassphrase('')
      const savedPath = server?.id ? storage.getItem(`ssh-man:explorer:path:${server.id}`) : ''
      const opened = await openDirectory(savedPath || result.homePath, 'replace')
      if (!opened && savedPath && savedPath !== result.homePath) {
        await openDirectory(result.homePath, 'replace')
      }
    } catch (nextError) {
      setError(nextError.message || 'The SSH connection could not be opened.')
      setPhase('error')
    }
  }, [api, openDirectory, server?.id, storage])

  useEffect(() => {
    if (startedRef.current) return
    startedRef.current = true
    let active = true
    async function start() {
      try {
        const state = await api.initialState()
        if (!active) return
        setServer(state.server)
        setFavoritePaths(readFavoritePaths(storage, state.server.id))
        document.title = `${state.server.name} — SSH Man Explorer`
        const result = await api.connect('')
        if (!active) return
        if (result.needsPassphrase) {
          setPhase('locked')
          return
        }
        setHomePath(result.homePath)
        const savedPath = storage.getItem(`ssh-man:explorer:path:${state.server.id}`)
        let next
        try {
          next = await api.listDirectory(savedPath || result.homePath)
        } catch (nextError) {
          if (!savedPath || savedPath === result.homePath) throw nextError
          next = await api.listDirectory(result.homePath)
        }
        if (!active) return
        setDirectory(next)
        setPathInput(next.path)
        setPhase('ready')
        commitHistory(next.path, 'replace')
      } catch (nextError) {
        if (!active) return
        setError(nextError.message || 'The server explorer could not connect.')
        setPhase('error')
      }
    }
    start()
    return () => { active = false }
  }, [api, commitHistory, storage])

  const selectedEntries = useMemo(() => directory.entries.filter((entry) => selectedPaths.includes(entry.path)), [directory.entries, selectedPaths])
  const selectedEntry = selectedEntries.length === 1 ? selectedEntries[0] : null
  const editablePreview = Boolean(preview?.revision && preview?.content != null && !preview.truncated)
  const closeContextMenu = useCallback(() => setContextMenu(null), [])

  const refreshDirectory = useCallback(async (preferredSelection = []) => {
    const next = await api.listDirectory(directory.path)
    const visiblePaths = new Set(next.entries.map((entry) => entry.path))
    setDirectory(next)
    setSelectedPaths(preferredSelection.filter((remotePath) => visiblePaths.has(remotePath)))
    return next
  }, [api, directory.path])

  async function reconcileDirectory(preferredSelection = []) {
    try {
      await refreshDirectory(preferredSelection)
    } catch {
      // Keep the original operation error visible if the follow-up refresh also fails.
    }
  }

  function clearClipboardForPaths(paths) {
    setClipboard((current) => current.paths.some((clipboardPath) => (
      paths.some((remotePath) => remotePathContains(remotePath, clipboardPath))
    )) ? { mode: '', paths: [] } : current)
  }

  function blockUploadDestinationChange(paths) {
    const uploadDestination = uploadDestinationRef.current
    if (!uploadDestinationOverlapsPaths(uploadingRef.current, uploadDestination, paths)) return false
    closeContextMenu()
    setError('')
    setNotice(`Finish the upload to ${uploadDestination} before renaming or deleting items there.`)
    return true
  }

  const saveCurrentFile = useCallback(async (contentOverride) => {
    if (!editablePreview || saving) return
    const nextContent = typeof contentOverride === 'string' ? contentOverride : draftContent
    if (nextContent === preview.content) {
      setDirty(false)
      return
    }
    setSaving(true)
    setError('')
    setNotice('')
    try {
      const saved = await api.saveFile(preview.path, nextContent, preview.revision)
      setPreview(saved)
      setDraftContent(saved.content ?? nextContent)
      setDirty(false)
      setNotice(`Saved ${saved.name}.`)
    } catch (nextError) {
      setError(nextError.message || 'The remote file could not be saved.')
    } finally {
      setSaving(false)
    }
  }, [api, draftContent, editablePreview, preview, saving])

  const uploadDroppedFiles = useCallback(async (_x, _y, localPaths) => {
    const files = (localPaths || []).filter((localPath) => (
      typeof localPath === 'string' && localPath.trim() !== ''
    ))
    if (phase !== 'ready' || !directory.path || !files.length) return
    if (uploadingRef.current) {
      setError('')
      setNotice(`Still uploading to ${uploadDestinationRef.current || directory.path}. These files weren’t added — drop them again when it finishes.`)
      return
    }
    const remoteDirectory = directory.path
    const uploadID = uploadIDRef.current + 1
    uploadIDRef.current = uploadID
    uploadingRef.current = true
    uploadDestinationRef.current = remoteDirectory
    setUploadQueue(createUploadQueue(files, remoteDirectory, uploadID))
    setDetailTab('transfers')
    setError('')
    setNotice('')
    let uploadError = null
    let uploadedCount = 0
    try {
      const result = await api.uploadFiles(uploadID, remoteDirectory, files)
      uploadedCount = Array.isArray(result?.uploaded) ? result.uploaded.length : 0
      setUploadQueue((current) => current?.id === uploadID ? completeUploadQueue(current, result) : current)
      const feedback = uploadFeedback(result, remoteDirectory)
      if (feedback.error) {
        uploadError = new Error(feedback.error)
        setError(feedback.error)
      } else if (feedback.notice) {
        setNotice(feedback.notice)
      }
    } catch (nextError) {
      uploadError = nextError
      setUploadQueue((current) => current?.id === uploadID ? failUploadQueue(current) : current)
      setError('The files could not be uploaded. Check the connection and try again.')
    } finally {
      uploadingRef.current = false
      uploadDestinationRef.current = ''
    }
    if (directoryPathRef.current === remoteDirectory) {
      setUploadQueue((current) => current?.id === uploadID ? { ...current, refreshStatus: 'refreshing' } : current)
      try {
        const refreshed = await api.listDirectory(remoteDirectory)
        setDirectory((current) => current.path === remoteDirectory ? refreshed : current)
        setUploadQueue((current) => current?.id === uploadID ? { ...current, refreshStatus: 'completed' } : current)
      } catch (nextError) {
        setUploadQueue((current) => current?.id === uploadID ? { ...current, refreshStatus: 'failed' } : current)
        if (uploadedCount > 0 && uploadError) {
          setNotice('')
          setError(`${uploadError.message} Reopen ${remoteDirectory} to see the files that were added.`)
        } else if (!uploadError) {
          setNotice('')
          setError(`Your files uploaded to ${remoteDirectory}, but it could not be refreshed. Reopen it to see them.`)
        }
      }
    }
  }, [api, directory.path, phase])

  useEffect(() => {
    if (!api.subscribeFileDrop) return undefined
    return api.subscribeFileDrop(uploadDroppedFiles)
  }, [api, uploadDroppedFiles])

  useEffect(() => {
    if (!api.subscribeUploadProgress) return undefined
    return api.subscribeUploadProgress((progress) => {
      setUploadQueue((current) => applyUploadProgress(current, progress))
    })
  }, [api])

  useEffect(() => {
    if (!dirty) return undefined
    const handleBeforeUnload = (event) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [dirty])

  useEffect(() => {
    let active = true
    setPreview(null)
    setDraftContent('')
    setDirty(false)
    if (!selectedEntry || selectedEntry.kind === 'directory') return () => { active = false }
    setPreviewLoading(true)
    api.previewFile(selectedEntry.path)
      .then((next) => {
        if (!active) return
        setPreview(next)
        setDraftContent(next.content ?? '')
        setDirty(false)
        setPreviewMode(next.kind === 'text' ? 'source' : 'rendered')
      })
      .catch((nextError) => {
        if (active) setError(nextError.message || 'The file preview could not be loaded.')
      })
      .finally(() => {
        if (active) setPreviewLoading(false)
      })
    return () => { active = false }
  }, [api, selectedEntry])

  useEffect(() => {
    if (!preview?.path || !api.previewWindowState) return undefined
    let active = true
    const remotePath = preview.path
    const request = Symbol(remotePath)
    previewWindowStateRequestsRef.current.set(remotePath, request)
    api.previewWindowState(preview.path)
      .then((state) => {
        const requestIsCurrent = previewWindowStateRequestsRef.current.get(remotePath) === request
        if (active && requestIsCurrent) setPreviewWindowOpen(remotePath, Boolean(state?.open))
      })
      .catch(() => {})
    return () => {
      active = false
      if (previewWindowStateRequestsRef.current.get(remotePath) === request) {
        previewWindowStateRequestsRef.current.delete(remotePath)
      }
    }
  }, [api, preview?.path, setPreviewWindowOpen])

  function selectEntry(event, entry) {
    if (operation) return
    if (!selectedPaths.includes(entry.path) && !confirmDiscard()) return
    closeContextMenu()
    setRenaming(null)
    if (event.metaKey || event.ctrlKey) {
      setSelectedPaths((current) => current.includes(entry.path)
        ? current.filter((item) => item !== entry.path)
        : current.concat(entry.path))
    } else {
      setSelectedPaths([entry.path])
    }
  }

  function selectedPathsForEntry(entry) {
    return selectedPaths.includes(entry.path) ? selectedPaths : [entry.path]
  }

  function openEntryContextMenu(event, entry) {
    event.preventDefault()
    event.stopPropagation()
    if (!selectedPaths.includes(entry.path)) {
      if (!confirmDiscard()) return
      setSelectedPaths([entry.path])
    }
    setContextMenu({
      entry,
      paths: selectedPathsForEntry(entry),
      position: { x: event.clientX, y: event.clientY },
    })
  }

  function openBackgroundContextMenu(event) {
    if (event.target.closest('.explorer-row')) return
    event.preventDefault()
    setContextMenu({
      entry: null,
      paths: [],
      position: { x: event.clientX, y: event.clientY },
    })
  }

  async function navigateHistory(direction) {
    if (operation) return
    const nextIndex = historyIndexRef.current + direction
    const remotePath = historyRef.current[nextIndex]
    if (!remotePath) return
    historyIndexRef.current = nextIndex
    setHistoryVersion((value) => value + 1)
    await openDirectory(remotePath, 'history')
  }

  async function downloadPaths(paths = selectedPaths) {
    if (!paths.length) return
    setDownloading(true)
    setError('')
    try {
      const downloadedPaths = await api.download(paths)
      if (downloadedPaths?.length) {
        setNotice(`Downloaded ${downloadedPaths.length} item${downloadedPaths.length === 1 ? '' : 's'} to ${parentRemotePath(downloadedPaths[0])}.`)
      }
    } catch (nextError) {
      setError(nextError.message || 'The selected items could not be downloaded.')
    } finally {
      setDownloading(false)
    }
  }

  function rememberSelection(mode, paths = selectedPaths) {
    if (!paths.length) return
    setClipboard({ mode, paths: [...paths] })
    setNotice(`${mode === 'move' ? 'Cut' : 'Copied'} ${paths.length} item${paths.length === 1 ? '' : 's'}. Open a folder and paste to place ${paths.length === 1 ? 'it' : 'them'}.`)
    setError('')
  }

  async function pasteClipboard(destinationDirectory = directory.path) {
    if (!clipboard.paths.length || operation || !confirmDiscard()) return
    const clipboardMode = clipboard.mode
    setOperation('paste')
    setError('')
    setNotice('')
    try {
      const pastedPaths = clipboardMode === 'move'
        ? await api.move(clipboard.paths, destinationDirectory)
        : await api.copy(clipboard.paths, destinationDirectory)
      if (clipboardMode === 'move') setClipboard({ mode: '', paths: [] })
      const selection = destinationDirectory === directory.path ? pastedPaths : []
      await refreshDirectory(selection)
      setNotice(`${clipboardMode === 'move' ? 'Moved' : 'Pasted'} ${pastedPaths.length} item${pastedPaths.length === 1 ? '' : 's'} into ${remoteName(destinationDirectory)}.`)
    } catch (nextError) {
      if (clipboardMode === 'move') setClipboard({ mode: '', paths: [] })
      await reconcileDirectory([])
      setError(nextError.message || 'The items could not be pasted here.')
    } finally {
      setOperation('')
    }
  }

  async function duplicatePaths(paths = selectedPaths) {
    if (!paths.length || operation || !confirmDiscard()) return
    setOperation('duplicate')
    setError('')
    setNotice('')
    try {
      const duplicatedPaths = await api.copy(paths, directory.path)
      await refreshDirectory(duplicatedPaths)
      setNotice(`Duplicated ${duplicatedPaths.length} item${duplicatedPaths.length === 1 ? '' : 's'}.`)
    } catch (nextError) {
      await reconcileDirectory([])
      setError(nextError.message || 'The selected items could not be duplicated.')
    } finally {
      setOperation('')
    }
  }

  function beginRename(entry = selectedEntry) {
    if (!entry || operation || blockUploadDestinationChange([entry.path]) || !confirmDiscard()) return
    closeContextMenu()
    setNewFolderName(null)
    setSelectedPaths([entry.path])
    setRenaming({ path: entry.path, name: entry.name })
  }

  async function commitRename() {
    if (!renaming || operation || blockUploadDestinationChange([renaming.path])) return
    const name = renaming.name.trim()
    if (name === remoteName(renaming.path)) {
      setRenaming(null)
      return
    }
    if (!validRemoteName(name)) {
      setError('Use a file name without slashes.')
      return
    }
    setOperation('rename')
    setError('')
    setNotice('')
    try {
      const renamedPath = await api.rename(renaming.path, name)
      const originalPath = renaming.path
      setRenaming(null)
      setClipboard((current) => ({
        ...current,
        paths: current.paths.map((clipboardPath) => remotePathContains(originalPath, clipboardPath)
          ? `${renamedPath}${clipboardPath.slice(originalPath.length)}`
          : clipboardPath),
      }))
      await refreshDirectory([renamedPath])
      setNotice(`Renamed to ${name}.`)
    } catch (nextError) {
      setError(nextError.message || 'The item could not be renamed.')
    } finally {
      setOperation('')
    }
  }

  function beginCreateFolder() {
    if (operation || !confirmDiscard()) return
    closeContextMenu()
    setRenaming(null)
    setSelectedPaths([])
    setNewFolderName('untitled folder')
  }

  async function commitCreateFolder() {
    if (newFolderName == null || operation) return
    const name = newFolderName.trim()
    if (!validRemoteName(name)) {
      setError('Use a folder name without slashes.')
      return
    }
    setOperation('create-folder')
    setError('')
    setNotice('')
    try {
      const folderPath = await api.createFolder(directory.path, name)
      setNewFolderName(null)
      await refreshDirectory([folderPath])
      setNotice(`Created ${name}.`)
    } catch (nextError) {
      setError(nextError.message || 'The folder could not be created.')
    } finally {
      setOperation('')
    }
  }

  function requestDelete(paths = selectedPaths) {
    if (!paths.length || operation || blockUploadDestinationChange(paths) || !confirmDiscard()) return
    const names = paths.map(remoteName)
    const oneItem = paths.length === 1
    setDeleteRequest({
      paths: [...paths],
      title: oneItem ? `Delete ${names[0]} permanently?` : `Delete ${paths.length} items permanently?`,
      description: oneItem
        ? `This permanently removes ${names[0]} from ${server?.name || 'the server'}. You can’t undo this action.`
        : `This permanently removes the selected items from ${server?.name || 'the server'}. You can’t undo this action.`,
      confirmLabel: 'Delete permanently',
    })
  }

  async function confirmDelete() {
    if (!deleteRequest || operation) return
    if (blockUploadDestinationChange(deleteRequest.paths)) {
      setDeleteRequest(null)
      return
    }
    setOperation('delete')
    setError('')
    setNotice('')
    try {
      await api.deleteItems(deleteRequest.paths)
      const deletedCount = deleteRequest.paths.length
      clearClipboardForPaths(deleteRequest.paths)
      setDeleteRequest(null)
      await refreshDirectory([])
      setNotice(`Deleted ${deletedCount} item${deletedCount === 1 ? '' : 's'} permanently.`)
    } catch (nextError) {
      clearClipboardForPaths(deleteRequest.paths)
      setDeleteRequest(null)
      await reconcileDirectory([])
      setError(nextError.message || 'The selected items could not be deleted.')
    } finally {
      setOperation('')
    }
  }

  async function copyPathsToSystemClipboard(paths) {
    try {
      if (!navigator.clipboard?.writeText) throw new Error('Clipboard access is unavailable.')
      await navigator.clipboard.writeText(paths.join('\n'))
      setNotice(`Copied ${paths.length === 1 ? 'path' : `${paths.length} paths`} to the clipboard.`)
      setError('')
    } catch (nextError) {
      setError(nextError.message || 'The paths could not be copied.')
    }
  }

  async function openRemoteItem(entry = selectedEntry) {
    if (!entry || operation) return
    if (entry.kind === 'directory') {
      await openDirectory(entry.path)
      return
    }
    if (dirty && preview?.path === entry.path) {
      setError('Save this file before opening its preview in a new window.')
      return
    }
    setError('')
    setPreviewWindowPendingPath(entry.path)
    try {
      const state = await api.openPreview(entry.path)
      setPreviewWindowOpen(state?.remotePath || entry.path, state?.open !== false)
    } catch (nextError) {
      setError(nextError.message || 'The preview window could not be opened.')
    } finally {
      setPreviewWindowPendingPath((current) => current === entry.path ? '' : current)
    }
  }

  async function openPreviewWindow() {
    if (!preview?.path || dirty) return
    const remotePath = preview.path
    setError('')
    setPreviewWindowPendingPath(remotePath)
    try {
      const state = await api.openPreview(remotePath)
      setPreviewWindowOpen(state?.remotePath || remotePath, state?.open !== false)
    } catch (nextError) {
      setError(nextError.message || 'The preview window could not be opened.')
    } finally {
      setPreviewWindowPendingPath((current) => current === remotePath ? '' : current)
    }
  }

  async function focusPreviewWindow() {
    if (!preview?.path) return
    const remotePath = preview.path
    setError('')
    setPreviewWindowPendingPath(remotePath)
    try {
      const state = await api.focusPreview(remotePath)
      setPreviewWindowOpen(state?.remotePath || remotePath, state?.open !== false)
    } catch (nextError) {
      setError(nextError.message || 'The preview window could not be focused.')
    } finally {
      setPreviewWindowPendingPath((current) => current === remotePath ? '' : current)
    }
  }

  function toggleVimMode(event) {
    const enabled = event.target.checked
    setVimMode(enabled)
    writeVimMode(storage, enabled)
  }

  function toggleCurrentFavorite() {
    if (!server?.id || !directory.path) return
    const next = favoritePaths.includes(directory.path)
      ? removeFavorite(favoritePaths, directory.path)
      : addFavorite(favoritePaths, directory.path)
    setFavoritePaths(writeFavoritePaths(storage, server.id, next))
  }

  function removeFavoritePath(remotePath) {
    if (!server?.id) return
    setFavoritePaths(writeFavoritePaths(storage, server.id, removeFavorite(favoritePaths, remotePath)))
  }

  function selectAllEntries() {
    if (!confirmDiscard()) return
    setSelectedPaths(directory.entries.map((entry) => entry.path))
  }

  function contextMenuItems() {
    if (!contextMenu) return []
    const busy = Boolean(operation || downloading)
    const count = contextMenu.paths.length
    const oneEntry = count === 1 ? contextMenu.entry : null
    const command = /Mac|iPhone|iPad/i.test(navigator.userAgentData?.platform || navigator.platform || '') ? '⌘' : 'Ctrl+'

    if (!count) {
      return [
        { label: 'New Folder', icon: FolderPlus, shortcut: `${command}⇧N`, disabled: busy, action: beginCreateFolder },
        {
          label: `Paste${clipboard.paths.length > 1 ? ` ${clipboard.paths.length} Items` : clipboard.paths.length ? ' Item' : ''}`,
          icon: ClipboardPaste,
          shortcut: `${command}V`,
          disabled: busy || !clipboard.paths.length,
          action: () => pasteClipboard(directory.path),
        },
        { separator: true },
        { label: 'Select All', icon: CheckSquare2, shortcut: `${command}A`, disabled: busy || !directory.entries.length, action: selectAllEntries },
        { label: 'Refresh', icon: RefreshCw, shortcut: `${command}R`, disabled: busy, action: () => openDirectory(directory.path, 'history') },
      ]
    }

    const items = []
    if (oneEntry) {
      items.push({
        label: oneEntry.kind === 'directory' ? 'Open' : 'Open in New Window',
        icon: oneEntry.kind === 'directory' ? FolderOpen : ExternalLink,
        shortcut: `${command}O`,
        disabled: busy,
        action: () => openRemoteItem(oneEntry),
      })
    }
    items.push(
      { label: count === 1 ? 'Download' : `Download ${count} Items`, icon: ArrowDownToLine, disabled: busy, action: () => downloadPaths(contextMenu.paths) },
      { separator: true },
      { label: count === 1 ? 'Copy' : `Copy ${count} Items`, icon: Copy, shortcut: `${command}C`, disabled: busy, action: () => rememberSelection('copy', contextMenu.paths) },
      { label: count === 1 ? 'Cut' : `Cut ${count} Items`, icon: Scissors, shortcut: `${command}X`, disabled: busy, action: () => rememberSelection('move', contextMenu.paths) },
    )
    if (oneEntry?.kind === 'directory') {
      items.push({
        label: `Paste${clipboard.paths.length > 1 ? ` ${clipboard.paths.length} Items` : clipboard.paths.length ? ' Item' : ''} Into Folder`,
        icon: ClipboardPaste,
        disabled: busy || !clipboard.paths.length,
        action: () => pasteClipboard(oneEntry.path),
      })
    }
    items.push(
      { label: count === 1 ? 'Duplicate' : `Duplicate ${count} Items`, icon: Files, shortcut: `${command}D`, disabled: busy, action: () => duplicatePaths(contextMenu.paths) },
    )
    if (oneEntry) {
      items.push({ label: 'Rename', icon: Pencil, shortcut: 'Enter', disabled: busy, action: () => beginRename(oneEntry) })
    }
    items.push(
      { label: count === 1 ? 'Delete Permanently…' : `Delete ${count} Items Permanently…`, icon: Trash2, shortcut: `${command}⌫`, danger: true, disabled: busy, action: () => requestDelete(contextMenu.paths) },
      { separator: true },
      { label: count === 1 ? 'Copy Path' : 'Copy Paths', icon: Link2, disabled: busy, action: () => copyPathsToSystemClipboard(contextMenu.paths) },
    )
    return items
  }

  useEffect(() => {
    function handleExplorerShortcut(event) {
      if (phase !== 'ready' || operation || deleteRequest || contextMenu || event.defaultPrevented) return
      const target = event.target
      if (target instanceof Element && target.closest('input, textarea, [contenteditable="true"], dialog, .explorer-monaco')) return

      const command = event.metaKey || event.ctrlKey
      const key = event.key.toLowerCase()
      if (command && key === 'c' && selectedPaths.length) {
        event.preventDefault()
        rememberSelection('copy')
      } else if (command && key === 'x' && selectedPaths.length) {
        event.preventDefault()
        rememberSelection('move')
      } else if (command && key === 'v' && clipboard.paths.length) {
        event.preventDefault()
        pasteClipboard()
      } else if (command && key === 'a') {
        event.preventDefault()
        selectAllEntries()
      } else if (command && key === 'd' && selectedPaths.length) {
        event.preventDefault()
        duplicatePaths()
      } else if (command && event.shiftKey && key === 'n') {
        event.preventDefault()
        beginCreateFolder()
      } else if (command && key === 'o' && selectedEntry) {
        event.preventDefault()
        openRemoteItem()
      } else if (command && key === 'r') {
        event.preventDefault()
        if (confirmDiscard()) refreshDirectory(selectedPaths)
      } else if ((event.key === 'Enter' || event.key === 'F2') && selectedEntry) {
        event.preventDefault()
        beginRename()
      } else if ((event.key === 'Delete' || (command && event.key === 'Backspace')) && selectedPaths.length) {
        event.preventDefault()
        requestDelete()
      }
    }

    window.addEventListener('keydown', handleExplorerShortcut)
    return () => window.removeEventListener('keydown', handleExplorerShortcut)
  })

  const canGoBack = historyVersion >= 0 && historyIndexRef.current > 0
  const canGoForward = historyIndexRef.current >= 0 && historyIndexRef.current < historyRef.current.length - 1
  const currentFolderIsFavorite = favoritePaths.includes(directory.path)
  const previewOpenInWindow = Boolean(preview?.path && previewWindowPaths.has(preview.path))
  const previewWindowPending = previewWindowPendingPath === preview?.path
  const sourceEditorVisible = !previewOpenInWindow && (preview?.kind === 'text' || (previewMode === 'source' && ['markdown', 'browser'].includes(preview?.kind)))
  const uploadSummary = useMemo(() => summarizeUploadQueue(uploadQueue), [uploadQueue])

  useEffect(() => {
    const wasOpen = previewWindowOpenRef.current
    previewWindowOpenRef.current = previewOpenInWindow
    if (!document.hasFocus()) return
    if (!wasOpen && previewOpenInWindow) previewFocusButtonRef.current?.focus()
    if (wasOpen && !previewOpenInWindow) previewPopoutButtonRef.current?.focus()
  }, [previewOpenInWindow])

  return (
    <div className="explorer-shell">
      <header className="explorer-toolbar">
        <div className="explorer-toolbar__nav" aria-label="Folder navigation">
          <button type="button" aria-label="Back" disabled={!canGoBack || Boolean(operation)} onClick={() => navigateHistory(-1)}><ArrowLeft /></button>
          <button type="button" aria-label="Forward" disabled={!canGoForward || Boolean(operation)} onClick={() => navigateHistory(1)}><ArrowRight /></button>
          <button type="button" aria-label="Up one folder" disabled={!directory.path || directory.path === '/' || Boolean(operation)} onClick={() => openDirectory(parentRemotePath(directory.path))}><ArrowUp /></button>
          <button type="button" aria-label="Refresh folder" disabled={phase !== 'ready' || Boolean(operation)} onClick={() => openDirectory(directory.path, 'history')}><RefreshCw /></button>
        </div>
        <form className="explorer-path" onSubmit={(event) => { event.preventDefault(); openDirectory(pathInput) }}>
          <Server aria-hidden="true" />
          <input aria-label="Remote path" value={pathInput} disabled={Boolean(operation)} onChange={(event) => setPathInput(event.target.value)} spellCheck="false" />
        </form>
        <button className="explorer-download" type="button" disabled={!selectedPaths.length || downloading || Boolean(operation)} onClick={() => downloadPaths()}>
          {downloading ? <LoaderCircle className="spin" /> : <ArrowDownToLine />}
          Download{selectedPaths.length > 1 ? ` (${selectedPaths.length})` : ''}
        </button>
      </header>

      <div className="explorer-workspace">
        <aside className="explorer-sidebar">
          <div className="explorer-server">
            <span>{server?.name?.slice(0, 2).toUpperCase() || 'SH'}</span>
            <div><strong>{server?.name || 'Server'}</strong><small>{server ? `${server.username}@${server.host}` : 'Connecting…'}</small></div>
          </div>
          <nav aria-label="Remote locations">
            <span>Locations</span>
            <button type="button" className={directory.path === homePath ? 'is-current' : ''} disabled={!homePath || Boolean(operation)} onClick={() => openDirectory(homePath)}><Home /> Home</button>
            <button type="button" className={directory.path === '/' ? 'is-current' : ''} disabled={Boolean(operation)} onClick={() => openDirectory('/')}><Monitor /> File system</button>
          </nav>
          {favoritePaths.length ? (
            <nav className="explorer-favorites" aria-label="Favorite folders">
              <span>Favorites</span>
              {favoritePaths.map((remotePath) => (
                <div className="explorer-favorite" key={remotePath}>
                  <button type="button" className={directory.path === remotePath ? 'is-current' : ''} aria-label={`Open favorite ${favoriteName(remotePath)}`} title={remotePath} disabled={Boolean(operation)} onClick={() => openDirectory(remotePath)}><Star /> <span>{favoriteName(remotePath)}</span></button>
                  <button type="button" className="explorer-favorite__remove" aria-label={`Remove favorite ${favoriteName(remotePath)}`} onClick={() => removeFavoritePath(remotePath)}><X /></button>
                </div>
              ))}
            </nav>
          ) : null}
          <div className="explorer-sidebar__footer">
            <MoonPixelsButton
              className="explorer-moonpixels"
              showLabel
              onClick={() => api.openExternalURL('https://moonpixels.tech')}
            />
            <div className="explorer-connection"><span className={phase === 'ready' ? 'is-online' : ''} />{phase === 'ready' ? 'Connected over SFTP' : 'SSH connection'}</div>
          </div>
        </aside>

        <main
          className="explorer-browser"
          style={{ '--wails-drop-target': phase === 'ready' ? 'drop' : undefined }}
          aria-busy={phase === 'loading' || phase === 'connecting' || Boolean(operation)}
          onContextMenu={openBackgroundContextMenu}
          onClick={(event) => {
            if (event.target.closest('.explorer-row') || !selectedPaths.length) return
            if (confirmDiscard()) setSelectedPaths([])
          }}
        >
          <div className="explorer-location-title">
            <FolderOpen />
            <strong>{directory.path ? directory.path.split('/').filter(Boolean).at(-1) || '/' : 'Files'}</strong>
            <span>{directory.entries.length} items</span>
            <button
              type="button"
              className={currentFolderIsFavorite ? 'is-favorite' : ''}
              aria-label={currentFolderIsFavorite ? 'Remove current folder from favorites' : 'Favorite current folder'}
              title={currentFolderIsFavorite ? 'Remove from favorites' : 'Add to favorites'}
              disabled={!directory.path || phase !== 'ready'}
              onClick={toggleCurrentFavorite}
            ><Star fill={currentFolderIsFavorite ? 'currentColor' : 'none'} /></button>
          </div>
          <div className="explorer-columns" aria-hidden="true"><span>Name</span><span>Size</span><span>Modified</span></div>
          {phase === 'loading' || phase === 'connecting' ? <div className="explorer-state"><LoaderCircle className="spin" /><span>{phase === 'connecting' ? 'Connecting over SSH…' : 'Opening folder…'}</span></div> : null}
          {phase === 'error' ? (
            <div className="explorer-state is-error"><CircleAlert /><strong>Couldn’t open this location</strong><span>{error}</span><button type="button" onClick={() => connect('')}>Reconnect</button></div>
          ) : null}
          {phase === 'ready' && !directory.entries.length && newFolderName == null ? <div className="explorer-state"><Folder /><strong>This folder is empty</strong><span>Right-click to create a folder or paste items.</span></div> : null}
          {phase === 'ready' && (directory.entries.length || newFolderName != null) ? (
            <div className="explorer-list" role="listbox" aria-label={`Files in ${directory.path}`} aria-multiselectable="true">
              {newFolderName != null ? (
                <div className="explorer-row is-selected" role="option" aria-selected="true">
                  <span className="explorer-file-icon is-directory"><Folder /></span>
                  <span className="explorer-file-name">
                    <input
                      autoFocus
                      className="explorer-rename-input"
                      aria-label="New folder name"
                      value={newFolderName}
                      onChange={(event) => setNewFolderName(event.target.value)}
                      onFocus={(event) => event.target.select()}
                      onBlur={() => commitCreateFolder()}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') {
                          event.preventDefault()
                          commitCreateFolder()
                        } else if (event.key === 'Escape') {
                          event.preventDefault()
                          setNewFolderName(null)
                        }
                      }}
                    />
                  </span>
                  <span>—</span>
                  <span>—</span>
                </div>
              ) : null}
              {directory.entries.map((entry) => {
                const Icon = itemIcon(entry)
                const selected = selectedPaths.includes(entry.path)
                const isRenaming = renaming?.path === entry.path
                const cut = clipboard.mode === 'move' && clipboard.paths.includes(entry.path)
                return (
                  <div
                    role="option"
                    tabIndex="0"
                    aria-disabled={Boolean(operation)}
                    aria-selected={selected}
                    className={`explorer-row${selected ? ' is-selected' : ''}${cut ? ' is-cut' : ''}`}
                    key={entry.path}
                    onClick={(event) => selectEntry(event, entry)}
                    onDoubleClick={() => openRemoteItem(entry)}
                    onContextMenu={(event) => openEntryContextMenu(event, entry)}
                  >
                    <span className={`explorer-file-icon is-${entry.kind}`}><Icon /></span>
                    <span className="explorer-file-name">
                      {isRenaming ? (
                        <input
                          autoFocus
                          className="explorer-rename-input"
                          aria-label={`Rename ${entry.name}`}
                          value={renaming.name}
                          onClick={(event) => event.stopPropagation()}
                          onChange={(event) => setRenaming((current) => ({ ...current, name: event.target.value }))}
                          onFocus={(event) => event.target.select()}
                          onBlur={() => commitRename()}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') {
                              event.preventDefault()
                              commitRename()
                            } else if (event.key === 'Escape') {
                              event.preventDefault()
                              setRenaming(null)
                            }
                          }}
                        />
                      ) : (
                        <>{entry.name}{entry.kind === 'directory' ? <ChevronRight /> : null}</>
                      )}
                    </span>
                    <span>{entry.kind === 'directory' ? '—' : formatFileSize(entry.size)}</span>
                    <span>{formatDate(entry.modifiedAt)}</span>
                  </div>
                )
              })}
            </div>
          ) : null}
          <div className="explorer-drop-overlay" aria-hidden="true">
            <CloudUpload />
            <strong>Drop files to upload</strong>
            <span>Add files to {directory.path || 'this folder'}</span>
          </div>
        </main>

        <aside className="explorer-preview">
          <div className="explorer-preview__header">
            <div className="explorer-preview-tabs" role="tablist" aria-label="Preview and transfers">
              <button
                type="button"
                role="tab"
                ref={previewTabRef}
                id="explorer-preview-tab"
                aria-controls="explorer-preview-panel"
                aria-selected={detailTab === 'preview'}
                tabIndex={detailTab === 'preview' ? 0 : -1}
                className={detailTab === 'preview' ? 'is-current' : ''}
                title={previewTitle(preview)}
                onClick={() => setDetailTab('preview')}
                onKeyDown={(event) => {
                  if (event.key !== 'ArrowRight' || !uploadQueue) return
                  event.preventDefault()
                  setDetailTab('transfers')
                  transfersTabRef.current?.focus()
                }}
              ><Eye /><span>Preview</span><small>{previewTitle(preview)}</small>{dirty ? <i>Edited</i> : null}</button>
              {uploadQueue ? (
                <button
                  type="button"
                  role="tab"
                  ref={transfersTabRef}
                  id="explorer-transfers-tab"
                  aria-controls="explorer-transfers-panel"
                  aria-selected={detailTab === 'transfers'}
                  tabIndex={detailTab === 'transfers' ? 0 : -1}
                  className={detailTab === 'transfers' ? 'is-current' : ''}
                  onClick={() => setDetailTab('transfers')}
                  onKeyDown={(event) => {
                    if (event.key !== 'ArrowLeft') return
                    event.preventDefault()
                    setDetailTab('preview')
                    previewTabRef.current?.focus()
                  }}
                >
                  <CloudUpload />
                  <span>Transfers</span>
                  {uploadSummary.remainingCount > 0 ? <b>{uploadSummary.remainingCount}</b> : null}
                </button>
              ) : null}
            </div>
            {detailTab === 'preview' ? <div className="explorer-preview-actions">
              {previewOpenInWindow ? <span className="explorer-preview-window-badge"><ExternalLink />In window</span> : null}
              {!previewOpenInWindow && preview?.kind === 'browser' && previewMode === 'rendered' ? <span className="explorer-preview-safety">Scripts disabled</span> : null}
              {sourceEditorVisible && editablePreview ? <label className="explorer-vim-toggle"><input type="checkbox" checked={vimMode} onChange={toggleVimMode} />Vim controls</label> : null}
              {!previewOpenInWindow && preview?.content != null && ['markdown', 'browser'].includes(preview.kind) ? (
                <div className="explorer-preview-toggle">
                  <button type="button" className={previewMode === 'rendered' ? 'is-current' : ''} onClick={() => setPreviewMode('rendered')}>Rendered</button>
                  <button type="button" className={previewMode === 'source' ? 'is-current' : ''} onClick={() => setPreviewMode('source')}>Source</button>
                </div>
              ) : null}
              {!previewOpenInWindow && preview && preview.kind !== 'unsupported' ? (
                <button
                  type="button"
                  ref={previewPopoutButtonRef}
                  className="explorer-preview-popout"
                  aria-label={`Open ${preview.name} preview in new window`}
                  title={dirty ? 'Save this file before opening its preview in a new window' : 'Open preview in new window'}
                  disabled={dirty || previewWindowPending}
                  onClick={openPreviewWindow}
                ><ExternalLink /></button>
              ) : null}
              {!previewOpenInWindow && editablePreview ? <button type="button" className="explorer-save" aria-label="Save" title="Save (Command/Ctrl+S or :w in Vim mode)" disabled={!dirty || saving} onClick={() => saveCurrentFile()}>{saving ? <LoaderCircle className="spin" /> : <Save />}<span>{saving ? 'Saving' : 'Save'}</span></button> : null}
            </div> : null}
          </div>
          {detailTab === 'preview' ? (
            <div className="explorer-preview-panel" id="explorer-preview-panel" role="tabpanel" aria-labelledby="explorer-preview-tab">
              {previewLoading ? <div className="explorer-preview-empty"><LoaderCircle className="spin" /> Loading preview…</div> : null}
              {!previewLoading && !selectedEntry ? <div className="explorer-preview-empty"><File /><span>Select one file to preview it.</span></div> : null}
              {!previewLoading && selectedEntry?.kind === 'directory' ? <div className="explorer-preview-empty"><Folder /><strong>{selectedEntry.name}</strong><span>Double-click to open this folder.</span></div> : null}
              {!previewLoading && previewOpenInWindow ? (
                <div className="explorer-preview-detached">
                  <div className="explorer-preview-detached__status" role="status" aria-live="polite">
                    <ExternalLink aria-hidden="true" />
                    <strong>Preview open in window</strong>
                    <span>{preview.name} is showing in its own window. Close that window to bring the preview back here.</span>
                  </div>
                  <button
                    type="button"
                    ref={previewFocusButtonRef}
                    aria-label={`Focus preview window for ${preview.name}`}
                    title="Bring the preview window to the front"
                    disabled={previewWindowPending}
                    onClick={focusPreviewWindow}
                  >
                    {previewWindowPending ? <LoaderCircle className="spin" aria-hidden="true" /> : <ExternalLink aria-hidden="true" />}
                    {previewWindowPending ? 'Focusing…' : 'Focus preview window'}
                  </button>
                </div>
              ) : null}
              {!previewLoading && preview && !previewOpenInWindow ? (
                <PreviewContent
                  content={draftContent}
                  mode={previewMode}
                  preview={preview}
                  vimMode={vimMode}
                  onChange={(content) => { setDraftContent(content); setDirty(content !== preview.content) }}
                  onSave={saveCurrentFile}
                />
              ) : null}
            </div>
          ) : null}
          {detailTab === 'transfers' && uploadQueue ? (
            <div className="explorer-preview-panel" id="explorer-transfers-panel" role="tabpanel" aria-labelledby="explorer-transfers-tab">
              <UploadTransfers queue={uploadQueue} />
            </div>
          ) : null}
        </aside>
      </div>

      {phase === 'locked' ? (
        <div className="explorer-unlock-backdrop">
          <form className="explorer-unlock" onSubmit={(event) => { event.preventDefault(); connect(passphrase) }}>
            <div className="explorer-unlock__icon"><Server /></div>
            <span>Encrypted SSH key</span>
            <h1>Unlock {server?.name || 'server'}</h1>
            <p>Enter the passphrase for the saved private key. It stays in this explorer process only.</p>
            <label>Key passphrase<input autoFocus type="password" value={passphrase} onChange={(event) => setPassphrase(event.target.value)} /></label>
            <button type="submit" disabled={!passphrase}>Connect and explore</button>
          </form>
        </div>
      ) : null}

      {error && phase !== 'error' ? <div className="explorer-toast is-error" role="alert">{error}</div> : null}
      {notice ? <div className="explorer-toast" role="status">{notice}</div> : null}
      {contextMenu ? (
        <ExplorerContextMenu
          items={contextMenuItems()}
          position={contextMenu.position}
          onClose={closeContextMenu}
        />
      ) : null}
      <ConfirmDialog
        request={deleteRequest}
        pending={operation === 'delete'}
        onClose={() => {
          if (operation !== 'delete') setDeleteRequest(null)
        }}
        onConfirm={confirmDelete}
      />
    </div>
  )
}
