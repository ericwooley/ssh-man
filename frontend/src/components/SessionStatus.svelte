<script>
  export let configuration = null
  export let session = null
  export let history = []
  export let onStart = () => {}
  export let onStop = () => {}
  export let onCopyHistory = () => {}

  const activeStatuses = ['starting', 'connected', 'reconnecting']
  const startableStatuses = ['stopped', 'failed']

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

  $: currentStatus = session?.status || 'stopped'
  $: canStart = !configuration ? false : startableStatuses.includes(currentStatus) || !session
  $: canStop = Boolean(configuration) && (activeStatuses.includes(currentStatus) || currentStatus === 'needs_attention')
</script>

<section class="panel status-panel" aria-labelledby="session-status-heading">
  <div class="panel-header">
    <div>
      <p class="eyebrow">Runtime</p>
      <h2 id="session-status-heading">Session status</h2>
      <p class="panel-copy">Operate the selected tunnel here and watch its current runtime state.</p>
    </div>
    <span class={`status-pill ${currentStatus}`} aria-live="polite" aria-label={`Session status ${currentStatus}`}>{statusLabel(currentStatus)}</span>
  </div>

  {#if configuration}
    <div class="runtime-summary">
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

    <p class="status-copy status-callout" role="status">{session?.statusDetail || 'This saved tunnel is idle.'}</p>

    <div class="runtime-actions-grid">
      <button class="button button-primary" type="button" disabled={!canStart} on:click={() => onStart(configuration.id)}>Start tunnel</button>
      <button class="button button-ghost danger" type="button" disabled={!canStop} on:click={() => onStop(configuration.id)}>Stop tunnel</button>
    </div>

    <div class="history-panel" aria-labelledby="session-history-heading">
      <div class="section-heading">
        <div>
          <h3 id="session-history-heading">Recent connection history</h3>
          <p>User-visible outcomes for this saved tunnel.</p>
        </div>
        <button class="button button-ghost button-small" type="button" disabled={history.length === 0} on:click={() => onCopyHistory(configuration.id)}>Copy history</button>
      </div>

      {#if history.length > 0}
        <ul class="history-list" aria-label="Recent connection history entries">
          {#each history as entry (entry.id)}
            <li class="history-item">
              <div class="history-item-topline">
                <strong>{entry.outcome.replaceAll('_', ' ')}</strong>
                <span class="history-timestamp">{new Date(entry.endedAt || entry.startedAt).toLocaleString()}</span>
              </div>
              <p>{entry.message}</p>
            </li>
          {/each}
        </ul>
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
