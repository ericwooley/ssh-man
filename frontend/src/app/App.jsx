import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { CircleAlert, Server } from 'lucide-react'
import {
  AppHeader,
  BottomNav,
  EmptyState,
  LoadingScreen,
  ToastRegion,
} from '../components/AppChrome'
import { ConfirmDialog, UnlockDialog } from '../components/Dialogs'
import { BrowserSwitcher } from '../components/BrowserSwitcher'
import { ActivityScreen, SettingsScreen } from '../screens/ActivitySettingsScreens'
import { ServerFormScreen, TunnelFormScreen } from '../screens/FormScreens'
import { ServerDetailScreen, ServersScreen } from '../screens/ServersScreens'
import { TunnelDetailScreen } from '../screens/TunnelScreen'
import {
  browserSelectionIndexForDirections,
  emptyServer,
  emptyTunnel,
  isManagedSOCKSConfiguration,
  moveBrowserSelectionIndex,
  normalizeBrowserSwitchDirection,
  orderBrowsersByRecentActivation,
  recordBrowserTargetActivation,
} from '../model/appModel'
import * as defaultApi from '../lib/api'
import { useSshMan } from './useSshMan'

const rootCopy = {
  servers: { title: 'SSH Man', subtitle: 'Servers and tunnels' },
  activity: { title: 'Active tunnels', subtitle: 'Live connections' },
  settings: { title: 'Settings', subtitle: 'Appearance and app health' },
}

function emptyBrowserSwitcherState() {
  return {
    open: false,
    items: [],
    selectedIndex: 0,
    loading: false,
    activating: false,
    error: '',
    sessionId: '',
    mode: 'shortcut',
  }
}

