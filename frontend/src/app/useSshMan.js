import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import * as defaultApi from '../lib/api'
import {
  activeSessions,
  buildRuntimeSessions,
  findConfigurationRecord,
  normalizeHistoryEntry,
  selectInitialServerId,
} from '../model/appModel'

const defaultPreferences = { theme: 'dark', lastSelectedServerId: '' }

async function writeClipboard(text) {
  if (!navigator.clipboard?.writeText) {
    throw new Error('Clipboard access is unavailable.')
  }
  await navigator.clipboard.writeText(text)
}

function firstConfigurationId(items, serverId) {
  return items.find((item) => item.server.id === serverId)?.configurations?.[0]?.id || ''
}

function mergeSession(source, session) {
  if (!session?.configurationId) return source
  return buildRuntimeSessions(source.filter((item) => item.configurationId !== session.configurationId).concat(session))
}

export function useSshMan(api = defaultApi, options = {}) {
  const pollMs = options.pollMs ?? 1500
  const copyText = options.copyText ?? writeClipboard
  const [phase, setPhase] = useState('loading')
  const [servers, setServers] = useState([])
  const [preferences, setPreferences] = useState(defaultPreferences)
  const [sessions, setSessions] = useState([])
  const [selectedServerId, setSelectedServerId] = useState('')
  const [selectedConfigurationId, setSelectedConfigurationId] = useState('')
  const [diagnostics, setDiagnostics] = useState({ appDataPath: '', databasePath: '' })
  const [currentUsername, setCurrentUsername] = useState('')
  const [storageIssue, setStorageIssue] = useState('')
  const [runtimeFresh, setRuntimeFresh] = useState(true)
  const [notification, setNotification] = useState(null)
  const [pending, setPending] = useState({})
  const [historyByConfiguration, setHistoryByConfiguration] = useState({})
  const [historyLoadingId, setHistoryLoadingId] = useState('')
  const [unlockRequest, setUnlockRequest] = useState(null)
  const [browserState, setBrowserState] = useState({
    configurationId: '',
    items: [],
    selectedId: '',
    preview: '',
    loading: false,
  })

  const notificationCounter = useRef(0)
  const preferenceWriteCounter = useRef(0)
  const pendingRef = useRef(new Set())

  const notify = useCallback((kind, message, detail = '') => {
    notificationCounter.current += 1
    setNotification({ id: notificationCounter.current, kind, message, detail })
  }, [])

  const dismissNotification = useCallback(() => setNotification(null), [])

  const runPending = useCallback(async (key, action) => {
    if (pendingRef.current.has(key)) return undefined
    pendingRef.current.add(key)
    setPending((current) => ({ ...current, [key]: true }))
    try {
      return await action()
    } finally {
      pendingRef.current.delete(key)
      setPending((current) => {
        const next = { ...current }
        delete next[key]
        return next
      })
    }
  }, [])

  const refreshRuntimeSessions = useCallback(async ({ quiet = true } = {}) => {
    try {
      const next = await api.listRuntimeSessions()
      setSessions(buildRuntimeSessions(next || []))
      setRuntimeFresh(true)
      return next
    } catch (error) {
      setRuntimeFresh(false)
      if (!quiet) notify('warning', 'Live connection status could not be refreshed.', error.message || '')
      return null
    }
  }, [api, notify])

  const hydrate = useCallback(async ({ quiet = false } = {}) => {
    if (!quiet) setPhase('loading')
    try {
      const state = await api.loadInitialState()
      const nextServers = state.servers || []
      const nextPreferences = { ...defaultPreferences, ...(state.preferences || {}) }
      const nextServerId = selectInitialServerId(nextServers, nextPreferences.lastSelectedServerId)

      setServers(nextServers)
      setPreferences(nextPreferences)
      setSessions(buildRuntimeSessions(state.sessions || []))
      setDiagnostics(state.diagnostics || { appDataPath: '', databasePath: '' })
      setCurrentUsername(state.currentUsername || '')
      setStorageIssue(state.recoverable ? state.message || 'Saved data needs attention.' : '')
      setSelectedServerId(nextServerId)
      setSelectedConfigurationId(firstConfigurationId(nextServers, nextServerId))
      setRuntimeFresh(true)
      setPhase('ready')

      if (state.message) {
        notify(state.recoverable ? 'warning' : 'info', state.message)
      }
      return state
    } catch (error) {
      const message = error.message || 'Saved data could not be loaded.'
      setStorageIssue(message)
      setPhase('error')
      notify('danger', 'SSH Man could not load your saved data.', message)
      return null
    }
  }, [api, notify])

  useEffect(() => {
    let active = true
    let intervalId

    api.loadInitialState()
      .then((state) => {
        if (!active) return
        const nextServers = state.servers || []
        const nextPreferences = { ...defaultPreferences, ...(state.preferences || {}) }
        const nextServerId = selectInitialServerId(nextServers, nextPreferences.lastSelectedServerId)

        setServers(nextServers)
        setPreferences(nextPreferences)
        setSessions(buildRuntimeSessions(state.sessions || []))
        setDiagnostics(state.diagnostics || { appDataPath: '', databasePath: '' })
        setCurrentUsername(state.currentUsername || '')
        setStorageIssue(state.recoverable ? state.message || 'Saved data needs attention.' : '')
        setSelectedServerId(nextServerId)
        setSelectedConfigurationId(firstConfigurationId(nextServers, nextServerId))
        setPhase('ready')
        if (state.message) notify(state.recoverable ? 'warning' : 'info', state.message)
      })
      .catch((error) => {
        if (!active) return
        const message = error.message || 'Saved data could not be loaded.'
        setStorageIssue(message)
        setPhase('error')
        notify('danger', 'SSH Man could not load your saved data.', message)
      })

    intervalId = window.setInterval(() => {
      refreshRuntimeSessions({ quiet: true })
    }, pollMs)

    return () => {
      active = false
      window.clearInterval(intervalId)
    }
  }, [api, notify, pollMs, refreshRuntimeSessions])

  const runtimeSessions = useMemo(() => buildRuntimeSessions(sessions), [sessions])
  const selectedServerRecord = useMemo(
    () => servers.find((item) => item.server.id === selectedServerId) || null,
    [servers, selectedServerId],
  )
  const selectedServer = selectedServerRecord?.server || null
  const selectedConfigurations = selectedServerRecord?.configurations || []
  const selectedConfiguration = selectedConfigurations.find((item) => item.id === selectedConfigurationId) || null
  const selectedSession = runtimeSessions.find((item) => item.configurationId === selectedConfigurationId) || null
  const selectedHistory = historyByConfiguration[selectedConfigurationId] || []
  const liveSessions = useMemo(() => activeSessions(runtimeSessions), [runtimeSessions])

  const savePreferencesQuietly = useCallback(async (next) => {
    preferenceWriteCounter.current += 1
    const writeId = preferenceWriteCounter.current
    setPreferences(next)
    try {
      const saved = await api.savePreferences(next)
      if (writeId === preferenceWriteCounter.current) setPreferences(saved || next)
    } catch (error) {
      notify('warning', 'Your preference could not be saved.', error.message || '')
    }
  }, [api, notify])

  const selectServer = useCallback((serverId) => {
    const nextConfigurationId = firstConfigurationId(servers, serverId)
    setSelectedServerId(serverId)
    setSelectedConfigurationId(nextConfigurationId)
    void savePreferencesQuietly({ ...preferences, lastSelectedServerId: serverId })
    return nextConfigurationId
  }, [preferences, savePreferencesQuietly, servers])

  const refreshHistory = useCallback(async (configurationId) => {
    if (!configurationId) return []
    setHistoryLoadingId(configurationId)
    try {
      const entries = await api.listSessionHistory(configurationId)
      const normalized = (entries || []).map(normalizeHistoryEntry).filter(Boolean)
      setHistoryByConfiguration((current) => ({ ...current, [configurationId]: normalized }))
      return normalized
    } catch (error) {
      notify('warning', 'Connection history could not be loaded.', error.message || '')
      return []
    } finally {
      setHistoryLoadingId((current) => current === configurationId ? '' : current)
    }
  }, [api, notify])

  const selectConfiguration = useCallback((configurationId) => {
    const record = findConfigurationRecord(servers, configurationId)
    if (!record) return null
    setSelectedServerId(record.server.id)
    setSelectedConfigurationId(configurationId)
    void savePreferencesQuietly({ ...preferences, lastSelectedServerId: record.server.id })
    void refreshHistory(configurationId)
    return record
  }, [preferences, refreshHistory, savePreferencesQuietly, servers])

  const saveServer = useCallback((value) => runPending(`save-server:${value.id || 'new'}`, async () => {
    try {
      const saved = await api.saveServer({ ...value, port: Number(value.port || 22) })
      setServers((current) => {
        const existing = current.find((item) => item.server.id === saved.id)
        return existing
          ? current.map((item) => item.server.id === saved.id ? { ...item, server: { ...item.server, ...saved } } : item)
          : current.concat({ server: saved, configurations: [] })
      })
      setSelectedServerId(saved.id)
      setSelectedConfigurationId((current) => value.id ? current : '')
      await savePreferencesQuietly({ ...preferences, lastSelectedServerId: saved.id })
      notify('success', `${saved.name} saved.`)
      return saved
    } catch (error) {
      notify('danger', 'The server could not be saved.', error.message || '')
      return null
    }
  }), [api, notify, preferences, runPending, savePreferencesQuietly])

  const deleteServer = useCallback((serverId) => runPending(`delete-server:${serverId}`, async () => {
    try {
      await api.deleteServer(serverId)
      setServers((current) => {
        const next = current.filter((item) => item.server.id !== serverId)
        const nextServerId = selectInitialServerId(next, '')
        setSelectedServerId(nextServerId)
        setSelectedConfigurationId(firstConfigurationId(next, nextServerId))
        return next
      })
      notify('success', 'Server deleted.')
      return true
    } catch (error) {
      notify('danger', 'The server could not be deleted.', error.message || '')
      return false
    }
  }), [api, notify, runPending])

  const saveTunnel = useCallback((value) => runPending(`save-tunnel:${value.id || 'new'}`, async () => {
    try {
      const saved = await api.saveConnectionConfiguration({
        ...value,
        serverId: value.serverId || selectedServerId,
        localPort: Number(value.localPort || 0),
        remotePort: Number(value.remotePort || 0),
        socksPort: value.connectionType === 'socks_proxy'
          ? (value.socksPort === '' || value.socksPort === 'auto' ? 0 : Number(value.socksPort || 0))
          : 0,
      })
      setServers((current) => current.map((item) => {
        if (item.server.id !== saved.serverId) return item
        const configurations = item.configurations.filter((entry) => entry.id !== saved.id).concat(saved)
        return { ...item, configurations }
      }))
      setSelectedServerId(saved.serverId)
      setSelectedConfigurationId(saved.id)
      notify('success', `${saved.label} saved.`)
      return saved
    } catch (error) {
      notify('danger', 'The tunnel could not be saved.', error.message || '')
      return null
    }
  }), [api, notify, runPending, selectedServerId])

  const deleteTunnel = useCallback((configurationId) => runPending(`delete-tunnel:${configurationId}`, async () => {
    try {
      await api.deleteConnectionConfiguration(configurationId)
      setServers((current) => current.map((item) => {
        if (!item.configurations.some((configuration) => configuration.id === configurationId)) return item
        const configurations = item.configurations.filter((configuration) => configuration.id !== configurationId)
        if (selectedConfigurationId === configurationId) {
          setSelectedConfigurationId(configurations[0]?.id || '')
        }
        return { ...item, configurations }
      }))
      setHistoryByConfiguration((current) => {
        const next = { ...current }
        delete next[configurationId]
        return next
      })
      notify('success', 'Tunnel deleted.')
      return true
    } catch (error) {
      notify('danger', 'The tunnel could not be deleted.', error.message || '')
      return false
    }
  }), [api, notify, runPending, selectedConfigurationId])

  const applySessions = useCallback((nextSessions) => {
    setSessions((current) => (nextSessions || []).reduce((next, session) => mergeSession(next, session), current))
  }, [])

  const requestUnlock = useCallback((attentionSessions) => {
    if (!attentionSessions.length) return
    const first = attentionSessions[0]
    const record = findConfigurationRecord(servers, first.configurationId)
    setUnlockRequest({
      configurationIds: attentionSessions.map((session) => session.configurationId),
      configurationLabel: record?.configuration?.label || 'SSH tunnel',
      detail: first.statusDetail || 'Unlock the SSH key to continue.',
    })
  }, [servers])

  const startTunnel = useCallback((configurationId) => runPending(`session:${configurationId}`, async () => {
    try {
      const session = await api.startConfiguration(configurationId)
      applySessions([session])
      if (session?.status === 'needs_attention') requestUnlock([session])
      notify(session?.status === 'failed' ? 'danger' : session?.status === 'needs_attention' ? 'warning' : 'success', session?.statusDetail || 'Tunnel started.')
      await Promise.all([refreshRuntimeSessions({ quiet: true }), refreshHistory(configurationId)])
      return session
    } catch (error) {
      notify('danger', 'The tunnel could not be started.', error.message || '')
      return null
    }
  }), [api, applySessions, notify, refreshHistory, refreshRuntimeSessions, requestUnlock, runPending])

  const stopTunnel = useCallback((configurationId) => runPending(`session:${configurationId}`, async () => {
    try {
      const session = await api.stopConfiguration(configurationId)
      applySessions([session])
      notify('success', session?.statusDetail || 'Tunnel stopped.')
      await Promise.all([refreshRuntimeSessions({ quiet: true }), refreshHistory(configurationId)])
      return session
    } catch (error) {
      notify('danger', 'The tunnel could not be stopped.', error.message || '')
      return null
    }
  }), [api, applySessions, notify, refreshHistory, refreshRuntimeSessions, runPending])

  const retryTunnel = useCallback((configurationId) => runPending(`session:${configurationId}`, async () => {
    try {
      const session = await api.retryConfiguration(configurationId)
      applySessions([session])
      if (session?.status === 'needs_attention') requestUnlock([session])
      notify(session?.status === 'failed' ? 'danger' : 'success', session?.statusDetail || 'Tunnel retry requested.')
      await Promise.all([refreshRuntimeSessions({ quiet: true }), refreshHistory(configurationId)])
      return session
    } catch (error) {
      notify('danger', 'The tunnel could not be retried.', error.message || '')
      return null
    }
  }), [api, applySessions, notify, refreshHistory, refreshRuntimeSessions, requestUnlock, runPending])

  const startAll = useCallback((serverId) => runPending(`start-all:${serverId}`, async () => {
    try {
      const nextSessions = await api.startServerConfigurations(serverId)
      applySessions(nextSessions)
      const attention = (nextSessions || []).filter((session) => session.status === 'needs_attention')
      const failed = (nextSessions || []).filter((session) => session.status === 'failed')
      const connected = (nextSessions || []).filter((session) => session.status === 'connected')
      requestUnlock(attention)
      const summary = [
        connected.length ? `${connected.length} connected` : '',
        attention.length ? `${attention.length} need attention` : '',
        failed.length ? `${failed.length} failed` : '',
      ].filter(Boolean).join(' · ') || 'No tunnels were started.'
      notify(failed.length || attention.length ? 'warning' : 'success', summary)
      await refreshRuntimeSessions({ quiet: true })
      return nextSessions
    } catch (error) {
      const refreshed = await refreshRuntimeSessions({ quiet: true })
      const refreshDetail = refreshed
        ? 'Live status was refreshed so any tunnels that did start are shown.'
        : 'Live status could not be refreshed.'
      notify('warning', 'Starting inactive tunnels did not complete.', [error.message, refreshDetail].filter(Boolean).join(' '))
      return refreshed
    }
  }), [api, applySessions, notify, refreshRuntimeSessions, requestUnlock, runPending])

  const submitUnlock = useCallback((secret) => {
    if (!unlockRequest) return Promise.resolve(null)
    return runPending('unlock', async () => {
      try {
        const nextSessions = []
        for (const configurationId of unlockRequest.configurationIds) {
          nextSessions.push(await api.submitKeyUnlock(configurationId, secret))
        }
        applySessions(nextSessions)
        const remaining = nextSessions.filter((session) => session.status === 'needs_attention')
        if (remaining.length) {
          requestUnlock(remaining)
          notify('warning', `${remaining.length} tunnel${remaining.length === 1 ? '' : 's'} still need attention.`)
        } else {
          setUnlockRequest(null)
          notify('success', `${nextSessions.filter((session) => session.status === 'connected').length} tunnel${nextSessions.length === 1 ? '' : 's'} unlocked.`)
        }
        await refreshRuntimeSessions({ quiet: true })
        return nextSessions
      } catch (error) {
        notify('danger', 'The SSH key could not be unlocked.', error.message || '')
        return null
      }
    })
  }, [api, applySessions, notify, refreshRuntimeSessions, requestUnlock, runPending, unlockRequest])

  const closeUnlock = useCallback(() => setUnlockRequest(null), [])

  const openUnlock = useCallback((configurationId) => {
    const session = sessions.find((item) => item.configurationId === configurationId)
    if (session?.status !== 'needs_attention') return false
    requestUnlock([session])
    return true
  }, [requestUnlock, sessions])

  const toggleTheme = useCallback(async () => {
    const next = { ...preferences, theme: preferences.theme === 'dark' ? 'light' : 'dark' }
    await savePreferencesQuietly(next)
  }, [preferences, savePreferencesQuietly])

  const refreshBrowsers = useCallback(async (configurationId = selectedConfigurationId) => {
    if (!configurationId) return []
    setBrowserState((current) => ({ ...current, configurationId, loading: true, preview: '' }))
    try {
      const items = await api.discoverBrowsers()
      setBrowserState((current) => {
        const existing = items.some((browser) => browser.id === current.selectedId) ? current.selectedId : ''
        const selectedId = existing || items.find((browser) => browser.supportsProxyLaunch)?.id || items[0]?.id || ''
        return { configurationId, items, selectedId, preview: '', loading: false }
      })
      return items
    } catch (error) {
      setBrowserState({ configurationId, items: [], selectedId: '', preview: '', loading: false })
      notify('warning', 'Installed browsers could not be discovered.', error.message || '')
      return []
    }
  }, [api, notify, selectedConfigurationId])

  useEffect(() => {
    if (selectedConfiguration?.connectionType !== 'socks_proxy') {
      setBrowserState({ configurationId: '', items: [], selectedId: '', preview: '', loading: false })
      return
    }
    if (browserState.configurationId !== selectedConfiguration.id) {
      refreshBrowsers(selectedConfiguration.id)
    }
  }, [browserState.configurationId, refreshBrowsers, selectedConfiguration])

  useEffect(() => {
    let active = true
    const selectedBrowser = browserState.items.find((browser) => browser.id === browserState.selectedId)
    if (
      !selectedConfiguration ||
      selectedConfiguration.connectionType !== 'socks_proxy' ||
      selectedSession?.status !== 'connected' ||
      !selectedBrowser?.supportsProxyLaunch
    ) {
      setBrowserState((current) => current.preview ? { ...current, preview: '' } : current)
      return () => { active = false }
    }

    api.previewBrowserLaunchThroughSocks(selectedConfiguration.id, selectedBrowser.id)
      .then((preview) => {
        if (active) setBrowserState((current) => ({ ...current, preview: preview.command || '' }))
      })
      .catch(() => {
        if (active) setBrowserState((current) => ({ ...current, preview: '' }))
      })

    return () => { active = false }
  }, [api, browserState.items, browserState.selectedId, selectedConfiguration, selectedSession?.status])

  const selectBrowser = useCallback((selectedId) => {
    setBrowserState((current) => ({ ...current, selectedId, preview: '' }))
  }, [])

  const launchBrowser = useCallback(() => runPending('launch-browser', async () => {
    if (!selectedConfiguration || !browserState.selectedId) return false
    try {
      await api.launchBrowserThroughSocks(selectedConfiguration.id, browserState.selectedId)
      notify('success', 'Browser launch request sent.')
      return true
    } catch (error) {
      notify('danger', 'The browser could not be launched.', error.message || '')
      return false
    }
  }), [api, browserState.selectedId, notify, runPending, selectedConfiguration])

  const copyHistory = useCallback(async (configurationId) => {
    const history = historyByConfiguration[configurationId] || []
    if (!history.length) return
    const lines = history.map((entry) => `[${entry.endedAt || entry.startedAt || ''}] ${entry.outcome}: ${entry.message}`)
    try {
      await copyText(lines.join('\n'))
      notify('success', 'Connection history copied.')
    } catch (error) {
      notify('warning', 'Connection history could not be copied.', error.message || '')
    }
  }, [copyText, historyByConfiguration, notify])

  const copyPath = useCallback(async (label, value) => {
    if (!value) return
    try {
      await copyText(value)
      notify('success', `${label} copied.`)
    } catch (error) {
      notify('warning', `${label} could not be copied.`, error.message || '')
    }
  }, [copyText, notify])

  const openDevTools = useCallback(async () => {
    try {
      await api.openDevTools()
    } catch (error) {
      notify('warning', 'Frontend devtools could not be opened.', error.message || '')
    }
  }, [api, notify])

  return {
    phase,
    servers,
    preferences,
    runtimeSessions,
    liveSessions,
    selectedServerId,
    selectedConfigurationId,
    selectedServerRecord,
    selectedServer,
    selectedConfigurations,
    selectedConfiguration,
    selectedSession,
    selectedHistory,
    diagnostics,
    currentUsername,
    storageIssue,
    runtimeFresh,
    notification,
    pending,
    historyLoading: historyLoadingId === selectedConfigurationId,
    unlockRequest,
    browserState,
    hydrate,
    refreshRuntimeSessions,
    selectServer,
    selectConfiguration,
    saveServer,
    deleteServer,
    saveTunnel,
    deleteTunnel,
    startTunnel,
    stopTunnel,
    retryTunnel,
    startAll,
    submitUnlock,
    closeUnlock,
    openUnlock,
    refreshHistory,
    refreshBrowsers,
    selectBrowser,
    launchBrowser,
    toggleTheme,
    copyHistory,
    copyPath,
    openDevTools,
    hideWindow: api.hideApplicationWindow,
    quitApplication: api.quitApplication,
    openExternalURL: api.openExternalURL,
    dismissNotification,
  }
}
