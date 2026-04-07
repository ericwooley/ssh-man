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

  tick().then(() => labelInput?.focus())
</script>

<form class="editor-card" on:submit|preventDefault={() => onSubmit(value)}>
  <div class="editor-header">
    <div>
      <p class="eyebrow">Editor</p>
      <h2>{value.id ? 'Edit tunnel' : 'New tunnel'}</h2>
      <p class="panel-copy">Define how traffic should move through this SSH host, then save it for one-click reuse.</p>
    </div>
    {#if server}
      <p class="muted">{server.name}</p>
    {/if}
  </div>

  <section class="form-section" aria-labelledby="tunnel-basics-heading">
    <div class="section-heading">
      <h3 id="tunnel-basics-heading">Basics</h3>
      <p>Name the tunnel and choose the traffic pattern.</p>
    </div>

    <label>
      <span>Label</span>
      <input bind:this={labelInput} bind:value={value.label} aria-label="Label" placeholder="Staging SOCKS" aria-invalid={errors.label ? 'true' : 'false'} />
      {#if errors.label}<small class="error-text">{errors.label}</small>{/if}
    </label>

    <label>
      <span>Type</span>
      <select bind:value={value.connectionType} aria-label="Type">
        <option value="local_forward">Local forward</option>
        <option value="socks_proxy">SOCKS proxy</option>
      </select>
    </label>
  </section>

  <section class="form-section" aria-labelledby="tunnel-routing-heading">
    <div class="section-heading">
      <h3 id="tunnel-routing-heading">Routing</h3>
      <p>{value.connectionType === 'local_forward' ? 'Choose the local listener and remote destination.' : 'Choose the local port that browsers and tools should use.'}</p>
    </div>

    {#if value.connectionType === 'local_forward'}
      <div class="field-grid">
        <label>
          <span>Local port</span>
          <input bind:value={value.localPort} aria-label="Local port" inputmode="numeric" aria-invalid={errors.localPort ? 'true' : 'false'} />
          {#if errors.localPort}<small class="error-text">{errors.localPort}</small>{/if}
        </label>
        <label>
          <span>Remote host</span>
          <input bind:value={value.remoteHost} aria-label="Remote host" placeholder="127.0.0.1" aria-invalid={errors.remoteHost ? 'true' : 'false'} />
          {#if errors.remoteHost}<small class="error-text">{errors.remoteHost}</small>{/if}
        </label>
        <label>
          <span>Remote port</span>
          <input bind:value={value.remotePort} aria-label="Remote port" inputmode="numeric" aria-invalid={errors.remotePort ? 'true' : 'false'} />
          {#if errors.remotePort}<small class="error-text">{errors.remotePort}</small>{/if}
        </label>
      </div>
    {:else}
      <label>
        <span>SOCKS port</span>
        <input bind:value={value.socksPort} aria-label="SOCKS port" inputmode="numeric" aria-invalid={errors.socksPort ? 'true' : 'false'} />
        {#if errors.socksPort}<small class="error-text">{errors.socksPort}</small>{/if}
      </label>
    {/if}
  </section>

  <section class="form-section" aria-labelledby="tunnel-behavior-heading">
    <div class="section-heading">
      <h3 id="tunnel-behavior-heading">Behavior</h3>
      <p>Add context for later and choose how aggressively the tunnel should recover.</p>
    </div>

    <label>
      <span>Notes</span>
      <textarea bind:value={value.notes} aria-label="Notes" rows="4" placeholder="Optional context for the next time you use this tunnel"></textarea>
    </label>

    <label class="checkbox-row checkbox-card">
      <input bind:checked={value.autoReconnectEnabled} type="checkbox" />
      <span>Reconnect automatically after transient disconnects</span>
    </label>
  </section>

  <div class="editor-actions">
    <button class="button button-primary" type="submit">Save tunnel</button>
    <button class="button button-ghost" type="button" on:click={onCancel}>Cancel</button>
  </div>
</form>
