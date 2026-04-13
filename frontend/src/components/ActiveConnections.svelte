<script>
  export let connections = []
  export let onSelect = () => {}
  export let onStop = () => {}

  function statusChipClass(status) {
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

  function statusLabel(status) {
    return (status || 'stopped').replaceAll('_', ' ')
  }
</script>

<section class="p-card panel active-connections-panel panel--compact" aria-labelledby="active-connections-heading">
  <div class="p-card__header panel-header">
    <div>
      <p class="eyebrow">Connections</p>
      <h2 id="active-connections-heading">Active tunnels</h2>
      <p class="panel-copy">Live, reconnecting, and unlock-pending sessions.</p>
    </div>
    <span class="p-chip is-inline">
      <span class="p-chip__lead">Live</span>
      <span class="p-chip__value">{connections.length}</span>
    </span>
  </div>

  {#if connections.length === 0}
    <div class="empty-state compact">
      <h3>No active tunnels</h3>
      <p>Connected, reconnecting, and unlock-pending tunnels will appear here.</p>
    </div>
  {:else}
    <ul class="stack-list" aria-label="Active tunnels">
      {#each connections as connection}
        <li>
          <article class="p-card list-item-shell list-item-shell--runtime">
            <div class="list-card-topline">
              <button class="p-button--base list-card-main" type="button" aria-label={`Show ${connection.configurationLabel}`} on:click={() => onSelect(connection.configurationId)}>
                <span class="list-primary">
                  <strong>{connection.configurationLabel}</strong>
                  <small>{connection.serverName}</small>
                  <small>{connection.statusDetail || connection.status}</small>
                </span>
              </button>

              <div class="list-card-tools">
                <span class={`p-chip status-pill is-inline ${statusChipClass(connection.status)} ${connection.status} ${connection.status === 'connected' ? 'status-running' : connection.status === 'reconnecting' ? 'status-reconnecting' : connection.status === 'failed' ? 'status-failed' : connection.status === 'needs_attention' ? 'status-attention' : connection.status === 'starting' ? 'status-info' : 'status-stopped'}`}>{statusLabel(connection.status)}</span>

                <div class="list-card-actions">
                  <button
                    class="list-action list-action--danger"
                    type="button"
                    aria-label={`Disconnect ${connection.configurationLabel}`}
                    on:click={() => onStop(connection.configurationId)}
                  >Disconnect</button>
                </div>
              </div>
            </div>
          </article>
        </li>
      {/each}
    </ul>
  {/if}
</section>
