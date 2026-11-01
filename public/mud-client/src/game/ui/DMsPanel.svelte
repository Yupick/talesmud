<script>
  import { afterUpdate, onDestroy, tick } from 'svelte';
  import { dmsPanelOpen } from '../uiChrome.js';
  import { overlayStore } from './overlayStore.js';
  import {
    getDMChannels,
    createDMChannel,
    getDMMessages,
    sendDMMessage,
    markDMChannelRead,
  } from '../../api/dms.js';

  export let store = null;
  export let authToken = '';

  const POLL_MS = 5000;

  let wasOpen = false;
  let escHandler = null;
  let pollTimer = null;

  let loading = false;
  let busy = false;
  let error = '';
  let channels = [];
  let activeChannelId = '';
  let messages = [];
  let draft = '';
  let newTarget = '';
  let pinnedToBottom = true;
  let listEl = null;

  $: open = $dmsPanelOpen;
  $: me = (store && $store && $store.character) || null;
  $: myId = me && me.id ? String(me.id) : '';
  $: roomNames = buildRoomNames(store && $store ? $store.players : []);
  $: activeChannel = channels.find((c) => c && c.id === activeChannelId) || null;
  $: partnerId = activeChannel ? partnerOf(activeChannel, myId) : '';
  $: title = partnerId ? displayName(partnerId) : 'Messages';

  $: if (open && !wasOpen) {
    wasOpen = true;
    refreshAll();
  } else if (!open && wasOpen) {
    wasOpen = false;
    error = '';
    draft = '';
    newTarget = '';
  }

  $: if (open) {
    if (!escHandler) {
      escHandler = (e) => {
        if (e.key === 'Escape') {
          e.preventDefault();
          close();
        }
      };
      if (typeof window !== 'undefined') window.addEventListener('keydown', escHandler);
    }
    if (!pollTimer && typeof window !== 'undefined') {
      pollTimer = setInterval(() => {
        refreshAll();
      }, POLL_MS);
    }
  } else {
    if (escHandler) {
      if (typeof window !== 'undefined') window.removeEventListener('keydown', escHandler);
      escHandler = null;
    }
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  afterUpdate(() => {
    if (open && pinnedToBottom) scrollToBottom();
  });

  onDestroy(() => {
    if (escHandler && typeof window !== 'undefined') window.removeEventListener('keydown', escHandler);
    if (pollTimer) clearInterval(pollTimer);
  });

  function buildRoomNames(players) {
    const map = {};
    for (const player of players || []) {
      if (player && player.id) map[String(player.id)] = player.name || String(player.id);
    }
    return map;
  }

  function shortId(id) {
    const value = String(id || '');
    if (!value) return 'Unknown';
    return value.length > 10 ? `${value.slice(0, 8)}\u2026` : value;
  }

  function displayName(id) {
    return roomNames[String(id || '')] || shortId(id);
  }

  function partnerOf(channel, selfId) {
    if (!channel) return '';
    const a = String(channel.participantA || '');
    const b = String(channel.participantB || '');
    if (a === selfId) return b;
    if (b === selfId) return a;
    return a || b;
  }

  function unreadFor(channel) {
    if (!channel) return 0;
    if (String(channel.participantA || '') === myId) return Number(channel.unreadA || 0);
    if (String(channel.participantB || '') === myId) return Number(channel.unreadB || 0);
    return 0;
  }

  function formatTime(value) {
    if (!value) return '';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '';
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  function close() {
    dmsPanelOpen.set(false);
  }

  function toast(message) {
    if (overlayStore && overlayStore.pushMessage) overlayStore.pushMessage(message);
  }

  function fail(err) {
    const message = (err && err.message) || 'Something went wrong.';
    error = message;
    toast(message);
  }

  async function refreshAll() {
    if (!authToken || !myId) {
      error = 'Select a character before sending messages.';
      return;
    }
    if (busy || loading) return;
    loading = true;
    error = '';
    try {
      await loadChannels();
      if (activeChannelId) {
        await loadMessages(activeChannelId, true);
      } else {
        messages = [];
      }
    } catch (err) {
      fail(err);
    } finally {
      loading = false;
    }
  }

  async function loadChannels() {
    const list = await getDMChannels(authToken, myId);
    channels = Array.isArray(list) ? list : [];
    if (!channels.some((c) => c && c.id === activeChannelId)) {
      activeChannelId = channels[0] ? channels[0].id : '';
    }
  }

  async function loadMessages(channelId, mark) {
    if (!authToken || !myId || !channelId) return;
    const list = await getDMMessages(authToken, channelId, { characterId: myId, limit: 100 });
    if (channelId !== activeChannelId) return; // ignore stale responses
    messages = Array.isArray(list) ? list : [];
    pinnedToBottom = true;
    if (mark) markRead(channelId);
    await scrollToBottom();
  }

  function markRead(channelId) {
    if (!authToken || !myId || !channelId) return;
    markDMChannelRead(authToken, channelId, myId).catch(() => {});
  }

  async function selectChannel(channel) {
    if (!channel || channel.id === activeChannelId) return;
    activeChannelId = channel.id;
    messages = [];
    error = '';
    try {
      await loadMessages(channel.id, true);
    } catch (err) {
      fail(err);
    }
  }

  async function send() {
    const body = String(draft || '').trim();
    if (!body || !activeChannelId) return;
    busy = true;
    error = '';
    try {
      const message = await sendDMMessage(authToken, activeChannelId, { characterId: myId, body });
      if (message) messages = [...messages, message];
      draft = '';
      pinnedToBottom = true;
      await scrollToBottom();
      await loadChannels();
    } catch (err) {
      fail(err);
    } finally {
      busy = false;
    }
  }

  async function startNew() {
    const target = String(newTarget || '').trim();
    if (!target) {
      error = 'Enter the character id to message.';
      return;
    }
    busy = true;
    error = '';
    try {
      const channel = await createDMChannel(authToken, {
        characterId: myId,
        targetCharacterId: target,
      });
      newTarget = '';
      await loadChannels();
      if (channel && channel.id) {
        activeChannelId = channel.id;
        await loadMessages(channel.id, true);
      }
    } catch (err) {
      fail(err);
    } finally {
      busy = false;
    }
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
</script>

<style>
  .dms-overlay {
    position: fixed;
    inset: 0;
    z-index: 130;
    background: rgba(0, 0, 0, 0.82);
    backdrop-filter: blur(6px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1em;
    padding-top: max(1em, env(safe-area-inset-top, 0px));
    padding-bottom: max(1em, env(safe-area-inset-bottom, 0px));
    box-sizing: border-box;
  }
  .dms-panel {
    width: min(620px, 100%);
    height: min(84vh, 680px);
    display: flex;
    flex-direction: column;
    background: var(--social-panel-bg, rgba(12, 16, 24, 0.97));
    border: 1px solid var(--social-border, rgba(212, 175, 55, 0.28));
    border-radius: 10px;
    overflow: hidden;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.55);
  }
  .dms-header {
    display: flex;
    align-items: center;
    gap: 0.75em;
    padding: 0.75em 1em;
    border-bottom: 1px solid rgba(148, 163, 184, 0.2);
    background: var(--social-header-bg, rgba(20, 26, 36, 0.9));
    flex-shrink: 0;
  }
  .dms-title {
    flex: 1;
    min-width: 0;
    font-weight: 700;
    color: #f8fafc;
    display: flex;
    align-items: center;
    gap: 0.4em;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    font-size: 13px;
  }
  .dms-title i { color: #93c5fd; font-size: 1.2em; }
  .dms-body { flex: 1; min-height: 0; display: flex; }
  .channel-list {
    width: 190px;
    flex-shrink: 0;
    border-right: 1px solid rgba(148, 163, 184, 0.15);
    overflow-y: auto;
    background: rgba(8, 10, 15, 0.6);
  }
  .channel {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    box-sizing: border-box;
    border: 0;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
    background: transparent;
    color: #e5e7eb;
    text-align: left;
    padding: 10px 10px;
    cursor: pointer;
    font-size: 13px;
  }
  .channel:hover { background: rgba(148, 163, 184, 0.08); }
  .channel.active { background: rgba(147, 197, 253, 0.12); color: #bfdbfe; }
  .channel-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .unread-badge {
    min-width: 17px;
    padding: 0 5px;
    border-radius: 9px;
    background: #fbbf24;
    color: #1a1408;
    font-size: 10px;
    font-weight: 700;
    line-height: 17px;
    text-align: center;
  }
  .channels-empty { color: #8a8070; font-size: 12px; padding: 1em 0.75em; line-height: 1.4; }
  .thread { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .thread-head {
    flex-shrink: 0;
    padding: 8px 12px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.15);
    color: #f3ead4;
    font-weight: 700;
    font-size: 13px;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .thread-head .muted { color: #9ca3af; font-weight: 400; font-size: 11px; }
  .message-list {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0.6em 0.75em;
    display: flex;
    flex-direction: column;
    gap: 0.5em;
  }
  .msg { display: flex; flex-direction: column; gap: 2px; max-width: 92%; }
  .msg.mine { align-self: flex-end; align-items: flex-end; }
  .msg-who { font-size: 11px; font-weight: 700; color: #93c5fd; }
  .msg-who.mine { color: #fbbf24; }
  .msg-bubble {
    background: rgba(148, 163, 184, 0.12);
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-radius: 10px;
    padding: 6px 10px;
    color: #e8dcc8;
    font-size: 13.5px;
    line-height: 1.4;
    word-break: break-word;
  }
  .msg.mine .msg-bubble {
    background: rgba(251, 191, 36, 0.12);
    border-color: rgba(251, 191, 36, 0.3);
  }
  .msg-time { font-size: 10px; color: #6b7280; }
  .empty {
    margin: auto;
    color: #8a8070;
    font-size: 13px;
    text-align: center;
    padding: 1.5em 1em;
    line-height: 1.45;
  }
  .composer {
    flex-shrink: 0;
    border-top: 1px solid rgba(148, 163, 184, 0.15);
    background: rgba(10, 12, 18, 0.92);
    padding: 10px 12px 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .compose { display: flex; gap: 6px; }
  .compose input, .finder input {
    flex: 1;
    min-width: 0;
    background: rgba(0, 0, 0, 0.35);
    border: 1px solid rgba(148, 163, 184, 0.35);
    color: #e5e7eb;
    border-radius: 6px;
    padding: 8px 10px;
    font: inherit;
    font-size: 14px;
  }
  .finder { display: flex; gap: 6px; }
  .act {
    border: 1px solid rgba(148, 163, 184, 0.3);
    background: rgba(255, 255, 255, 0.05);
    color: #e5e7eb;
    border-radius: 6px;
    min-height: 40px;
    min-width: 40px;
    padding: 0 12px;
    cursor: pointer;
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .act:disabled { opacity: 0.5; cursor: not-allowed; }
  .act.gold { background: var(--social-gold-fill, #c9a227); border-color: var(--social-gold, #d4af37); color: var(--social-ink, #1a1408); }
  .act.secondary { background: transparent; border-color: rgba(212, 175, 55, 0.55); color: #fbbf24; }
  .error-banner {
    margin: 8px 12px 0;
    padding: 0.5em 0.75em;
    border-radius: 6px;
    background: rgba(248, 113, 113, 0.12);
    border: 1px solid rgba(248, 113, 113, 0.4);
    color: #fca5a5;
    font-size: 12.5px;
  }
  @media (max-width: 768px) {
    .dms-overlay { align-items: flex-end; padding: 0; padding-bottom: env(safe-area-inset-bottom, 0px); }
    .dms-panel { width: 100%; height: min(94dvh, 760px); border-radius: 14px 14px 0 0; }
    .channel-list { width: 130px; }
    .act { min-height: 44px; min-width: 44px; }
    .compose input, .finder input { min-height: 44px; }
  }
</style>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="dms-overlay" role="dialog" aria-modal="true" aria-label="Direct messages" on:click={(e) => { if (e.target === e.currentTarget) close(); }}>
    <div class="dms-panel" on:click|stopPropagation>
      <div class="dms-header">
        <div class="dms-title"><i class="material-icons" aria-hidden="true">mail</i> {title}</div>
        <button class="act secondary" type="button" disabled={busy || loading} on:click={refreshAll}>Refresh</button>
        <button class="dms-close act" type="button" on:click={close} aria-label="Close direct messages">×</button>
      </div>

      {#if error}
        <div class="error-banner" role="alert">{error}</div>
      {/if}

      <div class="dms-body">
        <div class="channel-list" aria-label="Conversations">
          {#if channels.length === 0}
            <div class="channels-empty">No conversations yet. Start one below.</div>
          {:else}
            {#each channels as channel (channel.id)}
              {@const unread = unreadFor(channel)}
              <button
                class="channel"
                class:active={channel.id === activeChannelId}
                type="button"
                on:click={() => selectChannel(channel)}
              >
                <span class="channel-name">{displayName(partnerOf(channel, myId))}</span>
                {#if unread > 0}<span class="unread-badge">{unread}</span>{/if}
              </button>
            {/each}
          {/if}
        </div>

        <div class="thread">
          {#if activeChannel}
            <div class="thread-head">
              {displayName(partnerId)}
              <span class="muted">#{shortId(activeChannel.id)}</span>
            </div>
            <div class="message-list" bind:this={listEl} on:scroll={onScroll} aria-live="polite">
              {#if messages.length === 0}
                <div class="empty">No messages yet. Say hello.</div>
              {:else}
                {#each messages as message (message.id)}
                  {@const mine = String(message.fromCharID || '') === myId}
                  <div class="msg" class:mine={mine}>
                    <span class="msg-who" class:mine={mine}>{mine ? 'You' : displayName(message.fromCharID)}</span>
                    <span class="msg-bubble">{message.body}</span>
                    <span class="msg-time">{formatTime(message.sentAt)}</span>
                  </div>
                {/each}
              {/if}
            </div>
            <form class="composer" on:submit|preventDefault={send}>
              <div class="compose">
                <input bind:value={draft} placeholder="Message {displayName(partnerId)}…" aria-label="Message" maxlength="1000" />
                <button class="act gold" type="submit" disabled={busy || !draft.trim()}>Send</button>
              </div>
            </form>
          {:else}
            <div class="empty">Select a conversation, or start a new one below.</div>
          {/if}
        </div>
      </div>

      <form class="composer" on:submit|preventDefault={startNew}>
        <div class="finder">
          <input bind:value={newTarget} placeholder="New message — character id" aria-label="New message character id" maxlength="64" />
          <button class="act secondary" type="submit" disabled={busy}>New</button>
        </div>
      </form>
    </div>
  </div>
{/if}
