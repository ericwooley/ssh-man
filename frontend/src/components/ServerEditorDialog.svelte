<script>
  import { tick } from 'svelte'

  export let open = false
  export let value = {
    id: '',
    name: '',
    host: '',
    port: '22',
    username: '',
    authMode: 'agent',
    keyReference: '',
  }
  export let errors = {}
  export let onSubmit = () => {}
  export let onCancel = () => {}

  let nameInput

  $: if (open) {
    tick().then(() => nameInput?.focus())
  }

  function handleKeydown(event) {
    if (open && event.key === 'Escape') {
      onCancel()
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div class="p-modal dialog-backdrop" role="presentation" on:click|self={onCancel}>
    <div class="p-modal__dialog dialog-card dialog" aria-modal="true" role="dialog" aria-labelledby="server-dialog-heading" tabindex="0">
      <form class="modal-editor-card dialog-body gap-md p-form--stacked" on:submit|preventDefault={() => onSubmit(value)}>
        <div class="dialog-header dialog-header--modal">
          <div>
            <p class="eyebrow">Server</p>
            <h2 id="server-dialog-heading">{value.id ? 'Edit server' : 'New server'}</h2>
            <p class="panel-copy">Save the SSH host once, then attach as many tunnels as you need to it.</p>
          </div>
          <div class="dialog-header-actions">
            <button class="p-button--base dialog-close-button" type="button" aria-label="Close dialog" on:click={onCancel}>
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
        </div>

        <section class="p-card form-section" aria-labelledby="server-identity-heading">
          <div class="section-heading">
            <h3 id="server-identity-heading">Identity</h3>
            <p>Give the host a useful name and record the SSH target details.</p>
          </div>

            <label class:is-error={Boolean(errors.name)} class:field-error={Boolean(errors.name)} class="p-form__group">
              <span>Name</span>
            <input class="p-form-validation__input" bind:this={nameInput} bind:value={value.name} aria-label="Server name" aria-invalid={errors.name ? 'true' : 'false'} placeholder="Staging bastion" />
            {#if errors.name}<small class="p-form-validation__message error-text">{errors.name}</small>{/if}
          </label>

          <div class="field-grid server-field-grid">
            <label class:is-error={Boolean(errors.host)} class:field-error={Boolean(errors.host)} class="p-form__group">
              <span>Host</span>
              <input class="p-form-validation__input" bind:value={value.host} aria-label="Server host" aria-invalid={errors.host ? 'true' : 'false'} placeholder="example.com" />
              {#if errors.host}<small class="p-form-validation__message error-text">{errors.host}</small>{/if}
            </label>

            <label class:is-error={Boolean(errors.port)} class:field-error={Boolean(errors.port)} class="p-form__group">
              <span>Port</span>
              <input class="p-form-validation__input" bind:value={value.port} aria-label="Server port" inputmode="numeric" aria-invalid={errors.port ? 'true' : 'false'} />
              {#if errors.port}<small class="p-form-validation__message error-text">{errors.port}</small>{/if}
            </label>

            <label class:is-error={Boolean(errors.username)} class:field-error={Boolean(errors.username)} class="p-form__group">
              <span>Username</span>
              <input class="p-form-validation__input" bind:value={value.username} aria-label="SSH username" aria-invalid={errors.username ? 'true' : 'false'} placeholder="eric" />
              {#if errors.username}<small class="p-form-validation__message error-text">{errors.username}</small>{/if}
            </label>
          </div>
        </section>

        <section class="p-card form-section" aria-labelledby="server-auth-heading">
          <div class="section-heading">
            <h3 id="server-auth-heading">Authentication</h3>
            <p>Choose whether this host should use a private key file or the local SSH agent.</p>
          </div>

          <label class="p-form__group">
            <span>Authentication</span>
            <select class="p-form-validation__input" bind:value={value.authMode} aria-label="Authentication mode">
              <option value="private_key">Private key</option>
              <option value="agent">SSH agent</option>
            </select>
          </label>

          {#if value.authMode === 'private_key'}
            <label class:is-error={Boolean(errors.keyReference)} class:field-error={Boolean(errors.keyReference)} class="p-form__group">
              <span>Private key path</span>
              <input class="p-form-validation__input" bind:value={value.keyReference} aria-label="Private key path" aria-invalid={errors.keyReference ? 'true' : 'false'} placeholder="~/.ssh/id_ed25519" />
              {#if errors.keyReference}<small class="p-form-validation__message error-text">{errors.keyReference}</small>{/if}
            </label>
          {/if}
        </section>

        <div class="editor-actions dialog-actions">
          <button class="p-button--positive" type="submit">Save server</button>
          <button class="p-button--base" type="button" on:click={onCancel}>Cancel</button>
        </div>
      </form>
    </div>
  </div>
{/if}
