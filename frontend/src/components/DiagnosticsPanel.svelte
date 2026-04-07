<script>
  export let diagnostics = { appDataPath: '', databasePath: '' }
  export let issue = ''
  export let hasWarning = false
  export let onReload = () => {}
  export let onOpenDevTools = () => {}
  export let onCopyPath = () => {}
</script>

<section class="panel diagnostics-panel" aria-labelledby="diagnostics-heading">
  <div class="panel-header">
    <div>
      <p class="eyebrow">Diagnostics</p>
      <h2 id="diagnostics-heading">Storage and recovery</h2>
    </div>
    <span class={`pill ${hasWarning ? 'pill-warning' : ''}`}>{hasWarning ? 'Attention' : 'Healthy'}</span>
  </div>

  <div class="diagnostics-block">
    <div class="diagnostics-row">
      <span class="diagnostics-label">App data</span>
      <button class="button button-ghost button-small" type="button" disabled={!diagnostics.appDataPath} on:click={() => onCopyPath('App data path', diagnostics.appDataPath)}>Copy path</button>
    </div>
    <code>{diagnostics.appDataPath || 'Unavailable'}</code>
  </div>

  <div class="diagnostics-block">
    <div class="diagnostics-row">
      <span class="diagnostics-label">Database</span>
      <button class="button button-ghost button-small" type="button" disabled={!diagnostics.databasePath} on:click={() => onCopyPath('Database path', diagnostics.databasePath)}>Copy path</button>
    </div>
    <code>{diagnostics.databasePath || 'Unavailable'}</code>
  </div>

  {#if issue}
    <div class="diagnostics-issue" role={hasWarning ? 'alert' : 'status'}>
      <strong>Current issue</strong>
      <pre class="banner-detail">{issue}</pre>
    </div>
  {:else}
    <div class="empty-state compact">
      <h3>No active storage warnings</h3>
      <p>Saved data is loading normally. Use the tools below if something stops behaving as expected.</p>
    </div>
  {/if}

  <div class="button-row diagnostics-actions">
    <button class="button button-ghost" type="button" on:click={onReload}>Reload saved data</button>
    <button class="button button-ghost" type="button" on:click={onOpenDevTools}>Open devtools</button>
  </div>
</section>
