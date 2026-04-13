<script>
  export let configurations = []
  export let selectedConfigurationId = ''
  export let sessions = []
  export let enabled = true
  export let onSelect = () => {}
  export let onCreate = () => {}
  export let onStartAll = () => {}
  export let onStart = () => {}
  export let onStop = () => {}
  export let onEdit = () => {}
  export let onDelete = () => {}

  const activeStatuses = ['starting', 'connected', 'reconnecting']
  const startableStatuses = ['stopped', 'failed']

  let sessionByConfigurationId = new Map()

  $: sessionByConfigurationId = new Map(sessions.map((session) => [session.configurationId, session]))

  function statusChipClass(status) {
    switch (status) {
      case 'connected':    return 'p-chip--positive'
      case 'reconnecting':
      case 'needs_attention': return 'p-chip--caution'
      case 'failed':       return 'p-chip--negative'
      case 'starting':     return 'p-chip--information'
      default:             return ''
    }
  }

  function statusLabel(status) {
    return (status || 'stopped').replaceAll('_', ' ')
  }

  function canStart(runtime) {
    return !runtime || startableStatuses.includes(runtime.status || 'stopped')
  }

  function canStop(runtime) {
    const status = runtime?.status || 'stopped'
    return activeStatuses.includes(status) || status === 'needs_attention'
  }

  async function handleStart(configurationId) {
    await onSelect(configurationId)
    await onStart(configurationId)
  }

  async function handleStop(configurationId) {
    await onSelect(configurationId)
    await onStop(configurationId)
  }
</script>

<section class:is-disabled={!enabled} class="p-card panel" aria-label="Tunnels">
  <div class="panel-header">
    <h2 class="p-card__title">Tunnels</h2>

    <div class="panel-actions panel-actions--inline">
      <button class="p-button--base" type="button" disabled={!enabled || configurations.length === 0} on:click={onStartAll}>Start all</button>
      <button class="p-button--positive" type="button" disabled={!enabled} on:click={onCreate}>Add tunnel</button>
    </div>
  </div>

  {#if !enabled}
    <div class="empty-state compact">
      <h3 class="p-card__title">Select a target first</h3>
      <p>Choose a saved server before viewing or editing tunnels.</p>
    </div>
  {:else if configurations.length === 0}
    <div class="empty-state compact">
      <h3 class="p-card__title">No saved tunnels</h3>
      <p>Store a local forward or SOCKS proxy under the selected server.</p>
    </div>
  {:else}
    <div class="stack-list">
      {#each configurations as configuration}
        {@const runtime = sessionByConfigurationId.get(configuration.id)}
        <div class={selectedConfigurationId === configuration.id ? 'p-card--highlighted' : 'p-card'}>
          <button
            class="u-no-margin u-no-padding"
            style="background: none; border: none; width: 100%; text-align: left; cursor: pointer; display: block;"
            type="button"
            aria-pressed={selectedConfigurationId === configuration.id}
            aria-label={`Select tunnel ${configuration.label}`}
            on:click={() => onSelect(configuration.id)}
          >
            <strong class="p-card__title">{configuration.label}</strong>
            <p class="p-card__content u-text--muted">
              {#if configuration.connectionType === 'socks_proxy'}
                SOCKS {configuration.socksPort > 0 ? `:${configuration.socksPort}` : 'Auto'}
              {:else}
                {configuration.localPort} → {configuration.remoteHost}:{configuration.remotePort}
              {/if}
            </p>
            {#if runtime?.statusDetail}
              <p class="p-card__content">{runtime.statusDetail}</p>
            {/if}
          </button>
          <hr />
          <div style="display: flex; align-items: center; justify-content: space-between;">
            <span
              class="p-chip is-inline {statusChipClass(runtime?.status)}"
              aria-label={`Tunnel status ${runtime?.status || 'stopped'}`}
            >{statusLabel(runtime?.status)}</span>
            <div class="panel-actions panel-actions--inline panel-actions--compact">
              <button
                class="p-button--positive is-dense"
                type="button"
                disabled={!canStart(runtime)}
                aria-label={`Start ${configuration.label}`}
                on:click={() => handleStart(configuration.id)}
              >Start</button>
              <button
                class="p-button--negative is-dense"
                type="button"
                disabled={!canStop(runtime)}
                aria-label={`Stop ${configuration.label}`}
                on:click={() => handleStop(configuration.id)}
              >Stop</button>
              <button
                class="p-button--base is-dense"
                type="button"
                aria-label={`Edit ${configuration.label}`}
                on:click={() => onEdit(configuration)}
              >Edit</button>
              <button
                class="p-button--negative is-dense"
                type="button"
                aria-label={`Delete ${configuration.label}`}
                on:click={() => onDelete(configuration.id)}
              >Delete</button>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>
