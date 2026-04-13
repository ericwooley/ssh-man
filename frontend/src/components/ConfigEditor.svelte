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

<form class="dialog dialog-body gap-md p-form--stacked" on:submit|preventDefault={() => onSubmit(value)}>
  <div class="dialog-header dialog-header--modal">
    <div>
      <p class="eyebrow">Tunnel</p>
      <h2 id="tunnel-dialog-heading">{value.id ? 'Edit tunnel' : 'New tunnel'}</h2>
      <p class="panel-copy">Define the routing and save it for one-click reuse.</p>
    </div>

    <div class="dialog-header-actions">
      {#if server}
        <p class="muted dialog-context-chip">{server.name}</p>
      {/if}
      <button class="p-button--base dialog-close-button" type="button" aria-label="Close dialog" on:click={onCancel}>
        <span aria-hidden="true">&times;</span>
      </button>
    </div>
  </div>

  <section class="p-card form-section" aria-labelledby="tunnel-basics-heading">
    <div class="section-heading">
      <h3 id="tunnel-basics-heading">Basics</h3>
      <p>Name the tunnel and choose the traffic pattern.</p>
    </div>

    <label class:is-error={Boolean(errors.label)} class:field-error={Boolean(errors.label)} class="p-form__group">
      <span>Label</span>
      <input class="p-form-validation__input" bind:this={labelInput} bind:value={value.label} aria-label="Label" placeholder="Staging SOCKS" aria-invalid={errors.label ? 'true' : 'false'} />
      {#if errors.label}<small class="p-form-validation__message error-text">{errors.label}</small>{/if}
    </label>

    <label class="p-form__group">
      <span>Type</span>
      <select class="p-form-validation__input" bind:value={value.connectionType} aria-label="Type">
        <option value="local_forward">Local forward</option>
        <option value="socks_proxy">SOCKS proxy</option>
      </select>
    </label>
  </section>

  <section class="p-card form-section" aria-labelledby="tunnel-routing-heading">
    <div class="section-heading">
      <h3 id="tunnel-routing-heading">Routing</h3>
      <p>{value.connectionType === 'local_forward' ? 'Choose the local listener and remote destination.' : 'Choose the local port that browsers and tools should use.'}</p>
    </div>

    {#if value.connectionType === 'local_forward'}
      <div class="field-grid">
        <label class:is-error={Boolean(errors.localPort)} class:field-error={Boolean(errors.localPort)} class="p-form__group">
          <span>Local port</span>
          <input class="p-form-validation__input" bind:value={value.localPort} aria-label="Local port" inputmode="numeric" aria-invalid={errors.localPort ? 'true' : 'false'} />
          {#if errors.localPort}<small class="p-form-validation__message error-text">{errors.localPort}</small>{/if}
        </label>
        <label class:is-error={Boolean(errors.remoteHost)} class:field-error={Boolean(errors.remoteHost)} class="p-form__group">
          <span>Remote host</span>
          <input class="p-form-validation__input" bind:value={value.remoteHost} aria-label="Remote host" placeholder="127.0.0.1" aria-invalid={errors.remoteHost ? 'true' : 'false'} />
          {#if errors.remoteHost}<small class="p-form-validation__message error-text">{errors.remoteHost}</small>{/if}
        </label>
        <label class:is-error={Boolean(errors.remotePort)} class:field-error={Boolean(errors.remotePort)} class="p-form__group">
          <span>Remote port</span>
          <input class="p-form-validation__input" bind:value={value.remotePort} aria-label="Remote port" inputmode="numeric" aria-invalid={errors.remotePort ? 'true' : 'false'} />
          {#if errors.remotePort}<small class="p-form-validation__message error-text">{errors.remotePort}</small>{/if}
        </label>
      </div>
    {:else}
      <div class="p-card form-section-card">
        <label class="p-form__group">
          <span>SOCKS port mode</span>
          <select class="p-form-validation__input" bind:value={socksPortModeValue} aria-label="SOCKS port mode" on:change={(event) => setSocksPortMode(event.currentTarget.value)}>
            <option value="auto">Auto</option>
            <option value="manual">Manual</option>
          </select>
        </label>

        {#if socksPortModeValue === 'manual'}
          <label class:is-error={Boolean(errors.socksPort)} class:field-error={Boolean(errors.socksPort)} class="p-form__group">
            <span>SOCKS port</span>
            <input class="p-form-validation__input" bind:value={value.socksPort} aria-label="SOCKS port" inputmode="numeric" placeholder="1080" aria-invalid={errors.socksPort ? 'true' : 'false'} />
          </label>
        {:else}
          <div class="muted form-hint" aria-label="SOCKS port auto hint">An open localhost port will be chosen when the tunnel starts.</div>
        {/if}

        {#if errors.socksPort}<small class="p-form-validation__message error-text">{errors.socksPort}</small>{/if}
      </div>
    {/if}
  </section>

  <section class="p-card form-section" aria-labelledby="tunnel-behavior-heading">
    <div class="section-heading">
      <h3 id="tunnel-behavior-heading">Behavior</h3>
      <p>Add context for later and choose how aggressively the tunnel should recover.</p>
    </div>

    <label class="p-form__group">
      <span>Notes</span>
      <textarea class="p-form-validation__input" bind:value={value.notes} aria-label="Notes" rows="4" placeholder="Optional context for the next time you use this tunnel"></textarea>
    </label>

    <label class="p-card checkbox-row checkbox-card">
      <input bind:checked={value.autoReconnectEnabled} type="checkbox" />
      <span>Reconnect automatically after transient disconnects</span>
    </label>
  </section>

  <div class="editor-actions dialog-actions">
    <button class="p-button--positive" type="submit">Save tunnel</button>
    <button class="p-button--base" type="button" on:click={onCancel}>Cancel</button>
  </div>
</form>
