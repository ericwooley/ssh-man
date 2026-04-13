<script>
  import { onDestroy, onMount } from 'svelte'
  import {
    deleteConnectionConfiguration,
    deleteServer,
    discoverBrowsers,
    launchBrowserThroughSocks,
    listSessionHistory,
    listRuntimeSessions,
    loadInitialState,
    openDevTools,
    previewBrowserLaunchThroughSocks,
    retryConfiguration,
    saveConnectionConfiguration,
    savePreferences,
    saveServer,
    startConfiguration,
    startServerConfigurations,
    stopConfiguration,
    submitKeyUnlock,
  } from '../lib/api'
  import ServerList from '../components/ServerList.svelte'
  import ConfigList from '../components/ConfigList.svelte'
  import ActiveConnections from '../components/ActiveConnections.svelte'
  import ConfigEditor from '../components/ConfigEditor.svelte'
  import DiagnosticsPanel from '../components/DiagnosticsPanel.svelte'
  import ServerEditorDialog from '../components/ServerEditorDialog.svelte'
  import SessionStatus from '../components/SessionStatus.svelte'
  import UnlockKeyDialog from '../components/UnlockKeyDialog.svelte'
  import BrowserLauncher from '../components/BrowserLauncher.svelte'
  import ThemeToggle from '../components/ThemeToggle.svelte'

  const emptyConfig = () => ({
    serverId: '',
    label: '',
    connectionType: 'local_forward',
    localPort: '',
    remoteHost: '',
    remotePort: '',
    socksPort: '',
    autoReconnectEnabled: true,
    notes: '',
  })

  const emptyServer = () => ({
    id: '',
    name: '',
    host: '',
    port: '22',
    username: '',
    authMode: 'agent',
    keyReference: '',
  })

  let servers = []
  let preferences = { theme: 'dark', lastSelectedServerId: '' }
  let sessions = []
  let selectedServerId = ''
  let selectedConfigurationId = ''
  let editorValue = emptyConfig()
  let editorErrors = {}
  let serverDialogOpen = false
  let serverValue = emptyServer()
  let serverErrors = {}
  let tunnelDialogOpen = false
  let browsers = []
  let selectedBrowserId = ''
  let browserLaunchPreview = ''
  let banner = ''
  let bannerKind = 'info'
  let loadRecoverable = false
  let isHydrating = false
  let unlockDialogOpen = false
  let unlockConfigurationId = ''
  let unlockConfigurationIds = []
  let diagnosticDetails = ''
  let diagnostics = { appDataPath: '', databasePath: '' }
  let runtimeRefreshHandle = null
  let selectedServerRecord = null
  let selectedServer = null
  let selectedConfigurations = []
  let selectedConfiguration = null
  let selectedSession = null
  let unlockConfiguration = null
  let unlockSession = null
  let runtimeSessionSnapshot = []
  let sessionHistory = []
  let activeRuntimeConnections = []
  let totalTunnelCount = 0
  let connectedSessionCount = 0
  let attentionSessionCount = 0
  let selectedServerActiveCount = 0
  let canCreateTunnel = false
  let canStartSelectedServer = false
  let currentIssue = ''
  let browserDiscoveryKey = ''
  let browserPreviewKey = ''
  let historyConfigurationKey = ''

  function normalizeHistoryEntry(entry) {
    if (!entry) return null

    const id = entry.id || entry.ID || ''
    const configurationId = entry.configurationId || entry.ConfigurationID || ''
    if (!id || !configurationId) return null

    return {
      id,
      configurationId,
      startedAt: entry.startedAt || entry.StartedAt || '',
      endedAt: entry.endedAt || entry.EndedAt || '',
      outcome: entry.outcome || entry.Outcome || '',
      message: entry.message || entry.Message || '',
    }
  }

  function replaceSessionHistory(nextEntries) {
    sessionHistory = (nextEntries || []).map((entry) => normalizeHistoryEntry(entry)).filter(Boolean)
  }

  async function refreshSessionHistory(configurationId) {
    if (!configurationId) {
      sessionHistory = []
      return
    }

    try {
      replaceSessionHistory(await listSessionHistory(configurationId))
    } catch (error) {
      banner = error.message || 'Connection history could not be loaded.'
      diagnosticDetails = error.message || ''
      bannerKind = 'warning'
    }
  }

  function normalizeSession(session) {
    if (!session) return null

    const configurationId = session.configurationId || session.ConfigurationID || ''
    if (!configurationId) return null

    return {
      configurationId,
      status: session.status || session.Status || 'stopped',
      boundPort: session.boundPort || session.BoundPort || 0,
      statusDetail: session.statusDetail || session.StatusDetail || '',
      startedAt: session.startedAt || session.StartedAt || '',
      lastStateChangeAt: session.lastStateChangeAt || session.LastStateChangeAt || '',
      reconnectAttemptCount: session.reconnectAttemptCount || session.ReconnectAttemptCount || 0,
      lastError: session.lastError || session.LastError || '',
      needsUserInput: session.needsUserInput ?? session.NeedsUserInput ?? false,
    }
  }

  function sessionTimestamp(session) {
    const value = Date.parse(session?.lastStateChangeAt || '')
    return Number.isNaN(value) ? 0 : value
  }

  function buildRuntimeSessions(sourceSessions) {
    const latestByConfiguration = new Map()

    for (const item of sourceSessions) {
      const normalized = normalizeSession(item)
      if (!normalized) continue

      const existing = latestByConfiguration.get(normalized.configurationId)
      if (!existing || sessionTimestamp(normalized) >= sessionTimestamp(existing)) {
        latestByConfiguration.set(normalized.configurationId, normalized)
      }
    }

    return Array.from(latestByConfiguration.values())
  }

  function replaceRuntimeSessions(nextSessions) {
    sessions = (nextSessions || []).map((session) => normalizeSession(session)).filter(Boolean)
  }

  async function refreshRuntimeSessions() {
    try {
      replaceRuntimeSessions(await listRuntimeSessions())
    } catch {
      // Keep the last known runtime snapshot if polling fails.
    }
  }

  function startRuntimeRefreshLoop() {
    if (typeof window === 'undefined' || runtimeRefreshHandle) return

    runtimeRefreshHandle = window.setInterval(() => {
      refreshRuntimeSessions()
    }, 1500)
  }

  function stopRuntimeRefreshLoop() {
    if (typeof window === 'undefined' || !runtimeRefreshHandle) return
    window.clearInterval(runtimeRefreshHandle)
    runtimeRefreshHandle = null
  }

  function applyTheme(theme) {
    if (typeof document === 'undefined') return

    document.documentElement.dataset.theme = theme
    document.documentElement.classList.toggle('is-dark', theme === 'dark')
    document.documentElement.classList.toggle('is-light', theme !== 'dark')
  }

  const modalOpen = () => serverDialogOpen || tunnelDialogOpen || unlockDialogOpen

  function findConfigurationRecord(items, configurationId) {
    for (const item of items) {
      const configuration = item.configurations.find((entry) => entry.id === configurationId)
      if (configuration) {
        return { server: item.server, configuration }
      }
    }
    return null
  }

  function configurationsForServer(serverId) {
    return servers.find((item) => item.server.id === serverId)?.configurations || []
  }

  function firstConfigurationIdForServer(serverId) {
    return configurationsForServer(serverId)[0]?.id || ''
  }

  const activeConnections = () => sessions
    .map((session) => normalizeSession(session))
    .filter(Boolean)
    .filter((session) => !['stopped', 'failed'].includes(session.status))
    .map((session) => {
      const record = findConfigurationRecord(servers, session.configurationId)
      return {
        configurationId: session.configurationId,
        configurationLabel: record?.configuration.label || 'Unknown tunnel',
        serverName: record?.server.name || 'Unknown server',
        status: session.status,
        statusDetail: session.statusDetail,
      }
    })

  async function hydrate() {
    isHydrating = true
    try {
      const state = await loadInitialState()
      servers = state.servers || []
      preferences = state.preferences || preferences
      replaceRuntimeSessions(state.sessions || [])
      diagnostics = state.diagnostics || diagnostics
      selectedServerId = servers.some((item) => item.server.id === preferences.lastSelectedServerId) ? preferences.lastSelectedServerId : ''
      selectedConfigurationId = firstConfigurationIdForServer(selectedServerId)
      editorValue = { ...emptyConfig(), serverId: selectedServerId }
      banner = state.message || ''
      diagnosticDetails = state.message || ''
      bannerKind = state.recoverable ? 'warning' : 'info'
      loadRecoverable = Boolean(state.recoverable)
    } catch (error) {
      banner = error.message || 'The configuration store could not be loaded.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
      loadRecoverable = true
    } finally {
      isHydrating = false
    }
  }

  function upsertSession(session) {
    const normalized = normalizeSession(session)
    if (!normalized) return

    sessions = runtimeSessionSnapshot.filter((item) => item.configurationId !== normalized.configurationId).concat(normalized)
    if (normalized.status === 'needs_attention') {
      unlockConfigurationId = normalized.configurationId
      unlockConfigurationIds = Array.from(new Set(unlockConfigurationIds.concat(normalized.configurationId)))
      unlockDialogOpen = true
      return
    }

    if (unlockConfigurationId === normalized.configurationId) {
      unlockDialogOpen = false
      unlockConfigurationId = ''
      unlockConfigurationIds = unlockConfigurationIds.filter((id) => id !== normalized.configurationId)
    }
  }

  function closeUnlockDialog() {
    unlockDialogOpen = false
    unlockConfigurationId = ''
    unlockConfigurationIds = []
  }

  function validateConfig(configuration) {
    const errors = {}
    if (!configuration.label?.trim()) errors.label = 'A label is required.'
    if (configuration.connectionType === 'local_forward') {
      if (!configuration.localPort) errors.localPort = 'Local port is required.'
      if (!configuration.remoteHost?.trim()) errors.remoteHost = 'Remote host is required.'
      if (!configuration.remotePort) errors.remotePort = 'Remote port is required.'
    }
    if (configuration.connectionType === 'socks_proxy' && configuration.socksPort !== '' && configuration.socksPort !== 'auto') {
      const socksPort = Number(configuration.socksPort)
      if (!Number.isInteger(socksPort) || socksPort < 1 || socksPort > 65535) {
        errors.socksPort = 'SOCKS port must be Auto or between 1 and 65535.'
      }
    }
    return errors
  }

  function validateServer(server) {
    const errors = {}
    if (!server.name?.trim()) errors.name = 'A server name is required.'
    if (!server.host?.trim()) errors.host = 'A server host is required.'
    if (!server.username?.trim()) errors.username = 'An SSH username is required.'
    const port = Number(server.port || 0)
    if (!Number.isInteger(port) || port < 1 || port > 65535) errors.port = 'Port must be between 1 and 65535.'
    if (server.authMode === 'private_key' && !server.keyReference?.trim()) errors.keyReference = 'A private key path is required.'
    return errors
  }

  function openCreateServerDialog() {
    serverValue = emptyServer()
    serverErrors = {}
    serverDialogOpen = true
  }

  function closeServerDialog() {
    serverDialogOpen = false
    serverErrors = {}
  }

  async function handleSaveServer(server) {
    serverErrors = validateServer(server)
    if (Object.keys(serverErrors).length > 0) return

    try {
      const saved = await saveServer({
        ...server,
        port: Number(server.port || 22),
      })
      const existing = servers.find((item) => item.server.id === saved.id)
      if (existing) {
        servers = servers.map((item) => item.server.id === saved.id ? { ...item, server: { ...item.server, ...saved } } : item)
      } else {
        servers = servers.concat({ server: saved, configurations: [] })
      }
      serverDialogOpen = false
      serverErrors = {}
      await selectServer(saved.id)
      banner = `Saved ${saved.name}.`
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The server could not be saved.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  function openCreateTunnelDialog() {
    if (!selectedServerId) {
      banner = 'Select a server before adding a tunnel.'
      bannerKind = 'warning'
      return
    }
    editorValue = { ...emptyConfig(), serverId: selectedServerId }
    editorErrors = {}
    tunnelDialogOpen = true
  }

  function closeTunnelDialog() {
    tunnelDialogOpen = false
    editorErrors = {}
    editorValue = { ...emptyConfig(), serverId: selectedServerId }
  }

  function handleGlobalKeydown(event) {
    if (event.key !== 'Escape') return

    if (unlockDialogOpen) {
      closeUnlockDialog()
      return
    }

    if (tunnelDialogOpen) {
      closeTunnelDialog()
      return
    }
  }

  async function handleDeleteServer(serverId) {
    try {
      await deleteServer(serverId)
      servers = servers.filter((item) => item.server.id !== serverId)
      if (selectedServerId === serverId) {
        selectedServerId = ''
        selectedConfigurationId = ''
      }
      banner = 'Server deleted.'
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The server could not be deleted.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function handleSaveConfiguration(configuration) {
    editorErrors = validateConfig(configuration)
    if (Object.keys(editorErrors).length > 0) return
    try {
      const saved = await saveConnectionConfiguration({
        ...configuration,
        serverId: selectedServerId,
        localPort: Number(configuration.localPort || 0),
        remotePort: Number(configuration.remotePort || 0),
        socksPort: configuration.connectionType === 'socks_proxy'
          ? (configuration.socksPort === '' || configuration.socksPort === 'auto' ? 0 : Number(configuration.socksPort || 0))
          : 0,
      })
      servers = servers.map((item) => {
        if (item.server.id !== selectedServerId) return item
        const configurations = item.configurations.filter((entry) => entry.id !== saved.id).concat(saved)
        return { ...item, configurations }
      })
      selectedConfigurationId = saved.id
      tunnelDialogOpen = false
      editorValue = { ...emptyConfig(), serverId: selectedServerId }
      editorErrors = {}
      banner = ''
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be saved.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function handleDeleteConfiguration(configurationId) {
    try {
      await deleteConnectionConfiguration(configurationId)
      servers = servers.map((item) => ({
        ...item,
        configurations: item.configurations.filter((entry) => entry.id !== configurationId),
      }))
      if (selectedConfigurationId === configurationId) {
        selectedConfigurationId = firstConfigurationIdForServer(selectedServerId)
      }
      banner = 'Tunnel deleted.'
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be deleted.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function selectServer(serverId) {
    selectedServerId = serverId
    selectedConfigurationId = firstConfigurationIdForServer(serverId)
    editorValue = { ...emptyConfig(), serverId }
    try {
      preferences = await savePreferences({ ...preferences, lastSelectedServerId: serverId })
      diagnosticDetails = ''
    } catch (error) {
      banner = error.message || 'The current server selection could not be saved.'
      diagnosticDetails = error.message || ''
      bannerKind = 'warning'
    }
  }

  async function focusConfiguration(configurationId) {
    const record = findConfigurationRecord(servers, configurationId)
    if (!record) return

    selectedServerId = record.server.id
    selectedConfigurationId = configurationId
    editorValue = { ...record.configuration }

    try {
      preferences = await savePreferences({ ...preferences, lastSelectedServerId: record.server.id })
      diagnosticDetails = ''
    } catch (error) {
      banner = error.message || 'The current server selection could not be saved.'
      diagnosticDetails = error.message || ''
      bannerKind = 'warning'
    }
  }

  async function handleToggleTheme() {
    const nextTheme = preferences.theme === 'dark' ? 'light' : 'dark'
    try {
      preferences = await savePreferences({ ...preferences, theme: nextTheme })
      diagnosticDetails = ''
    } catch (error) {
      banner = error.message || 'The theme preference could not be saved.'
      diagnosticDetails = error.message || ''
      bannerKind = 'warning'
    }
  }

  async function handleStart(configurationId) {
    try {
      const session = await startConfiguration(configurationId)
      upsertSession(session)
      await refreshRuntimeSessions()
      await refreshSessionHistory(configurationId)
      banner = session.statusDetail || ''
      diagnosticDetails = ''
      bannerKind = session.status === 'failed' ? 'danger' : session.status === 'needs_attention' ? 'warning' : 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be started.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function handleStartAll() {
    if (!selectedServerId) {
      banner = 'Select a server before starting its tunnels.'
      bannerKind = 'warning'
      return
    }

    try {
      const nextSessions = await startServerConfigurations(selectedServerId)
      nextSessions.forEach((session) => upsertSession(session))
      await refreshRuntimeSessions()

      const connectedCount = nextSessions.filter((session) => session.status === 'connected').length
      const attentionSessions = nextSessions.filter((session) => session.status === 'needs_attention')
      const attentionCount = attentionSessions.length
      const failedCount = nextSessions.filter((session) => session.status === 'failed').length
      const attentionSummary = attentionSessions[0]?.statusDetail || (attentionCount ? 'A tunnel needs attention before it can continue.' : '')

      banner = [
        connectedCount ? `Started ${connectedCount} tunnel${connectedCount === 1 ? '' : 's'}.` : '',
        attentionCount ? `${attentionCount} need${attentionCount === 1 ? 's' : ''} attention. ${attentionSummary}` : '',
        failedCount ? `${failedCount} failed to start.` : '',
      ].filter(Boolean).join(' ') || 'No tunnels were started.'
      diagnosticDetails = attentionCount ? attentionSessions.map((session) => `${findConfigurationRecord(servers, session.configurationId)?.configuration.label || 'Tunnel'}: ${session.statusDetail || 'Needs attention.'}`).join('\n') : ''
      bannerKind = failedCount > 0 || attentionCount > 0 ? 'warning' : 'info'
      if (attentionCount > 0) {
        unlockConfigurationIds = attentionSessions.map((session) => session.configurationId)
        unlockConfigurationId = unlockConfigurationIds[0] || ''
        unlockDialogOpen = true
      }
    } catch (error) {
      banner = error.message || 'The selected server tunnels could not be started.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function handleStop(configurationId) {
    try {
      const session = await stopConfiguration(configurationId)
      upsertSession(session)
      await refreshRuntimeSessions()
      await refreshSessionHistory(configurationId)
      banner = session.statusDetail || ''
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be stopped.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function handleRetry(configurationId) {
    try {
      const session = await retryConfiguration(configurationId)
      upsertSession(session)
      await refreshRuntimeSessions()
      await refreshSessionHistory(configurationId)
      banner = session.statusDetail || ''
      diagnosticDetails = ''
      bannerKind = session.status === 'failed' ? 'danger' : session.status === 'needs_attention' ? 'warning' : 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be retried.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function handleUnlock(configurationId, secret) {
    try {
      const pendingUnlockIds = unlockConfigurationIds.length > 0 ? unlockConfigurationIds : [configurationId]
      const unlockedSessions = []

      for (const pendingId of pendingUnlockIds) {
        const session = await submitKeyUnlock(pendingId, secret)
        unlockedSessions.push(session)
        upsertSession(session)
      }

      await refreshRuntimeSessions()
      await refreshSessionHistory(configurationId)
      const remainingAttentionSessions = unlockedSessions.filter((session) => session.status === 'needs_attention')
      const connectedCount = unlockedSessions.filter((session) => session.status === 'connected').length

      banner = [
        connectedCount ? `Unlocked and started ${connectedCount} tunnel${connectedCount === 1 ? '' : 's'}.` : '',
        remainingAttentionSessions.length ? `${remainingAttentionSessions.length} still need attention.` : '',
      ].filter(Boolean).join(' ') || unlockedSessions[0]?.statusDetail || ''
      diagnosticDetails = remainingAttentionSessions.map((session) => `${findConfigurationRecord(servers, session.configurationId)?.configuration.label || 'Tunnel'}: ${session.statusDetail || 'Needs attention.'}`).join('\n')
      bannerKind = remainingAttentionSessions.length > 0 ? 'warning' : 'info'
      unlockConfigurationIds = remainingAttentionSessions.map((session) => session.configurationId)
      unlockConfigurationId = unlockConfigurationIds[0] || ''
      unlockDialogOpen = unlockConfigurationIds.length > 0
    } catch (error) {
      banner = error.message || 'The SSH key could not be unlocked.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function handleDiscoverBrowsers() {
    try {
      browsers = await discoverBrowsers()
      if (!browsers.some((browser) => browser.id === selectedBrowserId)) {
        selectedBrowserId = browsers[0]?.id || ''
      }
      banner = browsers.length === 0 ? 'No supported browsers were found.' : ''
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'Installed browsers could not be discovered.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function handleLaunchBrowser(configurationId, browserId) {
    try {
      await launchBrowserThroughSocks(configurationId, browserId)
      banner = 'Browser launch request sent.'
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'Browser launch failed.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function refreshBrowserPreview(configurationId, browserId) {
    if (!configurationId || !browserId || selectedConfiguration?.id !== configurationId || selectedSession?.status !== 'connected') {
      browserLaunchPreview = ''
      return
    }

    try {
      const preview = await previewBrowserLaunchThroughSocks(configurationId, browserId)
      browserLaunchPreview = preview.command || ''
    } catch {
      browserLaunchPreview = ''
    }
  }

  async function handleOpenDevTools() {
    try {
      await openDevTools()
      banner = 'Requested frontend devtools.'
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'Frontend devtools could not be opened.'
      diagnosticDetails = error.message || ''
      bannerKind = 'warning'
    }
  }

  async function handleCopyPath(label, value) {
    if (!value) return

    try {
      await navigator.clipboard.writeText(value)
      banner = `${label} copied.`
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || `${label} could not be copied.`
      diagnosticDetails = error.message || ''
      bannerKind = 'warning'
    }
  }

  async function handleReloadState() {
    banner = ''
    diagnosticDetails = ''
    bannerKind = 'info'
    await hydrate()
    await refreshSessionHistory(selectedConfigurationId)
  }

  async function handleCopyHistory(configurationId) {
    if (!configurationId || sessionHistory.length === 0) return

    const lines = sessionHistory.map((entry) => {
      const timestamp = entry.endedAt || entry.startedAt || ''
      return `[${timestamp}] ${entry.outcome}: ${entry.message}`
    })

    try {
      await navigator.clipboard.writeText(lines.join('\n'))
      banner = 'Connection history copied.'
      diagnosticDetails = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'Connection history could not be copied.'
      diagnosticDetails = error.message || ''
      bannerKind = 'warning'
    }
  }

  function editConfiguration(configuration) {
    selectedConfigurationId = configuration.id
    editorValue = { ...configuration }
    editorErrors = {}
    tunnelDialogOpen = true
  }

  function editServer(server) {
    serverValue = {
      id: server.id,
      name: server.name,
      host: server.host,
      port: String(server.port),
      username: server.username,
      authMode: server.authMode,
      keyReference: server.keyReference || '',
    }
    serverErrors = {}
    serverDialogOpen = true
  }

  onMount(() => {
    hydrate()
    startRuntimeRefreshLoop()
  })

  $: runtimeSessionSnapshot = buildRuntimeSessions(sessions)
  $: selectedServerRecord = servers.find((item) => item.server.id === selectedServerId) || null
  $: selectedServer = selectedServerRecord?.server || null
  $: selectedConfigurations = selectedServerRecord?.configurations || []
  $: if (!selectedServerId) {
    selectedConfigurationId = ''
  } else if (!selectedConfigurations.some((item) => item.id === selectedConfigurationId)) {
    selectedConfigurationId = selectedConfigurations[0]?.id || ''
  }
  $: selectedConfiguration = selectedConfigurations.find((item) => item.id === selectedConfigurationId) || null
  $: selectedSession = runtimeSessionSnapshot.find((item) => item.configurationId === selectedConfigurationId) || null
  $: unlockConfiguration = findConfigurationRecord(servers, unlockConfigurationId)?.configuration || null
  $: unlockSession = runtimeSessionSnapshot.find((item) => item.configurationId === unlockConfigurationId) || null
  $: totalTunnelCount = servers.reduce((count, item) => count + item.configurations.length, 0)
  $: connectedSessionCount = runtimeSessionSnapshot.filter((item) => item.status === 'connected').length
  $: attentionSessionCount = runtimeSessionSnapshot.filter((item) => item.status === 'needs_attention').length
  $: activeRuntimeConnections = runtimeSessionSnapshot
    .filter((session) => !['stopped', 'failed'].includes(session.status))
    .map((session) => {
      const record = findConfigurationRecord(servers, session.configurationId)
      return {
        configurationId: session.configurationId,
        configurationLabel: record?.configuration.label || 'Unknown tunnel',
        serverName: record?.server.name || 'Unknown server',
        status: session.status,
        statusDetail: session.statusDetail,
      }
    })
  $: selectedServerActiveCount = activeRuntimeConnections.filter((item) => findConfigurationRecord(servers, item.configurationId)?.server.id === selectedServerId).length
  $: canCreateTunnel = Boolean(selectedServerId)
  $: canStartSelectedServer = canCreateTunnel && selectedConfigurations.length > 0
  $: currentIssue = bannerKind === 'info' && !loadRecoverable ? '' : diagnosticDetails || banner || ''
  $: nextBrowserDiscoveryKey = selectedConfiguration?.connectionType === 'socks_proxy' ? selectedConfigurationId : ''
  $: if (nextBrowserDiscoveryKey && nextBrowserDiscoveryKey !== browserDiscoveryKey) {
    browserDiscoveryKey = nextBrowserDiscoveryKey
    handleDiscoverBrowsers()
  } else if (!nextBrowserDiscoveryKey) {
    browserDiscoveryKey = ''
    browserPreviewKey = ''
    selectedBrowserId = ''
    browsers = []
    browserLaunchPreview = ''
  }
  $: nextBrowserPreviewKey = selectedConfiguration?.connectionType === 'socks_proxy' && selectedSession?.status === 'connected' && selectedBrowserId
    ? `${selectedConfiguration.id}:${selectedBrowserId}:${selectedSession.boundPort || 0}`
    : ''
  $: if (nextBrowserPreviewKey && nextBrowserPreviewKey !== browserPreviewKey) {
    browserPreviewKey = nextBrowserPreviewKey
    refreshBrowserPreview(selectedConfiguration.id, selectedBrowserId)
  } else if (!nextBrowserPreviewKey) {
    browserPreviewKey = ''
    browserLaunchPreview = ''
  }
  $: nextHistoryConfigurationKey = selectedConfigurationId || ''
  $: if (nextHistoryConfigurationKey && nextHistoryConfigurationKey !== historyConfigurationKey) {
    historyConfigurationKey = nextHistoryConfigurationKey
    refreshSessionHistory(nextHistoryConfigurationKey)
  } else if (!nextHistoryConfigurationKey) {
    historyConfigurationKey = ''
    sessionHistory = []
  }

  $: applyTheme(preferences.theme)

  $: if (typeof document !== 'undefined') {
    document.body.style.overflow = modalOpen() ? 'hidden' : ''
  }

  onDestroy(() => {
    stopRuntimeRefreshLoop()
    if (typeof document !== 'undefined') {
      document.body.style.overflow = ''
    }
  })
</script>

<svelte:head>
  <title>SSH Man</title>
</svelte:head>

<svelte:window on:keydown={handleGlobalKeydown} />

<main class="app-shell console-shell" aria-busy={isHydrating}>
  <div class="console-main">
    <header class="console-topbar is-compact" aria-labelledby="app-heading">
      <div class="console-topbar-copy">
        <div class="console-title-block">
          <p class="eyebrow">SSH Connection Manager</p>
          <h1 id="app-heading">SSH Man</h1>
        </div>
        <div class="console-search-shell" aria-label="Current workspace context">
          <span class="console-search-kicker">Workspace</span>
          <span class="console-search-value">{selectedServer ? `${selectedServer.username}@${selectedServer.host}:${selectedServer.port}` : 'Select a target to start managing tunnels'}</span>
        </div>
      </div>

      <div class="console-topbar-actions">
        <button class="p-button--base is-dense" type="button" on:click={handleReloadState}>Reload</button>
        <button class="p-button--base is-dense" type="button" on:click={openCreateServerDialog}>Server</button>
        <button class="p-button--positive is-dense" type="button" disabled={!canCreateTunnel} on:click={openCreateTunnelDialog}>Tunnel</button>
        <ThemeToggle theme={preferences.theme} onToggle={handleToggleTheme} />
      </div>
    </header>

    {#if banner}
      <div
        class={`p-notification banner ${bannerKind === 'danger' ? 'p-notification--negative' : bannerKind === 'warning' ? 'p-notification--caution' : ''}`}
        aria-live={bannerKind === 'danger' ? 'assertive' : 'polite'}
        role={bannerKind === 'danger' ? 'alert' : 'status'}
      >
        <div class="p-notification__content">
          <p class="p-notification__message"><strong>{banner}</strong></p>
          {#if diagnosticDetails && diagnosticDetails !== banner}
            <pre class="banner-detail">{diagnosticDetails}</pre>
          {/if}
        </div>
      </div>
    {/if}

    <section class="console-stage">
      <div class="console-pane console-pane--left" id="targets-pane">
        <ServerList
          {servers}
          {selectedServerId}
          onSelect={selectServer}
          onCreate={openCreateServerDialog}
          onEdit={editServer}
          onDelete={handleDeleteServer}
        />

        <DiagnosticsPanel
          diagnostics={diagnostics}
          issue={currentIssue}
          hasWarning={loadRecoverable || bannerKind !== 'info'}
          onReload={handleReloadState}
          onOpenDevTools={handleOpenDevTools}
          onCopyPath={handleCopyPath}
        />
      </div>

      <div class="console-pane console-pane--center" id="tunnels-pane">
        <section class="p-card panel console-overview" aria-labelledby="summary-heading">
          <div class="console-overview-header">
            <div>
              <p class="eyebrow">Workspace</p>
              <h2 id="summary-heading">{selectedServer?.name || 'Select a server to continue'}</h2>
              <p class="panel-copy">
                {#if selectedServer}
                  {selectedServer.username}@{selectedServer.host}:{selectedServer.port}
                {:else}
                  Create or select a server to unlock the main workspace.
                {/if}
              </p>
            </div>

            <div class="console-overview-actions">
              <button class="p-button--base" type="button" disabled={!canStartSelectedServer} on:click={handleStartAll}>Start all</button>
            </div>
          </div>

          <div class="summary-stats" aria-label="Workspace summary">
            <span class="p-chip is-inline">
              <span class="p-chip__lead">Servers</span>
              <span class="p-chip__value">{servers.length}</span>
            </span>
            <span class="p-chip is-inline">
              <span class="p-chip__lead">Tunnels</span>
              <span class="p-chip__value">{totalTunnelCount}</span>
            </span>
            <span class={`p-chip is-inline ${connectedSessionCount > 0 ? 'p-chip--positive' : ''}`}>
              <span class="p-chip__lead">Connected</span>
              <span class="p-chip__value">{connectedSessionCount}</span>
            </span>
            <span class={`p-chip is-inline ${attentionSessionCount > 0 ? 'p-chip--caution' : ''}`}>
              <span class="p-chip__lead">Attention</span>
              <span class="p-chip__value">{attentionSessionCount}</span>
            </span>
          </div>

          {#if selectedServer}
            <div class="p-card--highlighted selection-summary" aria-label="Selected server summary">
              <span class="selection-label">Focused target</span>
              <p><strong>{selectedServer.name}</strong></p>
              <p class="panel-copy">{selectedConfigurations.length} saved tunnel{selectedConfigurations.length === 1 ? '' : 's'} and {selectedServerActiveCount} active runtime session{selectedServerActiveCount === 1 ? '' : 's'}.</p>
            </div>
          {/if}
        </section>

        <ConfigList
          enabled={Boolean(selectedServerId)}
          configurations={selectedConfigurations}
          {selectedConfigurationId}
          sessions={runtimeSessionSnapshot}
          onSelect={focusConfiguration}
          onCreate={openCreateTunnelDialog}
          onStartAll={handleStartAll}
          onEdit={editConfiguration}
          onDelete={handleDeleteConfiguration}
        />

        <ActiveConnections connections={activeRuntimeConnections} onSelect={focusConfiguration} onStop={handleStop} />
      </div>

      <div class="console-pane console-pane--right" id="runtime-pane">
        {#if selectedConfiguration?.connectionType === 'socks_proxy'}
          <BrowserLauncher
            configuration={selectedConfiguration}
            session={selectedSession}
            {browsers}
            {selectedBrowserId}
            launchPreview={browserLaunchPreview}
            onSelect={(id) => (selectedBrowserId = id)}
            onRefresh={handleDiscoverBrowsers}
            onLaunch={handleLaunchBrowser}
          />
        {/if}

        <SessionStatus
          configuration={selectedConfiguration}
          session={selectedSession}
          history={sessionHistory}
          onStart={handleStart}
          onStop={handleStop}
          onCopyHistory={handleCopyHistory}
        />
      </div>
    </section>
  </div>

  <ServerEditorDialog
    open={serverDialogOpen}
    value={serverValue}
    errors={serverErrors}
    onSubmit={handleSaveServer}
    onCancel={closeServerDialog}
  />

  {#if tunnelDialogOpen}
    <div class="p-modal dialog-backdrop" role="presentation" on:click|self={closeTunnelDialog}>
      <div class="p-modal__dialog dialog-card dialog" aria-modal="true" role="dialog" aria-labelledby="tunnel-dialog-heading">
        <ConfigEditor
          server={selectedServer}
          value={editorValue}
          errors={editorErrors}
          onSubmit={handleSaveConfiguration}
          onCancel={closeTunnelDialog}
        />
      </div>
    </div>
  {/if}

  <UnlockKeyDialog
    open={unlockDialogOpen}
    configurationLabel={unlockConfiguration?.label || ''}
    detail={unlockSession?.statusDetail || ''}
    onSubmit={(secret) => handleUnlock(unlockConfigurationId, secret)}
    onClose={closeUnlockDialog}
  />
</main>
