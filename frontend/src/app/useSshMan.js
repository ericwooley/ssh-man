import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import * as defaultApi from '../lib/api'
import {
  activeSessions,
  buildRuntimeSessions,
  findConfigurationRecord,
  isManagedSOCKSConfiguration,
  managedSOCKSConfigurationForServer,
  normalizeHistoryEntry,
  selectInitialServerId,
  userConfigurations,
} from '../model/appModel'

const defaultPreferences = {
  theme: 'dark',
  lastSelectedServerId: '',
  automaticUpdates: true,
  useExperimentalChannel: false,
  browserSwitcherShortcut: 'Alt+X',
  browserSwitcherBackwardShortcut: 'Alt+Z',
  browserAppearances: {},
  defaultBrowserId: '',
  proxyBrowserId: '',
  disabledBrowserIds: [],
  customBrowsers: [],
  urlRules: [],
  urlPortAssignments: [],
}

const unavailableDiagnostics = {
  appDataPath: '',
  databasePath: '',
  version: '',
  automaticUpdatesSupported: false,
}

const defaultUpdateStatus = {
  state: 'idle',
  channel: 'stable',
}

function normalizePreferences(preferences = {}) {
  return {
    ...defaultPreferences,
    ...preferences,
    browserAppearances: preferences.browserAppearances || {},
    disabledBrowserIds: preferences.disabledBrowserIds || [],
    customBrowsers: preferences.customBrowsers || [],
    urlRules: preferences.urlRules || [],
    urlPortAssignments: preferences.urlPortAssignments || [],
  }
}

function browserConfigurationSignature(preferences = {}) {
  return JSON.stringify({
    defaultBrowserId: preferences.defaultBrowserId || '',
    proxyBrowserId: preferences.proxyBrowserId || '',
    disabledBrowserIds: preferences.disabledBrowserIds || [],
    customBrowsers: preferences.customBrowsers || [],
    urlRules: preferences.urlRules || [],
    urlPortAssignments: preferences.urlPortAssignments || [],
  })
}

function preferenceRevision(value) {
  const match = String(value || '').match(
    /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?Z$/,
  )
  if (!match) return null
  return [match[1], (match[2] || '').padEnd(9, '0')]
}

function comparePreferenceRevisions(candidate, current) {
  if (candidate?.updatedAt === current?.updatedAt) return 0
  const candidateRevision = preferenceRevision(candidate?.updatedAt)
  const currentRevision = preferenceRevision(current?.updatedAt)
  if (!currentRevision) return candidateRevision ? 1 : 0
  if (!candidateRevision) return -1
  if (candidateRevision[0] !== currentRevision[0]) {
    return candidateRevision[0] > currentRevision[0] ? 1 : -1
  }
  if (candidateRevision[1] === currentRevision[1]) return 0
  return candidateRevision[1] > currentRevision[1] ? 1 : -1
}

function isPreferenceConflict(error) {
  return String(error?.message || error || '').includes('preference update conflict')
}

function preferenceSaveErrorDetail(error) {
  if (isPreferenceConflict(error)) {
    return 'Settings changed before this update was saved. Please try again.'
  }
  return error?.message || ''
}

async function writeClipboard(text) {
  if (!navigator.clipboard?.writeText) {
    throw new Error('Clipboard access is unavailable.')
  }
  await navigator.clipboard.writeText(text)
}

