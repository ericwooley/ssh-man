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

  function sessionFor(id) {
    return sessions.findLast ? sessions.findLast((item) => item.configurationId === id) : [...sessions].reverse().find((item) => item.configurationId === id)
  }

  function toggleMenu(configurationId) {
    openMenuId = openMenuId === configurationId ? '' : configurationId
  }

  function closeMenu() {
    openMenuId = ''
  }

  function handleWindowClick() {
    closeMenu()
  }

  function handleWindowKeydown(event) {
    if (event.key === 'Escape') {
      closeMenu()
    }
  }
</script>

<svelte:window on:click={handleWindowClick} on:keydown={handleWindowKeydown} />

<section class:is-disabled={!enabled} class="panel config-panel" aria-labelledby="config-list-heading">
  <div class="panel-header">
    <div>
      <p class="eyebrow">Configurations</p>
      <h2 id="config-list-heading">Tunnels</h2>
      <p class="panel-copy">Keep SOCKS proxies and forwards attached to the selected server so they are easy to restart.</p>
    </div>
    <div class="panel-actions">
      <button class="button button-ghost" disabled={!enabled || configurations.length === 0} type="button" on:click={onStartAll}>Start all</button>
      <button class="button button-primary" disabled={!enabled} type="button" on:click={onCreate}>Add tunnel</button>
    </div>
  </div>

  {#if !enabled}
    <div class="empty-state compact">
      <span class="empty-mark" aria-hidden="true">01</span>
      <h3>Select a target first</h3>
      <p>Choose a saved server in the targets lane before viewing or editing tunnels.</p>
    </div>
  {:else if configurations.length === 0}
    <div class="empty-state compact">
      <span class="empty-mark" aria-hidden="true">::</span>
      <h3>No saved tunnels</h3>
      <p>Store a local forward or SOCKS proxy under the selected server.</p>
    </div>
  {:else}
    <ul class="stack-list" aria-label="Saved configurations">
      {#each configurations as configuration}
        {@const runtime = sessionFor(configuration.id)}
        <li>
          <div class:selected={selectedConfigurationId === configuration.id} class:menu-open={openMenuId === configuration.id} class="list-item-shell">
            <div class="list-card-topline">
            <button
              class="list-card-main"
              type="button"
              aria-pressed={selectedConfigurationId === configuration.id}
              aria-label={`Select tunnel ${configuration.label}`}
              on:click={() => onSelect(configuration.id)}
            >
              <span class="list-primary">
                <strong>{configuration.label}</strong>
                <small>
                  {#if configuration.connectionType === 'socks_proxy'}
                    SOCKS :{configuration.socksPort}
                  {:else}
                    {configuration.localPort} -> {configuration.remoteHost}:{configuration.remotePort}
                  {/if}
                </small>
              </span>
            </button>

              <div class="list-card-tools">
                <span class={`status-pill ${runtime?.status || 'stopped'}`} aria-label={`Tunnel status ${runtime?.status || 'stopped'}`}>{runtime?.status || 'stopped'}</span>

                <div class:open={openMenuId === configuration.id} class="row-menu">
                  <button
                    class="button button-ghost button-small row-menu-trigger"
                    type="button"
                    aria-label={`More actions for ${configuration.label}`}
                    aria-expanded={openMenuId === configuration.id}
                    aria-haspopup="menu"
                    on:click={() => toggleMenu(configuration.id)}
                  >...</button>
                  {#if openMenuId === configuration.id}
                    <div class="row-menu-popover" role="menu" aria-label={`Actions for ${configuration.label}`}>
                      <button class="button button-ghost button-small" type="button" aria-label={`Edit ${configuration.label}`} on:click={() => { closeMenu(); onEdit(configuration) }}>Edit</button>
                      <button class="button button-ghost button-small danger" type="button" aria-label={`Delete ${configuration.label}`} on:click={() => { closeMenu(); onDelete(configuration.id) }}>Delete</button>
                    </div>
                  {/if}
                </div>
              </div>
            </div>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</section>
