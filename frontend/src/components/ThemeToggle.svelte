<script>
  export let theme = 'dark'
  export let onToggle = () => {}
</script>

<button
  class="p-button--base theme-toggle"
  type="button"
  on:click={onToggle}
  aria-label="Toggle light and dark theme"
  aria-pressed={theme === 'dark'}
>
  <span class="theme-toggle-label">Appearance</span>
  <span class="theme-toggle-value">{theme === 'dark' ? 'Dark mode' : 'Light mode'}</span>
</button>
