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
      <header class="p-modal__header">
        <h2 class="p-modal__title" id="server-dialog-heading">{value.id ? 'Edit server' : 'New server'}</h2>
        <button class="p-modal__close" type="button" aria-label="Close dialog" on:click={onCancel}>Close</button>
      </header>

      <form class="p-form p-form--stacked dialog-body" on:submit|preventDefault={() => onSubmit(value)}>
        <fieldset>
          <legend>Identity</legend>

          <div class="p-form__group {errors.name ? 'p-form-validation is-error' : ''}">
            <label for="server-name" class="p-form__label">Name</label>
            <div class="p-form__control">
              <input
                id="server-name"
                class={errors.name ? 'p-form-validation__input' : ''}
                bind:this={nameInput}
                bind:value={value.name}
                aria-label="Server name"
                aria-invalid={errors.name ? 'true' : 'false'}
                placeholder="Staging bastion"
              />
              {#if errors.name}
                <p class="p-form-validation__message">{errors.name}</p>
              {/if}
            </div>
          </div>

          <div class="p-form__group {errors.host ? 'p-form-validation is-error' : ''}">
            <label for="server-host" class="p-form__label">Host</label>
            <div class="p-form__control">
              <input
                id="server-host"
                class={errors.host ? 'p-form-validation__input' : ''}
                bind:value={value.host}
                aria-label="Server host"
                aria-invalid={errors.host ? 'true' : 'false'}
                placeholder="example.com"
              />
              {#if errors.host}
                <p class="p-form-validation__message">{errors.host}</p>
              {/if}
            </div>
          </div>

          <div class="p-form__group {errors.port ? 'p-form-validation is-error' : ''}">
            <label for="server-port" class="p-form__label">Port</label>
            <div class="p-form__control">
              <input
                id="server-port"
                class={errors.port ? 'p-form-validation__input' : ''}
                bind:value={value.port}
                aria-label="Server port"
                inputmode="numeric"
                aria-invalid={errors.port ? 'true' : 'false'}
              />
              {#if errors.port}
                <p class="p-form-validation__message">{errors.port}</p>
              {/if}
            </div>
          </div>

          <div class="p-form__group {errors.username ? 'p-form-validation is-error' : ''}">
            <label for="server-username" class="p-form__label">Username</label>
            <div class="p-form__control">
              <input
                id="server-username"
                class={errors.username ? 'p-form-validation__input' : ''}
                bind:value={value.username}
                aria-label="SSH username"
                aria-invalid={errors.username ? 'true' : 'false'}
                placeholder="eric"
              />
              {#if errors.username}
                <p class="p-form-validation__message">{errors.username}</p>
              {/if}
            </div>
          </div>
        </fieldset>

        <fieldset>
          <legend>Authentication</legend>

          <div class="p-form__group">
            <label for="server-auth-mode" class="p-form__label">Authentication</label>
            <div class="p-form__control">
              <select id="server-auth-mode" bind:value={value.authMode} aria-label="Authentication mode">
                <option value="private_key">Private key</option>
                <option value="agent">SSH agent</option>
              </select>
            </div>
          </div>

          {#if value.authMode === 'private_key'}
            <div class="p-form__group {errors.keyReference ? 'p-form-validation is-error' : ''}">
              <label for="server-key-ref" class="p-form__label">Private key path</label>
              <div class="p-form__control">
                <input
                  id="server-key-ref"
                  class={errors.keyReference ? 'p-form-validation__input' : ''}
                  bind:value={value.keyReference}
                  aria-label="Private key path"
                  aria-invalid={errors.keyReference ? 'true' : 'false'}
                  placeholder="~/.ssh/id_ed25519"
                />
                {#if errors.keyReference}
                  <p class="p-form-validation__message">{errors.keyReference}</p>
                {/if}
              </div>
            </div>
          {/if}
        </fieldset>

        <div class="p-modal__footer dialog-actions">
          <button class="p-button--positive" type="submit">Save server</button>
          <button class="p-button--base" type="button" on:click={onCancel}>Cancel</button>
        </div>
      </form>
    </div>
  </div>
{/if}
