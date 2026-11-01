<script>
  import { afterUpdate, tick } from 'svelte';
  import {
    CHAT_TAB_META,
    GLOBAL_CHANNELS,
    buildChannelCommand,
    buildTellCommand,
    channelLabel,
    filterChatByChannel,
    filterChatByTab,
    unreadForTab,
  } from '../chatTabs.js';
  import { chatStore, requestChatCommand } from '../chatStore.js';

  // Reuse the existing room-chat chrome (PlayersOverlay) so the panel does not
  // introduce a new theme. Sizing is driven by the host container.
  let listEl;
  let pinnedToBottom = true;
  let activeChannel = '';

  const EMPTY_TEXT = {
    say: 'No room chat yet. Say something.',
    tell: 'No private messages yet.',
    party: 'No party chat yet.',
    channel: 'No global channel messages yet.',
    system: 'Nothing in the log yet.',
  };

  $: activeTab = $chatStore.activeTab;
  $: tabMessages = filterChatByTab($chatStore.messages, activeTab);
  $: visible = activeTab === 'channel' && activeChannel
    ? filterChatByChannel(tabMessages, activeChannel)
    : tabMessages;

  function handleNameClick(line) {
    // Do not hijack a click that finished a text selection.
    const selection = typeof window !== 'undefined' && window.getSelection ? window.getSelection() : null;
    if (selection && selection.toString().trim().length > 0) return;
    replyTo(line);
  }

  function handleNameKeydown(event, line) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      replyTo(line);
    }
  }

  function lineActionable(line) {
    return !!(line && (line.channel || line.canReply));
  }

  function lineTitle(line) {
    if (!line) return undefined;
    if (line.channel) return `Chat in ${channelLabel(line.channel)}`;
    if (line.canReply) return `Tell ${line.name}`;
    return undefined;
  }

  function replyTo(line) {
    if (!line) return;
    // Global channel lines prefill `chat <channel> ` instead of a tell.
    if (line.channel) {
      const channelCmd = buildChannelCommand(line.channel);
      if (channelCmd) requestChatCommand(channelCmd);
      return;
    }
    if (!line.canReply) return;
    const command = buildTellCommand(line.name);
    if (command) requestChatCommand(command);
  }

  function quickReplyChannel(channel) {
    const command = buildChannelCommand(channel);
    if (command) requestChatCommand(command);
  }

  function setChannelFilter(channel) {
    activeChannel = channel || '';
    pinnedToBottom = true;
    scrollToBottom();
  }

  function selectTab(tab) {
    if (!tab || tab.disabled) return;
    chatStore.setActiveTab(tab.id);
    pinnedToBottom = true;
    scrollToBottom();
  }

  function onScroll() {
    if (!listEl) return;
    const distance = listEl.scrollHeight - listEl.scrollTop - listEl.clientHeight;
    pinnedToBottom = distance < 24;
  }

  async function scrollToBottom() {
    await tick();
    if (listEl) listEl.scrollTop = listEl.scrollHeight;
  }

  afterUpdate(() => {
    if (pinnedToBottom) scrollToBottom();
  });
</script>

