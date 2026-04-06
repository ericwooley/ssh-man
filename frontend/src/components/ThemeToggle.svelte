<script>
  export let theme = 'dark'
  export let onToggle = () => {}
</script>

<button class="theme-toggle" type="button" on:click={onToggle} aria-label="Toggle light and dark theme">
  <span>{theme === 'dark' ? 'Dark mode' : 'Light mode'}</span>
</button>
