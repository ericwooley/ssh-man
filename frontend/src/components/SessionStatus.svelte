<script>
  export let configuration = null
  export let session = null
  export let history = []
  export let onStart = () => {}
  export let onStop = () => {}
  export let onCopyHistory = () => {}

  const activeStatuses = ['starting', 'connected', 'reconnecting']
  const startableStatuses = ['stopped', 'failed']
  const collapsedHistoryCount = 5

  let historyExpanded = false
  let historyOwnerId = ''

  function statusLabel(status) {
    switch (status) {
      case 'starting':
        return 'Starting'
      case 'connected':
        return 'Connected'
      case 'reconnecting':
        return 'Reconnecting'
      case 'needs_attention':
        return 'Needs attention'
      case 'failed':
        return 'Failed'
      default:
        return 'Stopped'
    }
  }

  function statusClass(status) {
    switch (status) {
      case 'connected':
        return 'status-running'
      case 'reconnecting':
        return 'status-reconnecting'
      case 'failed':
        return 'status-failed'
      case 'needs_attention':
        return 'status-attention'
      case 'starting':
        return 'status-info'
      default:
        return 'status-stopped'
    }
  }

  function chipClass(status) {
    switch (status) {
      case 'connected':
        return 'p-chip--positive'
      case 'reconnecting':
      case 'needs_attention':
        return 'p-chip--caution'
      case 'failed':
        return 'p-chip--negative'
      case 'starting':
        return 'p-chip--information'
      default:
        return ''
    }
  }

  function calloutClass(status) {
    switch (status) {
      case 'connected':
        return 'p-notification--positive'
      case 'reconnecting':
      case 'needs_attention':
        return 'p-notification--caution'
      case 'failed':
        return 'p-notification--negative'
      default:
        return ''
    }
  }

  $: currentStatus = session?.status || 'stopped'
  $: canStart = !configuration ? false : startableStatuses.includes(currentStatus) || !session
  $: canStop = Boolean(configuration) && (activeStatuses.includes(currentStatus) || currentStatus === 'needs_attention')
  $: if ((configuration?.id || '') !== historyOwnerId) {
    historyOwnerId = configuration?.id || ''
    historyExpanded = false
  }
  $: hiddenHistoryCount = Math.max(history.length - collapsedHistoryCount, 0)
  $: visibleHistory = historyExpanded ? history : history.slice(0, collapsedHistoryCount)
</script>

<section class="p-card panel status-panel panel--compact" aria-labelledby="session-status-heading">
  <div class="p-card__header panel-header">
    <div>
      <p class="eyebrow">Runtime</p>
      <h2 id="session-status-heading">Session status</h2>
      <p class="panel-copy">Operate the focused tunnel and watch its live state.</p>
    </div>
    <span class={`p-chip status-pill is-inline ${chipClass(currentStatus)} ${currentStatus} ${statusClass(currentStatus)}`} aria-live="polite" aria-label={`Session status ${currentStatus}`}>{statusLabel(currentStatus)}</span>
  </div>

  {#if configuration}
    <div class="p-card runtime-summary runtime-summary--compact">
      <div>
        <span class="runtime-label">Selected tunnel</span>
        <strong>{configuration.label}</strong>
      </div>
        <small>
          {#if configuration.connectionType === 'socks_proxy'}
          SOCKS proxy on {session?.boundPort ? `:${session.boundPort}` : configuration.socksPort > 0 ? `:${configuration.socksPort}` : 'Auto'}
        {:else}
          Local {configuration.localPort} -> {configuration.remoteHost}:{configuration.remotePort}
        {/if}
      </small>
    </div>

      <div class={`p-notification status-callout ${calloutClass(currentStatus)} is-borderless ${statusClass(currentStatus)}`} role="status">
        <div class="p-notification__content">
          <p class="p-notification__message">{session?.statusDetail || 'This saved tunnel is idle.'}</p>
        </div>
    </div>

    <div class="runtime-actions-grid">
      <button class="p-button is-compact-button" type="button" disabled={!canStart} on:click={() => onStart(configuration.id)}>Start tunnel</button>
      <button class="p-button--negative is-compact-button" type="button" disabled={!canStop} on:click={() => onStop(configuration.id)}>Stop tunnel</button>
    </div>

    <div class="history-panel" aria-labelledby="session-history-heading">
      <div class="section-heading">
        <div>
          <h3 id="session-history-heading">Recent connection history</h3>
          <p>User-visible outcomes for this tunnel.</p>
        </div>
        <button class="p-button--base is-compact-button" type="button" disabled={history.length === 0} on:click={() => onCopyHistory(configuration.id)}>Copy history</button>
      </div>

      {#if history.length > 0}
        <ul class="history-list" aria-label="Recent connection history entries">
          {#each visibleHistory as entry (entry.id)}
            <li class="p-card history-item">
              <div class="history-item-topline">
                <strong>{entry.outcome.replaceAll('_', ' ')}</strong>
                <span class="history-timestamp">{new Date(entry.endedAt || entry.startedAt).toLocaleString()}</span>
              </div>
              <p>{entry.message}</p>
            </li>
          {/each}
        </ul>

        {#if hiddenHistoryCount > 0}
          <div class="history-toggle-row">
            <button
              class="p-button--base"
              type="button"
              aria-expanded={historyExpanded}
              on:click={() => (historyExpanded = !historyExpanded)}
            >
              {#if historyExpanded}
                Show fewer entries
              {:else}
                Show {hiddenHistoryCount} older entr{hiddenHistoryCount === 1 ? 'y' : 'ies'}
              {/if}
            </button>
          </div>
        {/if}
      {:else}
        <div class="empty-state compact">
          <h3>No recorded connection events</h3>
          <p>Start or stop this tunnel to build a shareable troubleshooting trail.</p>
        </div>
      {/if}
    </div>
  {:else}
    <div class="empty-state compact">
      <h3>No tunnel selected</h3>
      <p>Select a saved tunnel to start or stop its session.</p>
    </div>
  {/if}
</section>
