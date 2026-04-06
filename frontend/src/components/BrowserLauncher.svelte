<script>
  export let configuration = null
  export let session = null
  export let browsers = []
  export let selectedBrowserId = ''
  export let onSelect = () => {}
  export let onRefresh = () => {}
  export let onLaunch = () => {}

  $: canLaunch = configuration && configuration.connectionType === 'socks_proxy' && session?.status === 'connected'
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
    <label>
      <span>Installed browser</span>
      <select bind:value={selectedBrowserId} on:change={(event) => onSelect(event.currentTarget.value)}>
        <option value="">Select a browser</option>
        {#each browsers as browser}
          <option disabled={!browser.supportsProxyLaunch} value={browser.id}>{browser.displayName}</option>
        {/each}
      </select>
    </label>

    {#if selectedBrowserId}
      {@const selectedBrowser = browsers.find((browser) => browser.id === selectedBrowserId)}
      {#if selectedBrowser && !selectedBrowser.supportsProxyLaunch}
        <p class="error-text" role="alert">This browser was found, but this MVP cannot launch it through a SOCKS proxy on this platform.</p>
      {/if}
    {/if}

    <button class="button button-primary" disabled={!selectedBrowserId || !browsers.find((browser) => browser.id === selectedBrowserId)?.supportsProxyLaunch} type="button" on:click={() => onLaunch(configuration.id, selectedBrowserId)}>
      Launch through SOCKS
    </button>
  {/if}
</section>
