<script>
  export let connections = []
  export let onSelect = () => {}
  export let onStop = () => {}

  let openMenuId = ''

  function toggleMenu(configurationId) {
    openMenuId = openMenuId === configurationId ? '' : configurationId
  }

  function closeMenu() {
    openMenuId = ''
  }

  function handleWindowClick(event) {
    if (event.target?.closest?.('.row-menu')) return
    closeMenu()
  }

  function handleWindowKeydown(event) {
    if (event.key === 'Escape') {
      closeMenu()
    }
  }

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

<svelte:window on:click={handleWindowClick} on:keydown={handleWindowKeydown} />

<section class="p-card panel active-connections-panel" aria-labelledby="active-connections-heading">
  <div class="p-card__header panel-header">
    <div>
      <p class="eyebrow">Connections</p>
      <h2 id="active-connections-heading">Active tunnels</h2>
      <p class="panel-copy">Sessions that are live, reconnecting, or waiting for a passphrase stay visible here.</p>
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
          <article class="p-card list-item-shell" class:menu-open={openMenuId === connection.configurationId}>
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

                <div class="p-contextual-menu row-menu" class:open={openMenuId === connection.configurationId}>
                  <button
                    class="p-button--base p-contextual-menu__toggle row-menu-trigger"
                    type="button"
                    aria-label={`More actions for ${connection.configurationLabel}`}
                    aria-expanded={openMenuId === connection.configurationId}
                    aria-haspopup="menu"
                    on:click={() => toggleMenu(connection.configurationId)}
                  >More</button>
                  <div class="p-contextual-menu__dropdown row-menu-popover" role="menu" aria-label={`Actions for ${connection.configurationLabel}`} aria-hidden={openMenuId === connection.configurationId ? 'false' : 'true'}>
                    <span class="p-contextual-menu__group">
                      <button class="p-contextual-menu__link danger" type="button" aria-label={`Disconnect ${connection.configurationLabel}`} on:click={() => { closeMenu(); onStop(connection.configurationId) }}>Disconnect</button>
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </article>
        </li>
      {/each}
    </ul>
  {/if}
</section>
