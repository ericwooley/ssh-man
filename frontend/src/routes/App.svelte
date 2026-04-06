<script>
  import { onMount } from 'svelte'
  import {
    deleteConnectionConfiguration,
    deleteServer,
    discoverBrowsers,
    launchBrowserThroughSocks,
    loadInitialState,
    retryConfiguration,
    saveConnectionConfiguration,
    savePreferences,
    saveServer,
    startConfiguration,
    stopConfiguration,
    submitKeyUnlock,
  } from '../lib/api'
  import ServerList from '../components/ServerList.svelte'
  import ConfigList from '../components/ConfigList.svelte'
  import ConfigEditor from '../components/ConfigEditor.svelte'
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

  let servers = []
  let preferences = { theme: 'dark', lastSelectedServerId: '' }
  let sessions = []
  let selectedServerId = ''
  let selectedConfigurationId = ''
  let editorValue = emptyConfig()
  let editorErrors = {}
  let browsers = []
  let selectedBrowserId = ''
  let banner = ''
  let bannerKind = 'info'
  let loadRecoverable = false
  let isHydrating = false
  let unlockDialogOpen = false

  const selectedServer = () => servers.find((item) => item.server.id === selectedServerId)?.server || null
  const selectedConfigurations = () => servers.find((item) => item.server.id === selectedServerId)?.configurations || []
  const selectedConfiguration = () => selectedConfigurations().find((item) => item.id === selectedConfigurationId) || null
  const selectedSession = () => sessions.find((item) => item.configurationId === selectedConfigurationId) || null

  async function hydrate() {
    isHydrating = true
    try {
      const state = await loadInitialState()
      servers = state.servers || []
      preferences = state.preferences || preferences
      sessions = state.sessions || []
      selectedServerId = preferences.lastSelectedServerId || servers[0]?.server.id || ''
      selectedConfigurationId = selectedConfigurations()[0]?.id || ''
      editorValue = { ...emptyConfig(), serverId: selectedServerId }
      document.documentElement.dataset.theme = preferences.theme
      banner = state.message || ''
      bannerKind = state.recoverable ? 'warning' : 'info'
      loadRecoverable = Boolean(state.recoverable)
    } catch (error) {
      banner = error.message || 'The configuration store could not be loaded.'
      bannerKind = 'danger'
      loadRecoverable = true
    } finally {
      isHydrating = false
    }
  }

  function upsertSession(session) {
    sessions = sessions.filter((item) => item.configurationId !== session.configurationId).concat(session)
    unlockDialogOpen = session.status === 'needs_attention'
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

  async function handleCreateServer() {
    try {
      const name = prompt('Server name')
      if (!name) return
      const host = prompt('SSH host')
      const username = prompt('SSH username')
      const keyReference = prompt('SSH private key path')
      const port = Number(prompt('SSH port', '22') || '22')
      const server = await saveServer({ name, host, username, keyReference, port, authMode: 'private_key' })
      servers = servers.concat({ server, configurations: [] })
      await selectServer(server.id)
      banner = `Saved ${server.name}.`
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The server could not be saved.'
      bannerKind = 'danger'
    }
  }

  async function handleDeleteServer(serverId) {
    try {
      await deleteServer(serverId)
      servers = servers.filter((item) => item.server.id !== serverId)
      if (selectedServerId === serverId) {
        selectedServerId = servers[0]?.server.id || ''
        selectedConfigurationId = selectedConfigurations()[0]?.id || ''
      }
      banner = 'Server deleted.'
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The server could not be deleted.'
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
      editorValue = { ...emptyConfig(), serverId: selectedServerId }
      banner = ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be saved.'
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
        selectedConfigurationId = selectedConfigurations()[0]?.id || ''
      }
      banner = 'Tunnel deleted.'
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be deleted.'
      bannerKind = 'danger'
    }
  }

  async function selectServer(serverId) {
    selectedServerId = serverId
    selectedConfigurationId = selectedConfigurations()[0]?.id || ''
    editorValue = { ...emptyConfig(), serverId }
    try {
      preferences = await savePreferences({ ...preferences, lastSelectedServerId: serverId })
    } catch (error) {
      banner = error.message || 'The current server selection could not be saved.'
      bannerKind = 'warning'
    }
  }

  async function handleToggleTheme() {
    const nextTheme = preferences.theme === 'dark' ? 'light' : 'dark'
    try {
      preferences = await savePreferences({ ...preferences, theme: nextTheme })
      document.documentElement.dataset.theme = nextTheme
    } catch (error) {
      banner = error.message || 'The theme preference could not be saved.'
      bannerKind = 'warning'
    }
  }

  async function handleStart(configurationId) {
    try {
      const session = await startConfiguration(configurationId)
      upsertSession(session)
      banner = session.statusDetail || ''
      bannerKind = session.status === 'failed' ? 'danger' : session.status === 'needs_attention' ? 'warning' : 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be started.'
      bannerKind = 'danger'
    }
  }

  async function handleStop(configurationId) {
    try {
      const session = await stopConfiguration(configurationId)
      upsertSession(session)
      banner = session.statusDetail || ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be stopped.'
      bannerKind = 'danger'
    }
  }

  async function handleRetry(configurationId) {
    try {
      const session = await retryConfiguration(configurationId)
      upsertSession(session)
      banner = session.statusDetail || ''
      bannerKind = session.status === 'failed' ? 'danger' : session.status === 'needs_attention' ? 'warning' : 'info'
    } catch (error) {
      banner = error.message || 'The tunnel could not be retried.'
      bannerKind = 'danger'
    }
  }

  async function handleUnlock(configurationId, secret) {
    try {
      const session = await submitKeyUnlock(configurationId, secret)
      upsertSession(session)
      banner = session.statusDetail || ''
      bannerKind = session.status === 'failed' ? 'danger' : session.status === 'needs_attention' ? 'warning' : 'info'
    } catch (error) {
      banner = error.message || 'The SSH key could not be unlocked.'
      bannerKind = 'danger'
    }
  }

  async function handleDiscoverBrowsers() {
    try {
      browsers = await discoverBrowsers()
      banner = browsers.length === 0 ? 'No supported browsers were found.' : ''
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'Installed browsers could not be discovered.'
      bannerKind = 'danger'
    }
  }

  async function handleLaunchBrowser(configurationId, browserId) {
    try {
      await launchBrowserThroughSocks(configurationId, browserId)
      banner = 'Browser launch request sent.'
      bannerKind = 'info'
    } catch (error) {
      banner = error.message || 'Browser launch failed.'
      bannerKind = 'danger'
    }
  }

  async function handleReloadState() {
    banner = ''
    bannerKind = 'info'
    await hydrate()
  }

  function editConfiguration(configuration) {
    selectedConfigurationId = configuration.id
    editorValue = { ...configuration }
  }

  function editServer(server) {
    banner = `Editing ${server.name} is supported by selecting and resaving its details.`
    bannerKind = 'info'
  }

  onMount(() => {
    hydrate()
  })
