<script>
  export let configuration = null
  export let session = null
  export let browsers = []
  export let selectedBrowserId = ''
  export let launchPreview = ''
  export let onSelect = () => {}
  export let onRefresh = () => {}
  export let onLaunch = () => {}

  $: canLaunch = configuration && configuration.connectionType === 'socks_proxy' && session?.status === 'connected'
  $: selectedBrowser = browsers.find((browser) => browser.id === selectedBrowserId) || null
  $: canLaunchSelectedBrowser = Boolean(selectedBrowser?.supportsProxyLaunch)
</script>

<section class="p-card panel browser-panel" aria-labelledby="browser-launch-heading">
  <div class="panel-header">
    <div>
      <p class="eyebrow">Browser launch</p>
      <h2 id="browser-launch-heading">SOCKS-aware browser</h2>
      <p class="panel-copy">Launch a browser inside the focused SOCKS tunnel.</p>
    </div>
    <button class="p-button--base is-dense" type="button" on:click={onRefresh}>Refresh</button>
  </div>

  {#if !canLaunch}
    <p class="muted">Start a SOCKS tunnel to enable browser launch.</p>
  {:else}
    <div class="compact-form-grid p-form--stacked">
      <label class="p-form__group">
        <span>Installed browser</span>
        <select class="p-form-validation__input" bind:value={selectedBrowserId} on:change={(event) => onSelect(event.currentTarget.value)}>
          <option value="">Select a browser</option>
          {#each browsers as browser}
            <option value={browser.id}>{browser.displayName}{browser.supportsProxyLaunch ? '' : ' (unsupported)'}</option>
          {/each}
        </select>
      </label>

      <button class="p-button--positive is-dense" disabled={!selectedBrowserId || !canLaunchSelectedBrowser} type="button" on:click={() => onLaunch(configuration.id, selectedBrowserId)}>
        Launch through SOCKS
      </button>
    </div>

    {#if selectedBrowser && !selectedBrowser.supportsProxyLaunch}
      <div class="p-notification p-notification--negative is-borderless" role="alert">
        <div class="p-notification__content">
          <p class="p-notification__message">{selectedBrowser.displayName} was found, but macOS does not support launching it with a per-app SOCKS proxy. Use Chrome, Chromium, Brave, or Firefox for isolated tunnel launches.</p>
        </div>
      </div>
    {/if}

    {#if launchPreview}
      <div class="p-card browser-command-block">
        <span>Launch command</span>
        <pre class="banner-detail browser-command-preview">{launchPreview}</pre>
      </div>
    {/if}
  {/if}
</section>
