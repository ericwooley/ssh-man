<script>
  export let connections = []
  export let onSelect = () => {}
  export let onStop = () => {}
</script>

<section class="panel active-connections-panel" aria-labelledby="active-connections-heading">
  <div class="panel-header">
    <div>
      <p class="eyebrow">Connections</p>
      <h2 id="active-connections-heading">Active tunnels</h2>
      <p class="panel-copy">Sessions that are live, reconnecting, or waiting for a passphrase stay visible here.</p>
    </div>
    <span class="pill">{connections.length}</span>
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
          <button class="list-row" type="button" aria-label={`Show ${connection.configurationLabel}`} on:click={() => onSelect(connection.configurationId)}>
            <span>
              <strong>{connection.configurationLabel}</strong>
              <small>{connection.serverName}</small>
              <small>{connection.statusDetail || connection.status}</small>
            </span>
            <span class={`status-pill ${connection.status}`}>{connection.status}</span>
          </button>
          <div class="row-actions">
            <button class="button button-ghost danger" type="button" aria-label={`Disconnect ${connection.configurationLabel}`} on:click={() => onStop(connection.configurationId)}>Disconnect</button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</section>
