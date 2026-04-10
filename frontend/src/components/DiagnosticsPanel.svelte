<script>
  export let diagnostics = { appDataPath: '', databasePath: '' }
  export let issue = ''
  export let hasWarning = false
  export let onReload = () => {}
  export let onOpenDevTools = () => {}
  export let onCopyPath = () => {}
</script>

<section class="p-card panel diagnostics-panel" aria-labelledby="diagnostics-heading">
  <div class="p-card__header panel-header">
    <div>
      <p class="eyebrow">Diagnostics</p>
      <h2 id="diagnostics-heading">Storage and recovery</h2>
    </div>
    <span class={`p-chip is-inline ${hasWarning ? 'p-chip--caution' : 'p-chip--positive'}`}>{hasWarning ? 'Attention' : 'Healthy'}</span>
  </div>

  <div class="p-card diagnostics-block">
    <div class="diagnostics-row">
      <span class="diagnostics-label">App data</span>
      <button class="p-button--base" type="button" disabled={!diagnostics.appDataPath} on:click={() => onCopyPath('App data path', diagnostics.appDataPath)}>Copy path</button>
    </div>
    <code>{diagnostics.appDataPath || 'Unavailable'}</code>
  </div>

  <div class="p-card diagnostics-block">
    <div class="diagnostics-row">
      <span class="diagnostics-label">Database</span>
      <button class="p-button--base" type="button" disabled={!diagnostics.databasePath} on:click={() => onCopyPath('Database path', diagnostics.databasePath)}>Copy path</button>
    </div>
    <code>{diagnostics.databasePath || 'Unavailable'}</code>
  </div>

  {#if issue}
    <div class={`p-notification diagnostics-issue ${hasWarning ? 'p-notification--caution' : ''}`} role={hasWarning ? 'alert' : 'status'}>
      <div class="p-notification__content">
        <h3 class="p-notification__title">Current issue</h3>
        <pre class="banner-detail p-notification__message">{issue}</pre>
      </div>
    </div>
  {:else}
    <div class="empty-state compact">
      <h3>No active storage warnings</h3>
      <p>Saved data is loading normally. Use the tools below if something stops behaving as expected.</p>
    </div>
  {/if}

  <div class="button-row diagnostics-actions">
    <button class="p-button--base" type="button" on:click={onReload}>Reload saved data</button>
    <button class="p-button--base" type="button" on:click={onOpenDevTools}>Open devtools</button>
  </div>
</section>