<style>
  .chat-panel {
    display: flex;
    flex-direction: column;
    min-height: 0;
    flex: 1;
    background: var(--panel-bg, rgba(12, 8, 4, 0.94));
    border: 1px solid var(--panel-border, rgba(212, 175, 55, 0.32));
    border-radius: var(--panel-radius, 10px);
    overflow: hidden;
  }

  /* ── Tab bar ── */
  .chat-tabs {
    display: flex;
    flex-shrink: 0;
    gap: 2px;
    padding: 0.3em 0.35em 0;
    border-bottom: 1px solid var(--panel-header-border, rgba(180, 130, 60, 0.22));
    background: var(--panel-header-bg, linear-gradient(180deg, rgba(180, 130, 60, 0.1) 0%, transparent 100%));
  }

  .chat-tab {
    position: relative;
    flex: 1;
    min-width: 0;
    padding: 0.32em 0.4em 0.38em;
    border: 0;
    border-radius: 5px 5px 0 0;
    background: transparent;
    color: var(--text-secondary, #a89878);
    font-family: var(--font-display, 'Cinzel', serif);
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    cursor: pointer;
    transition: color 0.15s ease, background 0.15s ease;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    -webkit-tap-highlight-color: transparent;
  }

  .chat-tab:hover:not(:disabled) {
    color: var(--text-primary, #e8dcc8);
    background: rgba(180, 130, 60, 0.12);
  }

  .chat-tab.active {
    color: var(--accent-primary, #d4a44a);
    background: rgba(180, 130, 60, 0.16);
    box-shadow: inset 0 -2px 0 var(--accent-primary, #d4a44a);
  }

  .chat-tab:disabled {
    color: var(--text-dim, #6d5f48);
    opacity: 0.55;
    cursor: not-allowed;
  }

  .chat-badge {
    display: inline-block;
    min-width: 15px;
    margin-left: 3px;
    padding: 0 4px;
    border-radius: 8px;
    background: var(--accent-primary, #d4a44a);
    color: #1b1207;
    font-size: 9px;
    font-weight: 700;
    line-height: 15px;
    text-align: center;
    vertical-align: middle;
    font-variant-numeric: tabular-nums;
  }

  /* ── Channel sub-filter (Ch tab) ── */
  .channel-bar {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    gap: 3px;
    padding: 0.25em 0.35em;
    overflow-x: auto;
    border-bottom: 1px solid var(--panel-header-border, rgba(180, 130, 60, 0.22));
    background: rgba(180, 130, 60, 0.05);
    scrollbar-width: none;
  }

  .channel-bar::-webkit-scrollbar {
    display: none;
  }

  .channel-chip-group {
    display: inline-flex;
    align-items: stretch;
    flex-shrink: 0;
  }

  .channel-chip {
    flex-shrink: 0;
    padding: 0.2em 0.5em;
    border: 1px solid var(--panel-header-border, rgba(180, 130, 60, 0.28));
    border-radius: 999px;
    background: transparent;
    color: var(--text-secondary, #a89878);
    font-family: var(--font-display, 'Cinzel', serif);
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 0.03em;
    cursor: pointer;
    white-space: nowrap;
    -webkit-tap-highlight-color: transparent;
  }

  .channel-chip:hover {
    color: var(--text-primary, #e8dcc8);
    background: rgba(180, 130, 60, 0.12);
  }

  .channel-chip.active {
    color: #1b1207;
    background: var(--accent-primary, #d4a44a);
    border-color: var(--accent-primary, #d4a44a);
  }

  .channel-reply {
    flex-shrink: 0;
    margin-left: -4px;
    padding: 0 0.35em;
    border: 1px solid var(--panel-header-border, rgba(180, 130, 60, 0.28));
    border-radius: 0 999px 999px 0;
    background: transparent;
    color: var(--accent-primary, #d4a44a);
    font-size: 10px;
    line-height: 1;
    cursor: pointer;
  }

  .channel-reply:hover {
    background: rgba(180, 130, 60, 0.18);
  }

  .chat-channel {
    align-self: flex-start;
    margin-bottom: 0.1em;
    font-family: var(--font-display, 'Cinzel', serif);
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--text-dim, #6d5f48);
    cursor: pointer;
    border-bottom: 1px dotted transparent;
  }

  .chat-channel:hover,
  .chat-channel:focus-visible {
    outline: none;
    color: var(--accent-primary, #d4a44a);
    border-bottom-color: currentColor;
  }

  .chat-channel.active {
    color: var(--accent-primary, #d4a44a);
  }

  /* ── Message log ── */
  .chat-list {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0.15em 0.35em 0.35em;
    scrollbar-width: thin;
    scrollbar-color: rgba(180, 130, 60, 0.45) transparent;
  }

  .chat-list::-webkit-scrollbar {
    width: 6px;
  }

  .chat-list::-webkit-scrollbar-thumb {
    background: rgba(180, 130, 60, 0.45);
    border-radius: 3px;
  }

  .chat-empty {
    font-size: 12.5px;
    color: var(--text-dim, #6d5f48);
    padding: 0.45em 0.25em;
    font-style: italic;
  }

  .chat-line {
    display: flex;
    flex-direction: column;
    gap: 0.08em;
    padding: 0.32em 0.2em 0.38em;
    border-bottom: 1px solid transparent;
    background-image: linear-gradient(
      90deg,
      transparent 0%,
      rgba(180, 130, 60, 0.12) 20%,
      rgba(180, 130, 60, 0.12) 80%,
      transparent 100%
    );
    background-size: 100% 1px;
    background-repeat: no-repeat;
    background-position: bottom;
  }

  .chat-line:last-child {
    background-image: none;
  }

  .chat-who {
    align-self: flex-start;
    font-family: var(--font-display, 'Cinzel', serif);
    font-size: 11.5px;
    font-weight: 600;
    letter-spacing: 0.06em;
    color: var(--accent-primary, #d4a44a);
    padding: 0;
    border: 0;
    background: none;
  }

  .chat-who.you {
    color: var(--color-gold, #fbbf24);
  }

  .chat-who.replyable {
    cursor: pointer;
    border-bottom: 1px dotted transparent;
  }

  .chat-who.replyable:hover,
  .chat-who.replyable:focus-visible {
    outline: none;
    border-bottom-color: currentColor;
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .chat-text {
    font-size: 13.5px;
    color: var(--text-primary, #e8dcc8);
    line-height: 1.4;
    word-break: break-word;
  }

  .chat-line.system .chat-text {
    color: var(--text-secondary, #a89878);
    font-size: 12.5px;
  }

  /* ── Compact / mobile ── */
  @media (max-width: 640px) {
    .chat-tab {
      font-size: 10px;
      padding: 0.3em 0.25em 0.36em;
      letter-spacing: 0.02em;
    }

    .chat-text {
      font-size: 13px;
    }
  }
</style>

<div class="chat-panel">
  <div class="chat-tabs" role="tablist" aria-label="Chat channels">
    {#each CHAT_TAB_META as tab (tab.id)}
      {@const count = tab.disabled ? 0 : unreadForTab($chatStore.unread, tab.id)}
      <button
        type="button"
        class="chat-tab"
        class:active={activeTab === tab.id}
        role="tab"
        aria-selected={activeTab === tab.id}
        disabled={tab.disabled}
        title={tab.title || tab.label}
        on:click={() => selectTab(tab)}
      >
        {tab.label}{#if count > 0}<span class="chat-badge">{count}</span>{/if}
      </button>
    {/each}
  </div>

  {#if activeTab === 'channel'}
    <div class="channel-bar" role="group" aria-label="Channel filter">
      <button
        type="button"
        class="channel-chip"
        class:active={!activeChannel}
        on:click={() => setChannelFilter('')}
      >All</button>
      {#each GLOBAL_CHANNELS as channel (channel.id)}
        <span class="channel-chip-group">
          <button
            type="button"
            class="channel-chip"
            class:active={activeChannel === channel.id}
            title={`Filter ${channel.label}`}
            on:click={() => setChannelFilter(channel.id)}
          >{channel.label}</button>
          <button
            type="button"
            class="channel-reply"
            title={`Reply in ${channel.label}`}
            aria-label={`Reply in ${channel.label}`}
            on:click={() => quickReplyChannel(channel.id)}
          >↩</button>
        </span>
      {/each}
    </div>
  {/if}

  <div class="chat-list" bind:this={listEl} on:scroll={onScroll} role="tabpanel">
    {#if visible.length === 0}
      <div class="chat-empty">{EMPTY_TEXT[activeTab] || 'No messages yet.'}</div>
    {:else}
      {#each visible as line (line.id)}
        <div class="chat-line" class:system={line.system && !line.name}>
          {#if line.channel}
            <span
              class="chat-channel"
              class:active={activeChannel === line.channel}
              role="button"
              tabindex="0"
              title={`Filter ${channelLabel(line.channel)}`}
              on:click|stopPropagation={() => setChannelFilter(line.channel)}
              on:keydown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setChannelFilter(line.channel); } }}
            >[{channelLabel(line.channel)}]</span>
          {/if}
          {#if line.name}
            <!-- svelte-ignore a11y-no-noninteractive-tabindex -->
            <span
              class="chat-who"
              class:you={line.isYou}
              class:replyable={lineActionable(line)}
              role={lineActionable(line) ? 'button' : undefined}
              tabindex={lineActionable(line) ? 0 : undefined}
              title={lineTitle(line)}
              on:click|stopPropagation={() => handleNameClick(line)}
              on:keydown={(event) => handleNameKeydown(event, line)}
            >{line.isYou ? `${line.name} (YOU)` : line.name}</span>
          {/if}
          <span class="chat-text">{line.text}</span>
        </div>
      {/each}
    {/if}
  </div>
</div>
