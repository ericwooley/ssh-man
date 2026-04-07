<script>
  import { onDestroy, onMount } from 'svelte'
  import {
    deleteConnectionConfiguration,
    deleteServer,
    discoverBrowsers,
    launchBrowserThroughSocks,
    loadInitialState,
    openDevTools,
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
    authMode: 'private_key',
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
  let banner = ''
  let bannerKind = 'info'
  let loadRecoverable = false
  let isHydrating = false
  let unlockDialogOpen = false
  let unlockConfigurationId = ''
  let diagnosticDetails = ''
  let diagnostics = { appDataPath: '', databasePath: '' }
  let selectedServerRecord = null
  let selectedServer = null
  let selectedConfigurations = []
  let selectedConfiguration = null
  let selectedSession = null

  function normalizeSession(session) {
    if (!session) return null

    const configurationId = session.configurationId || session.ConfigurationID || ''
    if (!configurationId) return null

    return {
      configurationId,
      status: session.status || session.Status || 'stopped',
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

  function runtimeSessions() {
    const latestByConfiguration = new Map()

    for (const item of sessions) {
      const normalized = normalizeSession(item)
      if (!normalized) continue

      const existing = latestByConfiguration.get(normalized.configurationId)
      if (!existing || sessionTimestamp(normalized) >= sessionTimestamp(existing)) {
        latestByConfiguration.set(normalized.configurationId, normalized)
      }
    }

    return Array.from(latestByConfiguration.values())
  }

  function runtimeSessionFor(configurationId) {
    return runtimeSessions().find((item) => item.configurationId === configurationId) || null
  }

  const unlockConfiguration = () => configurationRecord(unlockConfigurationId)?.configuration || null
  const unlockSession = () => runtimeSessionFor(unlockConfigurationId)
  const totalTunnelCount = () => servers.reduce((count, item) => count + item.configurations.length, 0)
  const connectedSessionCount = () => runtimeSessions().filter((item) => item.status === 'connected').length
  const attentionSessionCount = () => runtimeSessions().filter((item) => item.status === 'needs_attention').length
  const selectedServerActiveCount = () => activeConnections().filter((item) => configurationRecord(item.configurationId)?.server.id === selectedServerId).length
  const currentIssue = () => bannerKind === 'info' && !loadRecoverable ? '' : diagnosticDetails || banner || ''
  const modalOpen = () => serverDialogOpen || tunnelDialogOpen || unlockDialogOpen

  function configurationRecord(configurationId) {
    for (const item of servers) {
      const configuration = item.configurations.find((entry) => entry.id === configurationId)
      if (configuration) {
        return { server: item.server, configuration }
      }
    }
    return null
  }

  const activeConnections = () => sessions
    .map((session) => normalizeSession(session))
    .filter(Boolean)
    .filter((session) => !['stopped', 'failed'].includes(session.status))
    .map((session) => {
      const record = configurationRecord(session.configurationId)
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
      sessions = state.sessions || []
      diagnostics = state.diagnostics || diagnostics
      selectedServerId = servers.some((item) => item.server.id === preferences.lastSelectedServerId) ? preferences.lastSelectedServerId : ''
      selectedConfigurationId = selectedConfigurations[0]?.id || ''
      editorValue = { ...emptyConfig(), serverId: selectedServerId }
      document.documentElement.dataset.theme = preferences.theme
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

    sessions = runtimeSessions().filter((item) => item.configurationId !== normalized.configurationId).concat(normalized)
    if (normalized.status === 'needs_attention') {
      unlockConfigurationId = normalized.configurationId
      unlockDialogOpen = true
      return
    }

    if (unlockConfigurationId === normalized.configurationId) {
      unlockDialogOpen = false
      unlockConfigurationId = ''
    }
  }

  function closeUnlockDialog() {
    unlockDialogOpen = false
    unlockConfigurationId = ''
  }

  function validateConfig(configuration) {
    const errors = {}
    if (!configuration.label?.trim()) errors.label = 'A label is required.'
    if (configuration.connectionType === 'local_forward') {
      if (!configuration.localPort) errors.localPort = 'Local port is required.'
      if (!configuration.remoteHost?.trim()) errors.remoteHost = 'Remote host is required.'
      if (!configuration.remotePort) errors.remotePort = 'Remote port is required.'
    }
    if (configuration.connectionType === 'socks_proxy' && !configuration.socksPort) errors.socksPort = 'SOCKS port is required.'
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
        socksPort: Number(configuration.socksPort || 0),
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
        selectedConfigurationId = selectedConfigurations[0]?.id || ''
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
    selectedConfigurationId = selectedConfigurations[0]?.id || ''
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
    const record = configurationRecord(configurationId)
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
      document.documentElement.dataset.theme = nextTheme
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

      const connectedCount = nextSessions.filter((session) => session.status === 'connected').length
      const attentionCount = nextSessions.filter((session) => session.status === 'needs_attention').length
      const failedCount = nextSessions.filter((session) => session.status === 'failed').length

      banner = [
        connectedCount ? `Started ${connectedCount} tunnel${connectedCount === 1 ? '' : 's'}.` : '',
        attentionCount ? `${attentionCount} need${attentionCount === 1 ? 's' : ''} a passphrase.` : '',
        failedCount ? `${failedCount} failed to start.` : '',
      ].filter(Boolean).join(' ') || 'No tunnels were started.'
      diagnosticDetails = ''
      bannerKind = failedCount > 0 || attentionCount > 0 ? 'warning' : 'info'
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
      const session = await submitKeyUnlock(configurationId, secret)
      upsertSession(session)
      banner = session.statusDetail || ''
      diagnosticDetails = ''
      bannerKind = session.status === 'failed' ? 'danger' : session.status === 'needs_attention' ? 'warning' : 'info'
    } catch (error) {
      banner = error.message || 'The SSH key could not be unlocked.'
      diagnosticDetails = error.message || ''
      bannerKind = 'danger'
    }
  }

  async function handleDiscoverBrowsers() {
    try {
      browsers = await discoverBrowsers()
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
  })

  $: selectedServerRecord = servers.find((item) => item.server.id === selectedServerId) || null
  $: selectedServer = selectedServerRecord?.server || null
  $: selectedConfigurations = selectedServerRecord?.configurations || []
  $: selectedConfiguration = selectedConfigurations.find((item) => item.id === selectedConfigurationId) || null
  $: selectedSession = runtimeSessionFor(selectedConfigurationId)

  $: if (typeof document !== 'undefined') {
    document.body.style.overflow = modalOpen() ? 'hidden' : ''
  }

  onDestroy(() => {
    if (typeof document !== 'undefined') {
      document.body.style.overflow = ''
    }
  })
</script>

<svelte:head>
  <title>SSH Man</title>
</svelte:head>

<svelte:window on:keydown={handleGlobalKeydown} />

<main class="app-shell" aria-busy={isHydrating}>
  <header class="hero">
    <div>
      <p class="eyebrow">SSH Connection Manager</p>
      <h1>Persistent tunnels without the command-line churn</h1>
      <p class="hero-copy">Save servers, reuse SOCKS and local forwards, and recover faster when sessions drop.</p>
    </div>
    <ThemeToggle theme={preferences.theme} onToggle={handleToggleTheme} />
  </header>

  {#if banner}
    <div class={`banner banner-${bannerKind}`} aria-live={bannerKind === 'danger' ? 'assertive' : 'polite'} role={bannerKind === 'danger' ? 'alert' : 'status'}>
      <div class="banner-content">
        <strong>{banner}</strong>
      </div>
    </div>
  {/if}

  <section class="dashboard-shell">
    <aside class="dashboard-sidebar">
      <section class="panel summary-panel" aria-labelledby="summary-heading">
        <div class="panel-header panel-header-stack">
          <div>
            <p class="eyebrow">Workspace</p>
            <h2 id="summary-heading">Current snapshot</h2>
            <p class="panel-copy">Use the left rail to move between servers. The main workspace stays focused on the selected target.</p>
          </div>
        </div>

        <div class="metric-grid" aria-label="Workspace summary">
          <div class="metric-card">
            <span class="metric-label">Servers</span>
            <strong>{servers.length}</strong>
          </div>
          <div class="metric-card">
            <span class="metric-label">Saved tunnels</span>
            <strong>{totalTunnelCount()}</strong>
          </div>
          <div class="metric-card">
            <span class="metric-label">Connected</span>
            <strong>{connectedSessionCount()}</strong>
          </div>
          <div class="metric-card">
            <span class="metric-label">Need attention</span>
            <strong>{attentionSessionCount()}</strong>
          </div>
        </div>

        {#if selectedServer}
          <div class="selection-card" aria-label="Selected server summary">
            <div>
              <span class="selection-label">Selected server</span>
              <strong>{selectedServer.name}</strong>
            </div>
            <small>{selectedServer.username}@{selectedServer.host}:{selectedServer.port}</small>
            <small>{selectedConfigurations.length} saved tunnel{selectedConfigurations.length === 1 ? '' : 's'} and {selectedServerActiveCount()} active.</small>
          </div>
        {:else}
          <div class="empty-state compact">
            <h3>No server selected</h3>
            <p>Create or select a server to unlock the main workspace.</p>
          </div>
        {/if}
      </section>

      <ServerList
        {servers}
        {selectedServerId}
        onSelect={selectServer}
        onCreate={openCreateServerDialog}
        onEdit={editServer}
        onDelete={handleDeleteServer}
      />

      <DiagnosticsPanel
        {diagnostics}
        issue={currentIssue()}
        hasWarning={loadRecoverable || bannerKind !== 'info'}
        onReload={handleReloadState}
        onOpenDevTools={handleOpenDevTools}
        onCopyPath={handleCopyPath}
      />
    </aside>

    <div class="workspace-main">
      <section class="panel workspace-header" aria-labelledby="workspace-heading">
        <div class="workspace-header-copy">
          <p class="eyebrow">Primary workspace</p>
          <h2 id="workspace-heading">{selectedServer?.name || 'Select a server to continue'}</h2>
          <p class="panel-copy">
            {#if selectedServer}
              Save, start, and troubleshoot tunnels for one host without losing track of runtime state.
            {:else}
              Choose a saved server from the sidebar to manage its tunnels and launch browsers through SOCKS.
            {/if}
          </p>
        </div>
      </section>

      <div class="workspace-main-grid">
        <div class="workspace-primary-column">
          <ConfigList
            enabled={Boolean(selectedServerId)}
            configurations={selectedConfigurations}
            {selectedConfigurationId}
            sessions={runtimeSessions()}
            onSelect={focusConfiguration}
            onCreate={openCreateTunnelDialog}
            onStartAll={handleStartAll}
            onEdit={editConfiguration}
            onDelete={handleDeleteConfiguration}
          />

          <ActiveConnections connections={activeConnections()} onSelect={focusConfiguration} onStop={handleStop} />
        </div>

        <div class="workspace-secondary-column">
          <SessionStatus
            configuration={selectedConfiguration}
            session={selectedSession}
            onStart={handleStart}
            onStop={handleStop}
            onRetry={handleRetry}
          />

          {#if selectedConfiguration?.connectionType === 'socks_proxy'}
            <BrowserLauncher
              configuration={selectedConfiguration}
              session={selectedSession}
              {browsers}
              {selectedBrowserId}
              onSelect={(id) => (selectedBrowserId = id)}
              onRefresh={handleDiscoverBrowsers}
              onLaunch={handleLaunchBrowser}
            />
          {/if}
        </div>
      </div>
    </div>
  </section>

  <ServerEditorDialog
    open={serverDialogOpen}
    value={serverValue}
    errors={serverErrors}
    onSubmit={handleSaveServer}
    onCancel={closeServerDialog}
  />

  {#if tunnelDialogOpen}
    <div class="dialog-backdrop" role="presentation" on:click|self={closeTunnelDialog}>
      <div class="dialog-card" aria-modal="true" role="dialog" aria-labelledby="tunnel-dialog-heading">
        <div class="sr-only" id="tunnel-dialog-heading">Tunnel editor</div>
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
    configurationLabel={unlockConfiguration()?.label || ''}
    detail={unlockSession()?.statusDetail || ''}
    onSubmit={(secret) => handleUnlock(unlockConfigurationId, secret)}
    onClose={closeUnlockDialog}
  />
</main>
