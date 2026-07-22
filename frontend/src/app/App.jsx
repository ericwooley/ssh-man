import { useEffect, useMemo, useState } from 'react'
import { CircleAlert, Server } from 'lucide-react'
import {
  AppHeader,
  BottomNav,
  EmptyState,
  LoadingScreen,
  ToastRegion,
} from '../components/AppChrome'
import { ConfirmDialog, UnlockDialog } from '../components/Dialogs'
import { ActivityScreen, SettingsScreen } from '../screens/ActivitySettingsScreens'
import { ServerFormScreen, TunnelFormScreen } from '../screens/FormScreens'
import { ServerDetailScreen, ServersScreen } from '../screens/ServersScreens'
import { TunnelDetailScreen } from '../screens/TunnelScreen'
import { emptyServer, emptyTunnel } from '../model/appModel'
import { useSshMan } from './useSshMan'

const rootCopy = {
  servers: { title: 'SSH Man', subtitle: 'Servers and tunnels' },
  activity: { title: 'Active tunnels', subtitle: 'Live connections' },
  settings: { title: 'Settings', subtitle: 'Appearance and app health' },
}

export default function App({ api, controllerOptions }) {
  const app = useSshMan(api, controllerOptions)
  const [route, setRoute] = useState({ type: 'root', tab: 'servers' })
  const [form, setForm] = useState(null)
  const [confirmation, setConfirmation] = useState(null)

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
      if (event.key !== 'Escape' || confirmation || app.unlockRequest) return
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
    if (record) setRoute({ type: 'tunnel' })
  }

  function openNewServer() {
    setForm({ type: 'server', value: emptyServer(app.currentUsername) })
  }

  function openEditServer(server) {
    setForm({ type: 'server', value: { ...server, port: String(server.port) } })
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
            <ServersScreen servers={app.servers} sessions={app.runtimeSessions} onAdd={openNewServer} onOpen={openServer} />
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
              onReload={app.hydrate}
              onRefreshRuntime={app.refreshRuntimeSessions}
              onCopyPath={app.copyPath}
              onOpenDevTools={app.openDevTools}
              onOpenExternalURL={app.openExternalURL}
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
              onExplore={app.openServerExplorer}
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
          />
        ) : null}

        <ToastRegion notification={app.notification} onDismiss={app.dismissNotification} />
        <ConfirmDialog request={confirmation} pending={confirmationPending} onClose={() => setConfirmation(null)} onConfirm={confirmDeletion} />
        <UnlockDialog request={app.unlockRequest} pending={Boolean(app.pending.unlock)} onClose={app.closeUnlock} onSubmit={app.submitUnlock} />
      </div>
    </div>
  )
}
