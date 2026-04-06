<script>
  export let configurations = []
  export let selectedConfigurationId = ''
  export let sessions = []
  export let onSelect = () => {}
  export let onCreate = () => {}
  export let onEdit = () => {}
  export let onDelete = () => {}

  function sessionFor(id) {
    return sessions.find((item) => item.configurationId === id)
  }
</script>

<section class="panel config-panel" aria-labelledby="config-list-heading">
  <div class="panel-header">
    <div>
      <p class="eyebrow">Configurations</p>
      <h2 id="config-list-heading">Tunnels</h2>
    </div>
    <button class="button button-primary" type="button" on:click={onCreate}>Add tunnel</button>
  </div>

  {#if configurations.length === 0}
    <div class="empty-state compact">
      <h3>No saved tunnels</h3>
      <p>Store a local forward or SOCKS proxy under the selected server.</p>
    </div>
  {:else}
    <ul class="stack-list" aria-label="Saved configurations">
      {#each configurations as configuration}
        {@const runtime = sessionFor(configuration.id)}
        <li>
          <button
            class:selected={selectedConfigurationId === configuration.id}
            class="list-row"
            type="button"
            aria-pressed={selectedConfigurationId === configuration.id}
            aria-label={`Select tunnel ${configuration.label}`}
            on:click={() => onSelect(configuration.id)}
          >
            <span>
              <strong>{configuration.label}</strong>
              <small>
                {#if configuration.connectionType === 'socks_proxy'}
                  SOCKS :{configuration.socksPort}
                {:else}
                  {configuration.localPort} -> {configuration.remoteHost}:{configuration.remotePort}
                {/if}
              </small>
            </span>
             <span class={`status-pill ${runtime?.status || 'stopped'}`} aria-label={`Tunnel status ${runtime?.status || 'stopped'}`}>{runtime?.status || 'stopped'}</span>
          </button>
          <div class="row-actions">
            <button class="button button-ghost" type="button" aria-label={`Edit ${configuration.label}`} on:click={() => onEdit(configuration)}>Edit</button>
            <button class="button button-ghost danger" type="button" aria-label={`Delete ${configuration.label}`} on:click={() => onDelete(configuration.id)}>Delete</button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</section>
