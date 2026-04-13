<script>
  export let servers = []
  export let selectedServerId = ''
  export let onSelect = () => {}
  export let onCreate = () => {}
  export let onEdit = () => {}
  export let onDelete = () => {}
</script>

<section class="p-card panel" aria-label="Servers">
  <div class="panel-header">
    <h2 class="p-card__title">Hosts</h2>

    <div class="panel-actions panel-actions--inline">
      <button class="p-button--positive" type="button" on:click={onCreate}>Add Host</button>
    </div>
  </div>

  {#if servers.length === 0}
    <div class="empty-state compact">
      <h3 class="p-card__title">No servers yet</h3>
      <p>Create a server to start saving SSH tunnels.</p>
    </div>
  {:else}
    <div class="stack-list">
      {#each servers as item}
        <div class={selectedServerId === item.server.id ? 'p-card--highlighted' : 'p-card'}>
          <button
            class="u-no-margin u-no-padding"
            style="background: none; border: none; width: 100%; text-align: left; cursor: pointer; display: block;"
            type="button"
            aria-pressed={selectedServerId === item.server.id}
            aria-label={`Select server ${item.server.name}`}
            on:click={() => onSelect(item.server.id)}
          >
            <strong class="p-card__title">{item.server.name}</strong>
            <p class="p-card__content u-text--muted">{item.server.username}@{item.server.host}:{item.server.port}</p>
          </button>
          <hr />
          <div style="display: flex; align-items: center; justify-content: space-between;">
            <span class="p-chip is-inline">
              <span class="p-chip__lead">Tunnels</span>
              <span class="p-chip__value">{item.configurations.length}</span>
            </span>
            <div>
              <button
                class="p-button--base is-dense"
                type="button"
                aria-label={`Edit ${item.server.name}`}
                on:click={() => onEdit(item.server)}
              >Edit</button>
              <button
                class="p-button--negative is-dense"
                type="button"
                aria-label={`Delete ${item.server.name}`}
                on:click={() => onDelete(item.server.id)}
              >Delete</button>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>
