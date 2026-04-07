<script>
  export let configuration = null
  export let session = null
  export let browsers = []
  export let selectedBrowserId = ''
  export let onSelect = () => {}
  export let onRefresh = () => {}
  export let onLaunch = () => {}

  $: canLaunch = configuration && configuration.connectionType === 'socks_proxy' && session?.status === 'connected'
  $: selectedBrowser = browsers.find((browser) => browser.id === selectedBrowserId) || null
  $: canLaunchSelectedBrowser = Boolean(selectedBrowser?.supportsProxyLaunch)
</script>

<section class="panel browser-panel" aria-labelledby="browser-launch-heading">
  <div class="panel-header">
    <div>
      <p class="eyebrow">Browser launch</p>
      <h2 id="browser-launch-heading">SOCKS-aware browser</h2>
    </div>
    <button class="button button-ghost" type="button" on:click={onRefresh}>Refresh browsers</button>
  </div>

  {#if !canLaunch}
    <p class="muted">Start a SOCKS tunnel to enable browser launch.</p>
  {:else}
    <div class="compact-form-grid">
      <label>
        <span>Installed browser</span>
        <select bind:value={selectedBrowserId} on:change={(event) => onSelect(event.currentTarget.value)}>
          <option value="">Select a browser</option>
          {#each browsers as browser}
            <option value={browser.id}>{browser.displayName}{browser.supportsProxyLaunch ? '' : ' (unsupported)'}</option>
          {/each}
        </select>
      </label>

      <button class="button button-primary" disabled={!selectedBrowserId || !canLaunchSelectedBrowser} type="button" on:click={() => onLaunch(configuration.id, selectedBrowserId)}>
        Launch through SOCKS
      </button>
    </div>

    {#if selectedBrowser && !selectedBrowser.supportsProxyLaunch}
      <p class="error-text" role="alert">{selectedBrowser.displayName} was found, but this app cannot launch it through a SOCKS proxy on this platform yet.</p>
    {/if}
  {/if}
</section>
