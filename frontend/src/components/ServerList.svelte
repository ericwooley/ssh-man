<script>
  export let servers = []
  export let selectedServerId = ''
  export let onSelect = () => {}
  export let onCreate = () => {}
  export let onEdit = () => {}
  export let onDelete = () => {}

  let openMenuId = ''

  function toggleMenu(serverId) {
    openMenuId = openMenuId === serverId ? '' : serverId
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
</script>

<svelte:window on:click={handleWindowClick} on:keydown={handleWindowKeydown} />

<section class="p-card panel server-panel" aria-labelledby="server-list-heading">
  <div class="p-card__header panel-header">
    <div>
      <p class="eyebrow">Saved servers</p>
      <h2 id="server-list-heading">Targets</h2>
      <p class="panel-copy">Choose where new tunnels should be saved and which host you want to work in.</p>
    </div>
    <button class="p-button" type="button" on:click={onCreate}>Add server</button>
  </div>

  {#if servers.length === 0}
    <div class="empty-state">
      <h3>No servers yet</h3>
      <p>Create a server to start saving SSH tunnels.</p>
    </div>
  {:else}
    <ul class="stack-list" aria-label="Saved servers">
      {#each servers as item}
        <li>
          <article class="p-card list-item-shell" class:selected={selectedServerId === item.server.id} class:is-selected={selectedServerId === item.server.id} class:menu-open={openMenuId === item.server.id}>
            <div class="list-card-topline">
              <button
                class="p-button--base list-card-main"
                type="button"
                aria-pressed={selectedServerId === item.server.id}
                aria-label={`Select server ${item.server.name}`}
                on:click={() => onSelect(item.server.id)}
              >
                <span class="list-primary">
                  <strong>{item.server.name}</strong>
                  <small>{item.server.username}@{item.server.host}:{item.server.port}</small>
                </span>
              </button>

              <div class="list-card-tools">
                <span class="p-chip is-inline">
                  <span class="p-chip__lead">Tunnels</span>
                  <span class="p-chip__value">{item.configurations.length}</span>
                </span>

                <div class="p-contextual-menu row-menu" class:open={openMenuId === item.server.id}>
                  <button
                    class="p-button--base p-contextual-menu__toggle row-menu-trigger"
                    type="button"
                    aria-label={`More actions for ${item.server.name}`}
                    aria-expanded={openMenuId === item.server.id}
                    aria-haspopup="menu"
                    on:click={() => toggleMenu(item.server.id)}
                  >More</button>
                  <div class="p-contextual-menu__dropdown row-menu-popover" role="menu" aria-label={`Actions for ${item.server.name}`} aria-hidden={openMenuId === item.server.id ? 'false' : 'true'}>
                    <span class="p-contextual-menu__group">
                      <button class="p-contextual-menu__link" type="button" aria-label={`Edit ${item.server.name}`} on:click={() => { closeMenu(); onEdit(item.server) }}>Edit</button>
                      <button class="p-contextual-menu__link danger" type="button" aria-label={`Delete ${item.server.name}`} on:click={() => { closeMenu(); onDelete(item.server.id) }}>Delete</button>
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
