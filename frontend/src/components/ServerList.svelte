<script>
  export let servers = []
  export let selectedServerId = ''
  export let onSelect = () => {}
  export let onCreate = () => {}
  export let onEdit = () => {}
  export let onDelete = () => {}
</script>

<section class="p-card panel server-panel panel--compact" aria-labelledby="server-list-heading">
  <div class="p-card__header panel-header">
    <div>
      <p class="eyebrow">Saved servers</p>
      <h2 id="server-list-heading">Targets</h2>
      <p class="panel-copy">Choose the host you want to work in.</p>
    </div>
    <button class="p-button is-compact-button" type="button" on:click={onCreate}>Add</button>
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
          <article class="p-card list-item-shell list-item-shell--server" class:selected={selectedServerId === item.server.id} class:is-selected={selectedServerId === item.server.id}>
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

                <div class="list-card-actions">
                  <button
                    class="list-action"
                    type="button"
                    aria-label={`Edit ${item.server.name}`}
                    on:click={() => onEdit(item.server)}
                  >Edit</button>
                  <button
                    class="list-action list-action--danger"
                    type="button"
                    aria-label={`Delete ${item.server.name}`}
                    on:click={() => onDelete(item.server.id)}
                  >Delete</button>
                </div>
              </div>
            </div>
          </article>
        </li>
      {/each}
    </ul>
  {/if}
</section>
