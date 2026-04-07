<script>
  export let configuration = null
  export let session = null
  export let onStart = () => {}
  export let onStop = () => {}
  export let onRetry = () => {}
</script>

<section class="panel status-panel" aria-labelledby="session-status-heading">
  <div class="panel-header">
    <div>
      <p class="eyebrow">Runtime</p>
      <h2 id="session-status-heading">Session status</h2>
    </div>
    <span class={`status-pill ${session?.status || 'stopped'}`} aria-live="polite" aria-label={`Session status ${session?.status || 'stopped'}`}>{session?.status || 'stopped'}</span>
  </div>

  {#if configuration}
    <p class="status-copy" role="status">{session?.statusDetail || 'This saved tunnel is idle.'}</p>

    <div class="button-row">
      <button class="button button-primary" type="button" on:click={() => onStart(configuration.id)}>Start</button>
      <button class="button button-ghost" type="button" on:click={() => onStop(configuration.id)}>Stop</button>
      <button class="button button-ghost" type="button" on:click={() => onRetry(configuration.id)}>Retry</button>
    </div>
  {:else}
    <div class="empty-state compact">
      <h3>No tunnel selected</h3>
      <p>Select a saved tunnel to start or stop its session.</p>
    </div>
  {/if}
</section>