</script>

<svelte:head>
  <title>SSH Man</title>
</svelte:head>

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
    <div class={`banner banner-${bannerKind}`} aria-live={bannerKind === 'danger' ? 'assertive' : 'polite'} role={bannerKind === 'danger' ? 'alert' : 'status'}>{banner}</div>
    {#if loadRecoverable}
      <div class="banner-actions">
        <button class="button button-ghost" type="button" on:click={handleReloadState}>Retry loading saved data</button>
      </div>
    {/if}
  {/if}

  <section class="workspace-grid">
    <ServerList
      {servers}
      {selectedServerId}
      onSelect={selectServer}
      onCreate={handleCreateServer}
      onEdit={editServer}
      onDelete={handleDeleteServer}
    />

    <ConfigList
      configurations={selectedConfigurations()}
      {selectedConfigurationId}
      {sessions}
      onSelect={(id) => {
        selectedConfigurationId = id
        const configuration = selectedConfigurations().find((item) => item.id === id)
        if (configuration) editorValue = { ...configuration }
      }}
      onCreate={() => { editorValue = { ...emptyConfig(), serverId: selectedServerId } }}
      onEdit={editConfiguration}
      onDelete={handleDeleteConfiguration}
    />

    <div class="detail-column">
      <ConfigEditor
        server={selectedServer()}
        value={editorValue}
        errors={editorErrors}
        onSubmit={handleSaveConfiguration}
        onCancel={() => { editorValue = { ...emptyConfig(), serverId: selectedServerId }; editorErrors = {} }}
      />

      <SessionStatus
        configuration={selectedConfiguration()}
        session={selectedSession()}
        onStart={handleStart}
        onStop={handleStop}
        onRetry={handleRetry}
        onUnlock={handleUnlock}
      />

      <BrowserLauncher
        configuration={selectedConfiguration()}
        session={selectedSession()}
        {browsers}
        {selectedBrowserId}
        onSelect={(id) => (selectedBrowserId = id)}
        onRefresh={handleDiscoverBrowsers}
        onLaunch={handleLaunchBrowser}
      />
    </div>
  </section>

  <UnlockKeyDialog open={unlockDialogOpen} onClose={() => (unlockDialogOpen = false)} />
</main>
