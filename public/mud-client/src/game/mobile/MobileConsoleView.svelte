<style>
  .mobile-console {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .terminal-container {
    flex: 1;
    min-height: 0;
    border-radius: 0;
    overflow: hidden;
  }

  /* Override terminal border radius on mobile for flush edges */
  .terminal-container :global(.tx-window) {
    border-radius: 0;
    border-left: none;
    border-right: none;
  }
</style>

<script>
  import TerminalXWidget from '../widgets/TerminalXWidget.svelte';

  export let onTerminalReady = () => {};
  export let onInput = () => {};
  // Passed through so Tab can autocomplete room player names.
  export let store = null;
</script>

<div class="mobile-console">
  <div class="terminal-container">
    <TerminalXWidget {onTerminalReady} {onInput} {store} />
  </div>
</div>