function firstConfigurationId(items, serverId) {
  const configurations = items.find((item) => item.server.id === serverId)?.configurations || []
  return userConfigurations(configurations)[0]?.id || ''
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
  const [diagnostics, setDiagnostics] = useState(unavailableDiagnostics)
  const [updateStatus, setUpdateStatus] = useState(defaultUpdateStatus)
  const [currentUsername, setCurrentUsername] = useState('')
  const [sshKeys, setSshKeys] = useState([])
  const [storageIssue, setStorageIssue] = useState('')
  const [runtimeFresh, setRuntimeFresh] = useState(true)
  const [notification, setNotification] = useState(null)
  const [pending, setPending] = useState({})
  const [historyByConfiguration, setHistoryByConfiguration] = useState({})
  const [historyLoadingId, setHistoryLoadingId] = useState('')
  const [unlockRequest, setUnlockRequest] = useState(null)
  const [browserAfterUnlock, setBrowserAfterUnlock] = useState(null)
  const [browserState, setBrowserState] = useState({
    configurationId: '',
    configurationRevision: 0,
    items: [],
    selectedId: '',
    preview: '',
    loading: false,
  })
  const [browserConfigurationRevision, setBrowserConfigurationRevision] = useState(0)
  const [urlRoutingState, setURLRoutingState] = useState({
    browsers: [],
    browserCatalog: [],
    defaultBrowser: { supported: false, isDefault: false },
    loading: false,
  })

  const notificationCounter = useRef(0)
  const preferenceWriteCounter = useRef(0)
  const preferenceWriteQueueRef = useRef(Promise.resolve())
  const persistedPreferencesRef = useRef(defaultPreferences)
  const pendingRef = useRef(new Set())
  const browserConfigurationSignatureRef = useRef(browserConfigurationSignature(defaultPreferences))
  const browserConfigurationRevisionRef = useRef(0)
  const browserDiscoveryRequestRef = useRef(0)

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
      const nextPreferences = normalizePreferences(state.preferences)

      if (comparePreferenceRevisions(nextPreferences, persistedPreferencesRef.current) >= 0) {
        browserConfigurationSignatureRef.current = browserConfigurationSignature(nextPreferences)
        persistedPreferencesRef.current = nextPreferences
        setPreferences(nextPreferences)
      }
      const nextServerId = selectInitialServerId(
        nextServers,
        persistedPreferencesRef.current.lastSelectedServerId,
      )
      setServers(nextServers)
      setSessions(buildRuntimeSessions(state.sessions || []))
      setDiagnostics(state.diagnostics || unavailableDiagnostics)
      setUpdateStatus(state.updateStatus || defaultUpdateStatus)
      setCurrentUsername(state.currentUsername || '')
      setSshKeys(state.sshKeys || [])
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
        const nextPreferences = normalizePreferences(state.preferences)

        if (comparePreferenceRevisions(nextPreferences, persistedPreferencesRef.current) >= 0) {
          browserConfigurationSignatureRef.current = browserConfigurationSignature(nextPreferences)
          persistedPreferencesRef.current = nextPreferences
          setPreferences(nextPreferences)
        }
        const nextServerId = selectInitialServerId(
          nextServers,
          persistedPreferencesRef.current.lastSelectedServerId,
        )
        setServers(nextServers)
        setSessions(buildRuntimeSessions(state.sessions || []))
        setDiagnostics(state.diagnostics || unavailableDiagnostics)
        setUpdateStatus(state.updateStatus || defaultUpdateStatus)
        setCurrentUsername(state.currentUsername || '')
        setSshKeys(state.sshKeys || [])
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

  useEffect(() => {
    if (!api.onPreferencesChanged) return undefined
    return api.onPreferencesChanged((nextPreferences) => {
      const normalized = normalizePreferences(nextPreferences)
      if (comparePreferenceRevisions(normalized, persistedPreferencesRef.current) < 0) return
      persistedPreferencesRef.current = normalized
      setPreferences(normalized)
      const signature = browserConfigurationSignature(normalized)
      if (signature !== browserConfigurationSignatureRef.current) {
        browserConfigurationSignatureRef.current = signature
        browserDiscoveryRequestRef.current += 1
        browserConfigurationRevisionRef.current += 1
        setBrowserConfigurationRevision(browserConfigurationRevisionRef.current)
        setBrowserState((current) => ({
          ...current,
          items: [],
          selectedId: '',
          preview: '',
          loading: Boolean(current.configurationId),
        }))
      }
      void hydrate({ quiet: true })
    })
  }, [api, hydrate])

  useEffect(() => {
    if (!api.onUpdateStatusChanged) return undefined
    return api.onUpdateStatusChanged((status) => {
      setUpdateStatus(status || defaultUpdateStatus)
    })
  }, [api])

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

  const savePreferencesQuietly = useCallback(async (update, persist = api.savePreferences) => {
    preferenceWriteCounter.current += 1
    const writeId = preferenceWriteCounter.current
    setPreferences((current) => update(current))

    const write = preferenceWriteQueueRef.current.then(async () => {
      try {
        const candidate = update(persistedPreferencesRef.current)
        const saved = await persist(candidate)
        const persisted = saved || candidate
        const current = persistedPreferencesRef.current
        const adopted = comparePreferenceRevisions(persisted, current) >= 0
        if (adopted) persistedPreferencesRef.current = persisted
        const result = adopted ? persisted : current
        if (writeId === preferenceWriteCounter.current) setPreferences(result)
        return result
      } catch (error) {
        if (isPreferenceConflict(error)) {
          await hydrate({ quiet: true })
        }
        throw error
      }
    })
    preferenceWriteQueueRef.current = write.catch(() => undefined)

    try {
      return await write
    } catch (error) {
      if (writeId === preferenceWriteCounter.current) {
        setPreferences(persistedPreferencesRef.current)
      }
      notify('warning', 'Your preference could not be saved.', preferenceSaveErrorDetail(error))
      return null
    }
  }, [api, hydrate, notify])

  const selectServer = useCallback((serverId) => {
    const nextConfigurationId = firstConfigurationId(servers, serverId)
    setSelectedServerId(serverId)
    setSelectedConfigurationId(nextConfigurationId)
    void savePreferencesQuietly((current) => ({ ...current, lastSelectedServerId: serverId }))
    return nextConfigurationId
  }, [savePreferencesQuietly, servers])

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
    setSelectedConfigurationId(isManagedSOCKSConfiguration(record.configuration)
      ? firstConfigurationId(servers, record.server.id)
      : configurationId)
    void savePreferencesQuietly((current) => ({ ...current, lastSelectedServerId: record.server.id }))
    if (!isManagedSOCKSConfiguration(record.configuration)) void refreshHistory(configurationId)
    return record
  }, [refreshHistory, savePreferencesQuietly, servers])

  const saveServer = useCallback((value) => runPending(`save-server:${value.id || 'new'}`, async () => {
    try {
      const saved = await api.saveServer({
        ...value,
        port: Number(value.port || 22),
        socksPort: Number(value.socksPort || 0),
      })
      setServers((current) => {
        const existing = current.find((item) => item.server.id === saved.id)
        const managedProxy = managedSOCKSConfigurationForServer(saved)
        return existing
          ? current.map((item) => {
              if (item.server.id !== saved.id) return item
              const hasManagedProxy = item.configurations.some(isManagedSOCKSConfiguration)
              const configurations = hasManagedProxy
                ? item.configurations.map((configuration) => isManagedSOCKSConfiguration(configuration)
                    ? { ...configuration, socksPort: saved.socksPort }
                    : configuration)
                : item.configurations.concat(managedProxy)
              return { ...item, server: { ...item.server, ...saved }, configurations }
            })
          : current.concat({ server: saved, configurations: [managedProxy] })
      })
      setSelectedServerId(saved.id)
      setSelectedConfigurationId((current) => value.id ? current : '')
      await savePreferencesQuietly((current) => ({ ...current, lastSelectedServerId: saved.id }))
      notify('success', `${saved.name} saved.`)
      return saved
    } catch (error) {
      notify('danger', 'The server could not be saved.', error.message || '')
      return null
    }
  }), [api, notify, runPending, savePreferencesQuietly])

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

  const launchServerBrowser = useCallback(async (serverName, configurationId, browserId = '') => {
    try {
      await api.launchBrowserThroughSocks(configurationId, browserId || preferences.proxyBrowserId || '')
      notify('success', `Browser is open through ${serverName}.`)
      return true
    } catch (error) {
      notify('danger', `The browser could not be opened through ${serverName}.`, error.message || '')
      return false
    }
  }, [api, notify, preferences.proxyBrowserId])

  const openServerBrowser = useCallback((serverId) => runPending(`browser:${serverId}`, async () => {
    const record = servers.find((item) => item.server.id === serverId)
    const managedProxy = record?.configurations.find(isManagedSOCKSConfiguration)
    if (!record || !managedProxy) {
      notify('danger', 'The browser could not be opened.', 'The automatic browser proxy is unavailable.')
      return false
    }

    try {
      let session = runtimeSessions.find((item) => item.configurationId === managedProxy.id) || null
      if (session?.status !== 'connected') {
        session = await api.startConfiguration(managedProxy.id)
        applySessions([session])
      }
      if (session?.status === 'needs_attention') {
        setBrowserAfterUnlock({
          serverName: record.server.name,
          configurationId: managedProxy.id,
          browserId: preferences.proxyBrowserId || '',
        })
        requestUnlock([session])
        return false
      }
      if (session?.status !== 'connected') {
        notify('danger', `The browser could not be opened through ${record.server.name}.`, session?.statusDetail || 'The browser proxy did not connect.')
        return false
      }

      setBrowserAfterUnlock(null)
      const opened = await launchServerBrowser(record.server.name, managedProxy.id)
      await refreshRuntimeSessions({ quiet: true })
      return opened
    } catch (error) {
      notify('danger', `The browser could not be opened through ${record.server.name}.`, error.message || '')
      return false
    }
  }), [api, applySessions, launchServerBrowser, notify, preferences.proxyBrowserId, refreshRuntimeSessions, requestUnlock, runPending, runtimeSessions, servers])

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
          if (
            browserAfterUnlock &&
            nextSessions.some((session) => session.configurationId === browserAfterUnlock.configurationId && session.status === 'connected')
          ) {
            const browser = browserAfterUnlock
            setBrowserAfterUnlock(null)
            await launchServerBrowser(browser.serverName, browser.configurationId, browser.browserId)
          }
        }
        await refreshRuntimeSessions({ quiet: true })
        return nextSessions
      } catch (error) {
        notify('danger', 'The SSH key could not be unlocked.', error.message || '')
        return null
      }
    })
  }, [api, applySessions, browserAfterUnlock, launchServerBrowser, notify, refreshRuntimeSessions, requestUnlock, runPending, unlockRequest])

  const closeUnlock = useCallback(() => {
    setUnlockRequest(null)
    setBrowserAfterUnlock(null)
  }, [])

  const openUnlock = useCallback((configurationId) => {
    const session = sessions.find((item) => item.configurationId === configurationId)
    if (session?.status !== 'needs_attention') return false
    requestUnlock([session])
    return true
  }, [requestUnlock, sessions])

  const toggleTheme = useCallback(async () => {
    const theme = preferences.theme === 'dark' ? 'light' : 'dark'
    await savePreferencesQuietly((current) => ({ ...current, theme }))
  }, [preferences, savePreferencesQuietly])

  const setAutomaticUpdates = useCallback(async (enabled) => {
    return savePreferencesQuietly((current) => ({ ...current, automaticUpdates: Boolean(enabled) }))
  }, [savePreferencesQuietly])

  const setExperimentalChannel = useCallback(async (enabled) => {
    return savePreferencesQuietly((current) => ({ ...current, useExperimentalChannel: Boolean(enabled) }))
  }, [savePreferencesQuietly])

  const installUpdate = useCallback(async () => runPending('install-update', async () => {
    try {
      await api.installApplicationUpdate()
      return true
    } catch (error) {
      notify('danger', 'SSH Man could not start the update.', error.message || '')
      return false
    }
  }), [api, notify, runPending])

  const setBrowserSwitcherShortcut = useCallback(async (shortcut) => {
    return savePreferencesQuietly((current) => ({ ...current, browserSwitcherShortcut: shortcut }))
  }, [savePreferencesQuietly])

  const setBrowserSwitcherBackwardShortcut = useCallback(async (shortcut) => {
    return savePreferencesQuietly((current) => ({ ...current, browserSwitcherBackwardShortcut: shortcut }))
  }, [savePreferencesQuietly])

  const setBrowserAppearance = useCallback(async (appearanceKey, appearance = {}) => {
    const key = String(appearanceKey || '').trim()
    if (!key) return null
    const icon = String(appearance.icon || '').trim()
    const primaryColor = String(appearance.primaryColor || '').trim().toUpperCase()
    const update = (current) => {
      const browserAppearances = { ...(current.browserAppearances || {}) }
      if (!icon && !primaryColor) {
        delete browserAppearances[key]
      } else {
        browserAppearances[key] = { icon, primaryColor }
      }
      return { ...current, browserAppearances }
    }
    const persist = api.saveBrowserAppearance
      ? () => api.saveBrowserAppearance(key, { icon, primaryColor })
      : (candidate) => api.savePreferences(candidate)
    return savePreferencesQuietly(update, persist)
  }, [api, savePreferencesQuietly])

  const loadURLRoutingSettings = useCallback(async () => {
    setURLRoutingState((current) => ({ ...current, loading: true }))
    try {
      const [browsers, browserCatalog, defaultBrowser] = await Promise.all([
        api.discoverBrowsers(),
        api.discoverBrowserCatalog?.() || api.discoverBrowsers(),
        api.defaultBrowserStatus?.() || Promise.resolve({ supported: false, isDefault: false }),
      ])
      const next = {
        browsers: browsers || [],
        browserCatalog: browserCatalog || [],
        defaultBrowser: defaultBrowser || { supported: false, isDefault: false },
        loading: false,
      }
      setURLRoutingState(next)
      return next
    } catch (error) {
      setURLRoutingState((current) => ({ ...current, loading: false }))
      notify('warning', 'URL routing settings could not be loaded.', error.message || '')
      return null
    }
  }, [api, notify])

  const saveURLRoutingSettings = useCallback(async (input) => {
    const update = (current) => ({
      ...current,
      defaultBrowserId: String(input.defaultBrowserId || '').trim(),
      proxyBrowserId: String(input.proxyBrowserId || '').trim(),
      disabledBrowserIds: input.disabledBrowserIds || [],
      customBrowsers: input.customBrowsers || [],
      urlRules: input.urlRules || [],
      urlPortAssignments: input.urlPortAssignments || [],
    })
    const saved = await savePreferencesQuietly(update)
    if (saved) {
      notify('success', 'Browser and routing settings saved.')
      await loadURLRoutingSettings()
    }
    return saved
  }, [loadURLRoutingSettings, notify, savePreferencesQuietly])

  const validateURLRulePattern = useCallback(
    (matchMode, pattern) => api.validateURLRulePattern(matchMode, pattern),
    [api],
  )

  const chooseBrowserApplication = useCallback(async () => {
    try {
      return await api.chooseBrowserApplication()
    } catch (error) {
      notify('warning', 'The browser application could not be selected.', error.message || '')
      return ''
    }
  }, [api, notify])

  const setAsDefaultBrowser = useCallback(async () => {
    try {
      const status = await api.setAsDefaultBrowser()
      setURLRoutingState((current) => ({ ...current, defaultBrowser: status }))
      notify('success', 'SSH Man is now your default browser.')
      return status
    } catch (error) {
      notify('danger', 'SSH Man could not become the default browser.', error.message || '')
      return null
    }
  }, [api, notify])

  const refreshBrowsers = useCallback(async (configurationId = selectedConfigurationId) => {
    if (!configurationId) return []
    const requestId = ++browserDiscoveryRequestRef.current
    const configurationRevision = browserConfigurationRevisionRef.current
    setBrowserState((current) => ({
      ...current,
      configurationId,
      configurationRevision,
      loading: true,
      preview: '',
    }))
    try {
      const items = await api.discoverBrowsers() || []
      if (requestId !== browserDiscoveryRequestRef.current) return []
      setBrowserState((current) => {
        const existing = items.some((browser) => browser.id === current.selectedId) ? current.selectedId : ''
        const selectedId = existing || items.find((browser) => browser.supportsProxyLaunch)?.id || items[0]?.id || ''
        return { configurationId, configurationRevision, items, selectedId, preview: '', loading: false }
      })
      return items
    } catch (error) {
      if (requestId !== browserDiscoveryRequestRef.current) return []
      setBrowserState({ configurationId, configurationRevision, items: [], selectedId: '', preview: '', loading: false })
      notify('warning', 'Installed browsers could not be discovered.', error.message || '')
      return []
    }
  }, [api, notify, selectedConfigurationId])

  useEffect(() => {
    if (selectedConfiguration?.connectionType !== 'socks_proxy') {
      browserDiscoveryRequestRef.current += 1
      setBrowserState({
        configurationId: '',
        configurationRevision: browserConfigurationRevision,
        items: [],
        selectedId: '',
        preview: '',
        loading: false,
      })
      return
    }
    if (
      browserState.configurationId !== selectedConfiguration.id ||
      browserState.configurationRevision !== browserConfigurationRevision
    ) {
      refreshBrowsers(selectedConfiguration.id)
    }
  }, [
    browserConfigurationRevision,
    browserState.configurationId,
    browserState.configurationRevision,
    refreshBrowsers,
    selectedConfiguration,
  ])

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
      let session = selectedSession
      if (session?.status !== 'connected') {
        session = await api.startConfiguration(selectedConfiguration.id)
        applySessions([session])
      }
      if (session?.status === 'needs_attention') {
        setBrowserAfterUnlock({
          serverName: selectedServer?.name || 'the selected server',
          configurationId: selectedConfiguration.id,
          browserId: browserState.selectedId,
        })
        requestUnlock([session])
        return false
      }
      if (session?.status !== 'connected') {
        notify('danger', 'The browser could not be launched.', session?.statusDetail || 'The SOCKS proxy did not connect.')
        return false
      }

      setBrowserAfterUnlock(null)
      const opened = await launchServerBrowser(
        selectedServer?.name || 'the selected server',
        selectedConfiguration.id,
        browserState.selectedId,
      )
      await refreshRuntimeSessions({ quiet: true })
      return opened
    } catch (error) {
      notify('danger', 'The browser could not be launched.', error.message || '')
      return false
    }
  }), [
    api,
    applySessions,
    browserState.selectedId,
    launchServerBrowser,
    notify,
    refreshRuntimeSessions,
    requestUnlock,
    runPending,
    selectedConfiguration,
    selectedServer?.name,
    selectedSession,
  ])

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

  const openServerExplorer = useCallback((serverId) => runPending(`explore-server:${serverId}`, async () => {
    const serverName = servers.find((item) => item.server.id === serverId)?.server.name || 'Server'
    try {
      await api.openServerExplorer(serverId)
      notify('success', `${serverName} explorer opened in its own window.`)
      return true
    } catch (error) {
      notify('danger', 'The server explorer could not be opened.', error.message || '')
      return false
    }
  }), [api, notify, runPending, servers])

  const openSettingsWindow = useCallback(() => runPending('open-settings', async () => {
    try {
      await api.openSettingsWindow()
      return true
    } catch (error) {
      notify('danger', 'Settings could not be opened.', error.message || '')
      return false
    }
  }), [api, notify, runPending])

  const openServerCommand = useCallback((serverId) => runPending(`command-server:${serverId}`, async () => {
    const serverName = servers.find((item) => item.server.id === serverId)?.server.name || 'Server'
    try {
      await api.openServerCommand(serverId)
      notify('success', `${serverName} command window opened.`)
      return true
    } catch (error) {
      notify('danger', 'The command window could not be opened.', error.message || '')
      return false
    }
  }), [api, notify, runPending, servers])

  const openHostWindow = useCallback((serverId) => runPending(`open-host:${serverId}`, async () => {
    try {
      await api.openHostWindow(serverId)
      notify('success', 'Host details opened in a new window.')
      return true
    } catch (error) {
      notify('danger', 'Host details could not be opened.', error.message || '')
      return false
    }
  }), [api, notify, runPending])

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
    updateStatus,
    currentUsername,
    sshKeys,
    storageIssue,
    runtimeFresh,
    notification,
    pending,
    historyLoading: historyLoadingId === selectedConfigurationId,
    unlockRequest,
    browserState,
    browserConfigurationRevision,
    urlRoutingState,
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
    setAutomaticUpdates,
    setExperimentalChannel,
    installUpdate,
    setBrowserSwitcherShortcut,
    setBrowserSwitcherBackwardShortcut,
    setBrowserAppearance,
    loadURLRoutingSettings,
    saveURLRoutingSettings,
    validateURLRulePattern,
    chooseBrowserApplication,
    setAsDefaultBrowser,
    copyHistory,
    copyPath,
    openDevTools,
    openHostWindow,
    openServerExplorer,
    openSettingsWindow,
    openServerCommand,
    openServerBrowser,
    hideWindow: api.hideApplicationWindow,
    quitApplication: api.quitApplication,
    openExternalURL: api.openExternalURL,
    dismissNotification,
  }
}
