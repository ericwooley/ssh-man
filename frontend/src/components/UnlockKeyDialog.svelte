<script>
  export let open = false
  export let onClose = () => {}
</script>

{#if open}
  <div class="dialog-backdrop" role="presentation">
    <div class="dialog-card" aria-modal="true" role="dialog" aria-labelledby="unlock-dialog-heading">
      <h2 id="unlock-dialog-heading">Unlock required</h2>
      <p>The active SSH key needs a passphrase before the tunnel can continue.</p>
      <button class="button button-primary" type="button" on:click={onClose}>Close</button>
    </div>
  </div>
{/if}