export default function App({ api = defaultApi, controllerOptions }) {
  const app = useSshMan(api, controllerOptions)
  const [route, setRoute] = useState({ type: 'root', tab: 'servers' })
  const [form, setForm] = useState(null)
  const [confirmation, setConfirmation] = useState(null)
  const [browserSwitcher, setBrowserSwitcher] = useState(emptyBrowserSwitcherState)
  const browserSwitcherRef = useRef(browserSwitcher)
  const pendingBrowserSwitcherDirectionsRef = useRef({ sessionId: '', directions: [] })
  const pendingBrowserSwitcherCommitSessionRef = useRef('')
  const recentBrowserTargetIdsRef = useRef([])
  const browserSwitcherLoadIdRef = useRef(0)
  const browserSwitcherActivationIdRef = useRef(0)
  const browserSwitcherSessionCounterRef = useRef(0)

  const updateBrowserSwitcher = useCallback((update) => {
    const next = typeof update === 'function' ? update(browserSwitcherRef.current) : update
    browserSwitcherRef.current = next
    setBrowserSwitcher(next)
    return next
  }, [])

  const resetBrowserSwitcher = useCallback(() => {
    browserSwitcherLoadIdRef.current += 1
    browserSwitcherActivationIdRef.current += 1
    pendingBrowserSwitcherDirectionsRef.current = { sessionId: '', directions: [] }
    pendingBrowserSwitcherCommitSessionRef.current = ''
    updateBrowserSwitcher(emptyBrowserSwitcherState())
  }, [updateBrowserSwitcher])

  const refreshBrowserSwitcher = useCallback(async ({
    cycleIfOpen = false,
    direction = 'forward',
    sessionId = '',
    mode = 'shortcut',
  } = {}) => {
    const normalizedDirection = normalizeBrowserSwitchDirection(direction)
    const current = browserSwitcherRef.current
    const requestedSessionId = String(sessionId || current.sessionId || '')
    if (!requestedSessionId) return
    const sameSession = current.open && current.sessionId === requestedSessionId

    if (cycleIfOpen && sameSession) {
      if (current.activating) return
      if (current.loading) {
        const pending = pendingBrowserSwitcherDirectionsRef.current
        if (pending.sessionId === requestedSessionId) {
          pending.directions.push(normalizedDirection)
        }
        return
      }
      if (current.error) return
      if (current.items.length) {
        updateBrowserSwitcher((state) => ({
          ...state,
          selectedIndex: moveBrowserSelectionIndex(state.selectedIndex, state.items.length, normalizedDirection),
        }))
        return
      }
    }
    if (sameSession && current.loading) return

    if (!sameSession) {
      browserSwitcherActivationIdRef.current += 1
      pendingBrowserSwitcherCommitSessionRef.current = ''
    }

    const loadId = ++browserSwitcherLoadIdRef.current
    pendingBrowserSwitcherDirectionsRef.current = {
      sessionId: requestedSessionId,
      directions: [normalizedDirection],
    }
    updateBrowserSwitcher({
      open: true,
      items: sameSession ? current.items : [],
      selectedIndex: sameSession ? current.selectedIndex : 0,
      loading: true,
      activating: false,
      error: '',
      sessionId: requestedSessionId,
      mode: sameSession ? current.mode : mode,
    })
    try {
      const items = await api.listRunningBrowsers()
      if (loadId !== browserSwitcherLoadIdRef.current || browserSwitcherRef.current.sessionId !== requestedSessionId) return
      const nextItems = orderBrowsersByRecentActivation(items || [], recentBrowserTargetIdsRef.current)
      const pending = pendingBrowserSwitcherDirectionsRef.current
      const directions = pending.sessionId === requestedSessionId ? pending.directions : [normalizedDirection]
      pendingBrowserSwitcherDirectionsRef.current = { sessionId: '', directions: [] }
      const mostRecentTargetId = recentBrowserTargetIdsRef.current[0]
      const currentTargetIndex = mostRecentTargetId
        ? nextItems.findIndex((item) => item.id === mostRecentTargetId)
        : -1
      updateBrowserSwitcher({
        open: true,
        items: nextItems,
        selectedIndex: browserSelectionIndexForDirections(nextItems.length, directions, currentTargetIndex),
        loading: false,
        activating: false,
        error: '',
        sessionId: requestedSessionId,
        mode: sameSession ? current.mode : mode,
      })
    } catch (error) {
      if (loadId !== browserSwitcherLoadIdRef.current || browserSwitcherRef.current.sessionId !== requestedSessionId) return
      pendingBrowserSwitcherDirectionsRef.current = { sessionId: '', directions: [] }
      updateBrowserSwitcher({
        open: true,
        items: [],
        selectedIndex: 0,
        loading: false,
        activating: false,
        error: error.message || 'Running browsers could not be listed.',
        sessionId: requestedSessionId,
        mode: sameSession ? current.mode : mode,
      })
    }
  }, [api, updateBrowserSwitcher])

  const openBrowserSwitcher = useCallback(async () => {
    const sessionId = `manual-${++browserSwitcherSessionCounterRef.current}`
    try {
      await api.showBrowserSwitcherWindow?.()
    } finally {
      await refreshBrowserSwitcher({ sessionId, mode: 'manual' })
    }
  }, [api, refreshBrowserSwitcher])

  const closeBrowserSwitcher = useCallback(() => {
    resetBrowserSwitcher()
    void api.hideApplicationWindow?.()
  }, [api, resetBrowserSwitcher])

  const activateBrowserTarget = useCallback(async (target, sessionId = browserSwitcherRef.current.sessionId) => {
    const current = browserSwitcherRef.current
    if (!target || !sessionId || current.sessionId !== sessionId || current.activating) return
    const activationId = ++browserSwitcherActivationIdRef.current
    pendingBrowserSwitcherCommitSessionRef.current = ''
    updateBrowserSwitcher((state) => ({ ...state, activating: true, error: '' }))
    try {
      await api.activateRunningBrowser(target.id)
      if (activationId !== browserSwitcherActivationIdRef.current || browserSwitcherRef.current.sessionId !== sessionId) return
      recentBrowserTargetIdsRef.current = recordBrowserTargetActivation(recentBrowserTargetIdsRef.current, target.id)
      browserSwitcherLoadIdRef.current += 1
      pendingBrowserSwitcherDirectionsRef.current = { sessionId: '', directions: [] }
      updateBrowserSwitcher(emptyBrowserSwitcherState())
      await api.hideApplicationWindow?.()
    } catch (error) {
      if (activationId !== browserSwitcherActivationIdRef.current || browserSwitcherRef.current.sessionId !== sessionId) return
      updateBrowserSwitcher((state) => ({ ...state, activating: false, error: error.message || 'The browser is no longer available.' }))
    }
  }, [api, updateBrowserSwitcher])

  const commitBrowserSwitcher = useCallback((request = {}) => {
    const current = browserSwitcherRef.current
    const requestedSessionId = typeof request === 'object' ? String(request.sessionId || '') : ''
    const sessionId = requestedSessionId || current.sessionId
    if (!current.open || current.mode !== 'shortcut' || current.sessionId !== sessionId || current.activating) return
    if (current.loading) {
      pendingBrowserSwitcherCommitSessionRef.current = sessionId
      return
    }
    void activateBrowserTarget(current.items[current.selectedIndex], sessionId)
  }, [activateBrowserTarget])

  const cancelBrowserSwitcher = useCallback((request = {}) => {
    const current = browserSwitcherRef.current
    const requestedSessionId = typeof request === 'object' ? String(request.sessionId || '') : ''
    const sessionId = requestedSessionId || current.sessionId
    if (!current.open || current.mode !== 'shortcut' || current.sessionId !== sessionId) return
    resetBrowserSwitcher()
  }, [resetBrowserSwitcher])

  useEffect(() => {
    if (!api.onBrowserSwitcherRequested) return undefined
    return api.onBrowserSwitcherRequested((request = {}) => {
      const current = browserSwitcherRef.current
      const direction = typeof request === 'object' ? request.direction : request
      const requestedSessionId = typeof request === 'object' ? String(request.sessionId || '') : ''
      const sessionId = requestedSessionId || (
        current.open && current.mode === 'shortcut'
          ? current.sessionId
          : `legacy-${++browserSwitcherSessionCounterRef.current}`
      )
      void refreshBrowserSwitcher({ cycleIfOpen: true, direction, sessionId, mode: 'shortcut' })
    })
  }, [api, refreshBrowserSwitcher])

  useEffect(() => {
    if (!api.onBrowserSwitcherCommitRequested) return undefined
    return api.onBrowserSwitcherCommitRequested(commitBrowserSwitcher)
  }, [api, commitBrowserSwitcher])

  useEffect(() => {
    if (!api.onBrowserSwitcherCancelRequested) return undefined
    return api.onBrowserSwitcherCancelRequested(cancelBrowserSwitcher)
  }, [api, cancelBrowserSwitcher])

  useEffect(() => {
    const pendingSessionId = pendingBrowserSwitcherCommitSessionRef.current
    if (!pendingSessionId || browserSwitcher.loading) return
    pendingBrowserSwitcherCommitSessionRef.current = ''
    if (!browserSwitcher.open || browserSwitcher.mode !== 'shortcut' || browserSwitcher.activating || browserSwitcher.sessionId !== pendingSessionId) return
    void activateBrowserTarget(browserSwitcher.items[browserSwitcher.selectedIndex], pendingSessionId)
  }, [activateBrowserTarget, browserSwitcher])

  useEffect(() => {
    function handleWindowBlur() {
      const current = browserSwitcherRef.current
      if (!current.open || current.mode !== 'manual' || current.activating) return
      resetBrowserSwitcher()
    }
    window.addEventListener('blur', handleWindowBlur)
    return () => window.removeEventListener('blur', handleWindowBlur)
  }, [resetBrowserSwitcher])

  useEffect(() => {
    document.documentElement.dataset.theme = app.preferences.theme
    document.documentElement.style.colorScheme = app.preferences.theme
  }, [app.preferences.theme])

  useEffect(() => {
    if (route.type === 'server' && !app.selectedServerRecord) {
      setRoute({ type: 'root', tab: 'servers' })
    }
    if (route.type === 'tunnel' && !app.selectedConfiguration) {
      setRoute(app.selectedServerRecord ? { type: 'server' } : { type: 'root', tab: 'servers' })
    }
  }, [app.selectedConfiguration, app.selectedServerRecord, route.type])

  useEffect(() => {
    function handleEscape(event) {
      if (event.key !== 'Escape' || event.defaultPrevented || browserSwitcherRef.current.open || confirmation || app.unlockRequest) return
      if (form) {
        event.preventDefault()
        setForm(null)
      } else if (route.type === 'tunnel') {
        event.preventDefault()
        setRoute({ type: 'server' })
      } else if (route.type === 'server') {
        event.preventDefault()
        setRoute({ type: 'root', tab: 'servers' })
      } else {
        app.hideWindow?.()
      }
    }
    window.addEventListener('keydown', handleEscape)
    return () => window.removeEventListener('keydown', handleEscape)
  }, [app, confirmation, form, route.type])

  function openServer(serverId) {
    app.selectServer(serverId)
    setRoute({ type: 'server' })
  }

  function openTunnel(configurationId) {
    const record = app.selectConfiguration(configurationId)
    if (!record) return
    setRoute(isManagedSOCKSConfiguration(record.configuration) ? { type: 'server' } : { type: 'tunnel' })
  }

  function openNewServer() {
    setForm({ type: 'server', value: emptyServer(app.currentUsername) })
  }

  function openEditServer(server) {
    setForm({
      type: 'server',
      value: {
        ...server,
        port: String(server.port),
        socksPort: server.socksPort ? String(server.socksPort) : '',
      },
    })
  }

  function openNewTunnel() {
    if (!app.selectedServer) return
    setForm({ type: 'tunnel', value: emptyTunnel(app.selectedServer.id) })
  }

  function openEditTunnel(configuration) {
    setForm({ type: 'tunnel', value: { ...configuration } })
  }

  function closeForm(saved) {
    const type = form?.type
    setForm(null)
    if (!saved) return
    setRoute(type === 'server' ? { type: 'server' } : { type: 'tunnel' })
  }

  function requestDeleteServer(server) {
    setConfirmation({
      kind: 'server',
      id: server.id,
      title: `Delete ${server.name}?`,
      description: 'This removes the server and every saved tunnel under it. Running tunnels for this server should be stopped first.',
      confirmLabel: 'Delete server',
    })
  }

  function requestDeleteTunnel(configuration) {
    setConfirmation({
      kind: 'tunnel',
      id: configuration.id,
      title: `Delete ${configuration.label}?`,
      description: 'This removes the saved tunnel configuration and its connection history. This cannot be undone.',
      confirmLabel: 'Delete tunnel',
    })
  }

  async function confirmDeletion() {
    if (!confirmation) return
    const deleted = confirmation.kind === 'server'
      ? await app.deleteServer(confirmation.id)
      : await app.deleteTunnel(confirmation.id)
    if (!deleted) return
    setConfirmation(null)
    setRoute(confirmation.kind === 'server' ? { type: 'root', tab: 'servers' } : { type: 'server' })
  }

  const confirmationPending = useMemo(() => {
    if (!confirmation) return false
    return Boolean(app.pending[`${confirmation.kind === 'server' ? 'delete-server' : 'delete-tunnel'}:${confirmation.id}`])
  }, [app.pending, confirmation])

  if (browserSwitcher.open) {
    return (
      <div className="window-shell browser-switcher-mode">
        <div className="app-frame browser-switcher-frame">
          <BrowserSwitcher
            items={browserSwitcher.items}
            selectedIndex={browserSwitcher.selectedIndex}
            loading={browserSwitcher.loading}
            error={browserSwitcher.error}
            activating={browserSwitcher.activating}
            mode={browserSwitcher.mode}
            forwardShortcut={app.preferences.browserSwitcherShortcut}
            backwardShortcut={app.preferences.browserSwitcherBackwardShortcut}
            appearances={app.preferences.browserAppearances}
            onSelect={(selectedIndex) => updateBrowserSwitcher((state) => ({ ...state, selectedIndex }))}
            onActivate={activateBrowserTarget}
            onSaveAppearance={app.setBrowserAppearance}
            onRefresh={() => refreshBrowserSwitcher({
              sessionId: browserSwitcher.sessionId,
              mode: browserSwitcher.mode,
            })}
            onClose={closeBrowserSwitcher}
          />
        </div>
      </div>
    )
  }

  if (form?.type === 'server') {
    return (
      <div className="window-shell">
        <div className="app-frame">
          <ServerFormScreen
            initialValue={form.value}
            currentUsername={app.currentUsername}
            sshKeys={app.sshKeys}
            pending={Boolean(app.pending[`save-server:${form.value.id || 'new'}`])}
            onCancel={closeForm}
            onSave={app.saveServer}
          />
          <ToastRegion notification={app.notification} onDismiss={app.dismissNotification} />
        </div>
      </div>
    )
  }

  if (form?.type === 'tunnel' && app.selectedServer) {
    return (
      <div className="window-shell">
        <div className="app-frame">
          <TunnelFormScreen
            server={app.selectedServer}
            initialValue={form.value}
            pending={Boolean(app.pending[`save-tunnel:${form.value.id || 'new'}`])}
            onCancel={closeForm}
            onSave={app.saveTunnel}
          />
          <ToastRegion notification={app.notification} onDismiss={app.dismissNotification} />
        </div>
      </div>
    )
  }

  const rootTab = route.type === 'root' ? route.tab : null
  const header = route.type === 'server' && app.selectedServer
    ? { title: app.selectedServer.name, subtitle: `${app.selectedServer.username}@${app.selectedServer.host}` }
    : route.type === 'tunnel' && app.selectedConfiguration
      ? { title: app.selectedConfiguration.label, subtitle: app.selectedServer?.name || 'Tunnel details' }
      : rootCopy[rootTab] || rootCopy.servers

  const handleBack = route.type === 'tunnel'
    ? () => setRoute({ type: 'server' })
    : route.type === 'server'
      ? () => setRoute({ type: 'root', tab: 'servers' })
      : null

  return (
    <div className="window-shell">
      <div className="app-frame" aria-busy={app.phase === 'loading'}>
        <AppHeader title={header.title} subtitle={header.subtitle} onBack={handleBack} onHide={app.hideWindow} />

        <main className="app-content">
          {app.phase === 'loading' ? <LoadingScreen /> : null}

          {app.phase === 'error' ? (
            <div className="screen-scroll screen-scroll--centered">
              <EmptyState
                icon={CircleAlert}
                title="Saved data did not load"
                description={app.storageIssue || 'Check the database and try again.'}
                action={<button className="primary-button" type="button" onClick={() => app.hydrate()}>Try again</button>}
              />
            </div>
          ) : null}

          {app.phase === 'ready' && route.type === 'root' && route.tab === 'servers' ? (
            <ServersScreen
              servers={app.servers}
              sessions={app.runtimeSessions}
              pending={app.pending}
              onAdd={openNewServer}
              onOpen={openServer}
              onOpenBrowser={app.openServerBrowser}
              onOpenExplorer={app.openServerExplorer}
            />
          ) : null}

          {app.phase === 'ready' && route.type === 'root' && route.tab === 'activity' ? (
            <ActivityScreen
              servers={app.servers}
              sessions={app.liveSessions}
              pending={app.pending}
              runtimeFresh={app.runtimeFresh}
              onOpen={openTunnel}
              onStop={app.stopTunnel}
              onRefresh={app.refreshRuntimeSessions}
            />
          ) : null}

          {app.phase === 'ready' && route.type === 'root' && route.tab === 'settings' ? (
            <SettingsScreen
              preferences={app.preferences}
              diagnostics={app.diagnostics}
              storageIssue={app.storageIssue}
              runtimeFresh={app.runtimeFresh}
              onToggleTheme={app.toggleTheme}
              onSetBrowserSwitcherShortcut={app.setBrowserSwitcherShortcut}
              onSetBrowserSwitcherBackwardShortcut={app.setBrowserSwitcherBackwardShortcut}
              onOpenBrowserSwitcher={openBrowserSwitcher}
              onReload={app.hydrate}
              onRefreshRuntime={app.refreshRuntimeSessions}
              onCopyPath={app.copyPath}
              onOpenDevTools={app.openDevTools}
              onQuit={app.quitApplication}
            />
          ) : null}

          {app.phase === 'ready' && route.type === 'server' && app.selectedServerRecord ? (
            <ServerDetailScreen
              record={app.selectedServerRecord}
              sessions={app.runtimeSessions}
              pending={app.pending}
              runtimeFresh={app.runtimeFresh}
              onAddTunnel={openNewTunnel}
              onEditServer={openEditServer}
              onDeleteServer={requestDeleteServer}
              onOpenTunnel={openTunnel}
              onStartTunnel={app.startTunnel}
              onStopTunnel={app.stopTunnel}
              onStartAll={app.startAll}
              onRefreshRuntime={app.refreshRuntimeSessions}
            />
          ) : null}

          {app.phase === 'ready' && route.type === 'tunnel' && app.selectedConfiguration ? (
            <TunnelDetailScreen
              configuration={app.selectedConfiguration}
              session={app.selectedSession}
              history={app.selectedHistory}
              historyLoading={app.historyLoading}
              runtimeFresh={app.runtimeFresh}
              browserState={app.browserState}
              pending={app.pending}
              onStart={app.startTunnel}
              onStop={app.stopTunnel}
              onRetry={app.retryTunnel}
              onEdit={openEditTunnel}
              onDelete={requestDeleteTunnel}
              onCopyHistory={app.copyHistory}
              onRefreshBrowsers={app.refreshBrowsers}
              onSelectBrowser={app.selectBrowser}
              onLaunchBrowser={app.launchBrowser}
              onUnlock={app.openUnlock}
              onRefreshRuntime={app.refreshRuntimeSessions}
            />
          ) : null}
        </main>

        {route.type === 'root' && app.phase === 'ready' ? (
          <BottomNav
            current={route.tab}
            activeCount={app.liveSessions.length}
            onChange={(tab) => setRoute({ type: 'root', tab })}
            onOpenMoonPixels={() => app.openExternalURL('https://moonpixels.tech')}
          />
        ) : null}

        <ToastRegion notification={app.notification} onDismiss={app.dismissNotification} />
        <ConfirmDialog request={confirmation} pending={confirmationPending} onClose={() => setConfirmation(null)} onConfirm={confirmDeletion} />
        <UnlockDialog request={app.unlockRequest} pending={Boolean(app.pending.unlock)} onClose={app.closeUnlock} onSubmit={app.submitUnlock} />
      </div>
    </div>
  )
}
