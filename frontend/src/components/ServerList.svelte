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
</script>

<section class="panel server-panel" aria-labelledby="server-list-heading">
  <div class="panel-header">
    <div>
      <p class="eyebrow">Saved servers</p>
      <h2 id="server-list-heading">Targets</h2>
      <p class="panel-copy">Choose where new tunnels should be saved and which host you want to work in.</p>
    </div>
    <button class="button button-primary" type="button" on:click={onCreate}>Add server</button>
  </div>

  {#if servers.length === 0}
    <div class="empty-state">
      <span class="empty-mark" aria-hidden="true">SSH</span>
      <h3>No servers yet</h3>
      <p>Create a server to start saving SSH tunnels.</p>
    </div>
  {:else}
    <ul class="stack-list" aria-label="Saved servers">
      {#each servers as item}
        <li>
          <div class:selected={selectedServerId === item.server.id} class="list-item-shell">
            <div class="list-card-topline">
            <button
              class="list-card-main"
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
                <span class="pill">{item.configurations.length}</span>

                <div class:open={openMenuId === item.server.id} class="row-menu">
                  <button
                    class="button button-ghost button-small row-menu-trigger"
                    type="button"
                    aria-label={`More actions for ${item.server.name}`}
                    aria-expanded={openMenuId === item.server.id}
                    aria-haspopup="menu"
                    on:click={() => toggleMenu(item.server.id)}
                  >...</button>
                  {#if openMenuId === item.server.id}
                    <div class="row-menu-popover" role="menu" aria-label={`Actions for ${item.server.name}`}>
                      <button class="button button-ghost button-small" type="button" aria-label={`Edit ${item.server.name}`} on:click={() => { closeMenu(); onEdit(item.server) }}>Edit</button>
                      <button class="button button-ghost button-small danger" type="button" aria-label={`Delete ${item.server.name}`} on:click={() => { closeMenu(); onDelete(item.server.id) }}>Delete</button>
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
