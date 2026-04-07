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
      <span class="empty-mark" aria-hidden="true">ON</span>
      <h3>No active tunnels</h3>
      <p>Connected, reconnecting, and unlock-pending tunnels will appear here.</p>
    </div>
  {:else}
    <ul class="stack-list" aria-label="Active tunnels">
      {#each connections as connection}
        <li>
          <div class="list-item-shell">
            <div class="list-card-topline">
            <button class="list-card-main" type="button" aria-label={`Show ${connection.configurationLabel}`} on:click={() => onSelect(connection.configurationId)}>
              <span class="list-primary">
                <strong>{connection.configurationLabel}</strong>
                <small>{connection.serverName}</small>
                <small>{connection.statusDetail || connection.status}</small>
              </span>
            </button>

              <div class="list-card-tools">
                <span class={`status-pill ${connection.status}`}>{connection.status}</span>

                <div class:open={openMenuId === connection.configurationId} class="row-menu">
                  <button
                    class="button button-ghost button-small row-menu-trigger"
                    type="button"
                    aria-label={`More actions for ${connection.configurationLabel}`}
                    aria-expanded={openMenuId === connection.configurationId}
                    aria-haspopup="menu"
                    on:click={() => toggleMenu(connection.configurationId)}
                  >...</button>
                  {#if openMenuId === connection.configurationId}
                    <div class="row-menu-popover" role="menu" aria-label={`Actions for ${connection.configurationLabel}`}>
                      <button class="button button-ghost button-small danger" type="button" aria-label={`Disconnect ${connection.configurationLabel}`} on:click={() => { closeMenu(); onStop(connection.configurationId) }}>Disconnect</button>
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
