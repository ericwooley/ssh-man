<script>
  import { tick } from 'svelte'

  export let server = null
  export let value = {
    serverId: '',
    label: '',
    connectionType: 'local_forward',
    localPort: '',
    remoteHost: '',
    remotePort: '',
    socksPort: '',
    autoReconnectEnabled: true,
    notes: '',
  }
  export let errors = {}
  export let onSubmit = () => {}
  export let onCancel = () => {}

  let labelInput
  let socksPortModeValue = socksPortMode(value.socksPort)

  function socksPortMode(currentValue) {
    return currentValue === '' || currentValue === 'auto' || Number(currentValue) === 0 ? 'auto' : 'manual'
  }

  function setSocksPortMode(mode) {
    socksPortModeValue = mode
    value = {
      ...value,
      socksPort: mode === 'auto' ? 'auto' : '',
    }
  }

  $: socksPortModeValue = value.connectionType === 'socks_proxy'
    ? (value.socksPort === '' && socksPortModeValue === 'manual' ? 'manual' : socksPortMode(value.socksPort))
    : 'auto'

  tick().then(() => labelInput?.focus())
</script>

<form class="p-form p-form--stacked" on:submit|preventDefault={() => onSubmit(value)}>
  <header class="p-modal__header">
    <h2 class="p-modal__title" id="tunnel-dialog-heading">{value.id ? 'Edit tunnel' : 'New tunnel'}</h2>
    <div style="display: flex; align-items: center; gap: 0.5rem;">
      {#if server}
        <span class="u-text--muted">{server.name}</span>
      {/if}
      <button class="p-modal__close" type="button" aria-label="Close dialog" on:click={onCancel}>Close</button>
    </div>
  </header>

  <fieldset>
    <legend>Basics</legend>

    <div class="p-form__group {errors.label ? 'p-form-validation is-error field-error' : ''}">
      <label for="tunnel-label" class="p-form__label">Label</label>
      <div class="p-form__control">
        <input
          id="tunnel-label"
          class={errors.label ? 'p-form-validation__input' : ''}
          bind:this={labelInput}
          bind:value={value.label}
          aria-label="Label"
          placeholder="Staging SOCKS"
          aria-invalid={errors.label ? 'true' : 'false'}
        />
        {#if errors.label}
          <p class="p-form-validation__message">{errors.label}</p>
        {/if}
      </div>
    </div>

    <div class="p-form__group">
      <label for="tunnel-type" class="p-form__label">Type</label>
      <div class="p-form__control">
        <select id="tunnel-type" bind:value={value.connectionType} aria-label="Type">
          <option value="local_forward">Local forward</option>
          <option value="socks_proxy">SOCKS proxy</option>
        </select>
      </div>
    </div>
  </fieldset>

  <fieldset>
    <legend>Routing</legend>

    {#if value.connectionType === 'local_forward'}
      <div class="p-form__group {errors.localPort ? 'p-form-validation is-error field-error' : ''}">
        <label for="tunnel-local-port" class="p-form__label">Local port</label>
        <div class="p-form__control">
          <input
            id="tunnel-local-port"
            type="number"
            min="1"
            max="65535"
            step="1"
            class={errors.localPort ? 'p-form-validation__input' : ''}
            bind:value={value.localPort}
            aria-label="Local port"
            inputmode="numeric"
            aria-invalid={errors.localPort ? 'true' : 'false'}
          />
          {#if errors.localPort}
            <p class="p-form-validation__message">{errors.localPort}</p>
          {/if}
        </div>
      </div>

      <div class="p-form__group {errors.remoteHost ? 'p-form-validation is-error field-error' : ''}">
        <label for="tunnel-remote-host" class="p-form__label">Remote host</label>
        <div class="p-form__control">
          <input
            id="tunnel-remote-host"
            class={errors.remoteHost ? 'p-form-validation__input' : ''}
            bind:value={value.remoteHost}
            aria-label="Remote host"
            placeholder="127.0.0.1"
            aria-invalid={errors.remoteHost ? 'true' : 'false'}
          />
          {#if errors.remoteHost}
            <p class="p-form-validation__message">{errors.remoteHost}</p>
          {/if}
        </div>
      </div>

      <div class="p-form__group {errors.remotePort ? 'p-form-validation is-error field-error' : ''}">
        <label for="tunnel-remote-port" class="p-form__label">Remote port</label>
        <div class="p-form__control">
          <input
            id="tunnel-remote-port"
            type="number"
            min="1"
            max="65535"
            step="1"
            class={errors.remotePort ? 'p-form-validation__input' : ''}
            bind:value={value.remotePort}
            aria-label="Remote port"
            inputmode="numeric"
            aria-invalid={errors.remotePort ? 'true' : 'false'}
          />
          {#if errors.remotePort}
            <p class="p-form-validation__message">{errors.remotePort}</p>
          {/if}
        </div>
      </div>
    {:else}
      <div class="p-form__group">
        <label for="tunnel-socks-mode" class="p-form__label">SOCKS port mode</label>
        <div class="p-form__control">
          <select
            id="tunnel-socks-mode"
            bind:value={socksPortModeValue}
            aria-label="SOCKS port mode"
            on:change={(event) => setSocksPortMode(event.currentTarget.value)}
          >
            <option value="auto">Auto</option>
            <option value="manual">Manual</option>
          </select>
        </div>
      </div>

      {#if socksPortModeValue === 'manual'}
        <div class="p-form__group {errors.socksPort ? 'p-form-validation is-error field-error' : ''}">
          <label for="tunnel-socks-port" class="p-form__label">SOCKS port</label>
          <div class="p-form__control">
            <input
              id="tunnel-socks-port"
              type="number"
              min="1"
              max="65535"
              step="1"
              class={errors.socksPort ? 'p-form-validation__input' : ''}
              bind:value={value.socksPort}
              aria-label="SOCKS port"
              inputmode="numeric"
              placeholder="1080"
              aria-invalid={errors.socksPort ? 'true' : 'false'}
            />
            {#if errors.socksPort}
              <p class="p-form-validation__message">{errors.socksPort}</p>
            {/if}
          </div>
        </div>
      {:else}
        <p class="p-form-help-text" aria-label="SOCKS port auto hint">An open localhost port will be chosen when the tunnel starts.</p>
      {/if}
    {/if}
  </fieldset>

  <fieldset>
    <legend>Behavior</legend>

    <div class="p-form__group">
      <label for="tunnel-notes" class="p-form__label">Notes</label>
      <div class="p-form__control">
        <textarea
          id="tunnel-notes"
          bind:value={value.notes}
          aria-label="Notes"
          rows="4"
          placeholder="Optional context for the next time you use this tunnel"
        ></textarea>
      </div>
    </div>

    <div class="p-form__group">
      <label class="p-checkbox">
        <input class="p-checkbox__input" bind:checked={value.autoReconnectEnabled} type="checkbox" />
        <span class="p-checkbox__label">Reconnect automatically after transient disconnects</span>
      </label>
    </div>
  </fieldset>

  <div class="p-modal__footer">
    <button class="p-button--positive" type="submit">Save tunnel</button>
    <button class="p-button--base" type="button" on:click={onCancel}>Cancel</button>
  </div>
</form>
