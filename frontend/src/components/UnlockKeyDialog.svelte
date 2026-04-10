<script>
  import { tick } from 'svelte'

  export let open = false
  export let configurationLabel = ''
  export let detail = ''
  export let onSubmit = () => {}
  export let onClose = () => {}

  let secret = ''
  let secretInput

  $: if (!open) {
    secret = ''
  }

  $: if (open) {
    tick().then(() => secretInput?.focus())
  }

  function handleSubmit() {
    onSubmit(secret)
  }

  function handleKeydown(event) {
    if (open && event.key === 'Escape') {
      onClose()
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div class="dialog-backdrop" role="presentation" on:click|self={onClose}>
    <div class="dialog-card dialog" aria-modal="true" role="dialog" aria-labelledby="unlock-dialog-heading">
      <form class="dialog-form dialog-body gap-md" on:submit|preventDefault={handleSubmit}>
        <div class="dialog-header">
          <div>
            <p class="eyebrow">Unlock</p>
            <h2 id="unlock-dialog-heading">SSH key passphrase required</h2>
          </div>
        </div>

        <p class="muted">{configurationLabel ? `Enter the passphrase for ${configurationLabel} to continue.` : 'Enter the SSH key passphrase to continue.'}</p>

        {#if detail}
          <p class="dialog-detail">{detail}</p>
        {/if}

        <label>
          <span>SSH key passphrase</span>
          <input bind:this={secretInput} bind:value={secret} type="password" autocomplete="current-password" />
        </label>

        <div class="editor-actions dialog-actions">
          <button class="button button-primary" type="submit" disabled={!secret.trim()}>Unlock key</button>
          <button class="button button-ghost" type="button" on:click={onClose}>Close</button>
        </div>
      </form>
    </div>
  </div>
{/if}
