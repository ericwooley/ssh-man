<script>
  export let configurations = []
  export let selectedConfigurationId = ''
  export let sessions = []
  export let enabled = true
  export let onSelect = () => {}
  export let onCreate = () => {}
  export let onStartAll = () => {}
  export let onEdit = () => {}
  export let onDelete = () => {}

  let openMenuId = ''
  let sessionByConfigurationId = new Map()

  $: sessionByConfigurationId = new Map(sessions.map((session) => [session.configurationId, session]))

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

<section class:is-disabled={!enabled} class="p-card panel config-panel panel--compact" aria-labelledby="config-list-heading">
  <div class="p-card__header panel-header">
    <div>
      <p class="eyebrow">Configurations</p>
      <h2 id="config-list-heading">Tunnels</h2>
      <p class="panel-copy">Saved forwards and SOCKS proxies for the focused target.</p>
    </div>
    <div class="panel-actions">
      <button class="p-button--base is-compact-button" disabled={!enabled || configurations.length === 0} type="button" on:click={onStartAll}>Start all</button>
      <button class="p-button is-compact-button" disabled={!enabled} type="button" on:click={onCreate}>Add tunnel</button>
    </div>
  </div>

  {#if !enabled}
    <div class="empty-state compact">
      <h3>Select a target first</h3>
      <p>Choose a saved server in the targets lane before viewing or editing tunnels.</p>
    </div>
  {:else if configurations.length === 0}
    <div class="empty-state compact">
      <h3>No saved tunnels</h3>
      <p>Store a local forward or SOCKS proxy under the selected server.</p>
    </div>
  {:else}
    <ul class="stack-list" aria-label="Saved configurations">
      {#each configurations as configuration}
        {@const runtime = sessionByConfigurationId.get(configuration.id)}
        <li>
          <article class="p-card list-item-shell list-item-shell--config" class:selected={selectedConfigurationId === configuration.id} class:is-selected={selectedConfigurationId === configuration.id} class:menu-open={openMenuId === configuration.id}>
            <div class="list-card-topline">
              <button
                class="p-button--base list-card-main"
                type="button"
                aria-pressed={selectedConfigurationId === configuration.id}
                aria-label={`Select tunnel ${configuration.label}`}
                on:click={() => onSelect(configuration.id)}
              >
                <span class="list-primary">
                  <strong>{configuration.label}</strong>
                  <small>
                    {#if configuration.connectionType === 'socks_proxy'}
                      SOCKS {configuration.socksPort > 0 ? `:${configuration.socksPort}` : 'Auto'}
                    {:else}
                      {configuration.localPort} -> {configuration.remoteHost}:{configuration.remotePort}
                    {/if}
                  </small>
                  {#if runtime?.statusDetail}
                    <small>{runtime.statusDetail}</small>
                  {/if}
                </span>
              </button>

              <div class="list-card-tools">
                <span class={`p-chip status-pill is-inline ${statusChipClass(runtime?.status)} ${runtime?.status || 'stopped'} ${runtime?.status === 'connected' ? 'status-running' : runtime?.status === 'reconnecting' ? 'status-reconnecting' : runtime?.status === 'failed' ? 'status-failed' : runtime?.status === 'needs_attention' ? 'status-attention' : runtime?.status === 'starting' ? 'status-info' : 'status-stopped'}`} aria-label={`Tunnel status ${runtime?.status || 'stopped'}`}>{statusLabel(runtime?.status)}</span>

                <div class="p-contextual-menu row-menu" class:open={openMenuId === configuration.id}>
                  <button
                    class="p-button--base p-contextual-menu__toggle row-menu-trigger"
                    type="button"
                    aria-label={`More actions for ${configuration.label}`}
                    aria-expanded={openMenuId === configuration.id}
                    aria-haspopup="menu"
                    on:click={() => toggleMenu(configuration.id)}
                  >More</button>
                  <div class="p-contextual-menu__dropdown row-menu-popover" role="menu" aria-label={`Actions for ${configuration.label}`} aria-hidden={openMenuId === configuration.id ? 'false' : 'true'}>
                    <span class="p-contextual-menu__group">
                      <button class="p-contextual-menu__link" type="button" aria-label={`Edit ${configuration.label}`} on:click={() => { closeMenu(); onEdit(configuration) }}>Edit</button>
                      <button class="p-contextual-menu__link danger" type="button" aria-label={`Delete ${configuration.label}`} on:click={() => { closeMenu(); onDelete(configuration.id) }}>Delete</button>
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
