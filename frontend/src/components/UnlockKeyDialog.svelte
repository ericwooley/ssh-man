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
  <div class="p-modal dialog-backdrop" role="presentation" on:click|self={onClose}>
    <div class="p-modal__dialog dialog-card dialog" aria-modal="true" role="dialog" aria-labelledby="unlock-dialog-heading">
      <form class="dialog-form dialog-body gap-md p-form--stacked" on:submit|preventDefault={handleSubmit}>
        <div class="dialog-header dialog-header--modal">
          <div>
            <p class="eyebrow">Unlock</p>
            <h2 id="unlock-dialog-heading">SSH key passphrase required</h2>
            <p class="panel-copy">Provide the passphrase so the blocked tunnel can continue.</p>
          </div>
          <div class="dialog-header-actions">
            <button class="p-button--base dialog-close-button" type="button" aria-label="Close dialog" on:click={onClose}>
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
        </div>

        <p class="muted">{configurationLabel ? `Enter the passphrase for ${configurationLabel} to continue.` : 'Enter the SSH key passphrase to continue.'}</p>

        {#if detail}
          <div class="dialog-detail p-notification p-notification--caution is-borderless">
            <div class="p-notification__content">
              <p class="p-notification__message">{detail}</p>
            </div>
          </div>
        {/if}

        <label class="p-form__group">
          <span>SSH key passphrase</span>
          <input class="p-form-validation__input" bind:this={secretInput} bind:value={secret} type="password" autocomplete="current-password" />
        </label>

        <div class="editor-actions dialog-actions">
          <button class="p-button" type="submit" disabled={!secret.trim()}>Unlock key</button>
          <button class="p-button--base" type="button" on:click={onClose}>Close</button>
        </div>
      </form>
    </div>
  </div>
{/if}
