<script>
  import { onDestroy } from 'svelte';
  import { clanPanelOpen } from '../uiChrome.js';
  import { overlayStore } from './overlayStore.js';
  import {
    createClan,
    getMyClan,
    inviteToClan,
    acceptClanInvite,
    declineClanInvite,
    kickFromClan,
    changeClanRank,
    depositToClan,
    withdrawFromClan,
    disbandClan,
  } from '../../api/clans.js';

  export let store = null;
  export let authToken = '';

  const RANK_ORDER = ['member', 'officer', 'leader'];

  let wasOpen = false;
  let escHandler = null;

  let loading = false;
  let busy = false;
  let error = '';
  let data = { clan: null, members: [], invites: [] };

  let createName = '';
  let createTag = '';
  let createDescription = '';
  let inviteTarget = '';
  let bankAmount = '';

  $: open = $clanPanelOpen;
  $: me = (store && $store && $store.character) || null;
  $: myId = me && me.id ? String(me.id) : '';
  $: clan = data && data.clan ? data.clan : null;
  $: members = data && Array.isArray(data.members) ? data.members : [];
  $: invites = data && Array.isArray(data.invites) ? data.invites : [];
  $: myMember = members.find((m) => String((m && m.characterID) || '') === myId) || null;
  $: myRank = rankName(myMember && myMember.rank);
  $: iAmLeader = myRank === 'leader';
  $: canInvite = iAmLeader || myRank === 'officer';
  $: canKick = canInvite;
  $: memberCount = members.length;
  $: maxMembers = Number(clan && clan.maxMembers) > 0 ? Number(clan.maxMembers) : 0;
  $: bank = clan ? Number(clan.bank || 0) : 0;

  // Room roster gives us id → name for the players we can see; outside the room
  // the API only exposes character ids, so we fall back to a short id.
  $: roomNames = buildRoomNames(store && $store ? $store.players : []);

  $: if (open && !wasOpen) {
    wasOpen = true;
    load();
  } else if (!open && wasOpen) {
    wasOpen = false;
    error = '';
    inviteTarget = '';
    bankAmount = '';
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
  } else if (escHandler) {
    if (typeof window !== 'undefined') window.removeEventListener('keydown', escHandler);
    escHandler = null;
  }

  onDestroy(() => {
    if (escHandler && typeof window !== 'undefined') window.removeEventListener('keydown', escHandler);
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

  function rankName(rank) {
    if (typeof rank === 'number') return RANK_ORDER[rank - 1] || 'member';
    const name = String(rank || '').toLowerCase();
    return RANK_ORDER.includes(name) ? name : 'member';
  }

  function rankValue(rank) {
    return RANK_ORDER.indexOf(rankName(rank)) + 1;
  }

  function nextRank(rank) {
    const index = rankValue(rank);
    return index < RANK_ORDER.length ? RANK_ORDER[index] : null;
  }

  function prevRank(rank) {
    const index = rankValue(rank);
    return index > 1 ? RANK_ORDER[index - 2] : null;
  }

  function isYou(member) {
    return Boolean(myId && member && String(member.characterID || '') === myId);
  }

  // Only the leader can change ranks; officers may invite/kick regular members.
  function canActOn(member) {
    if (!canKick || !member || isYou(member)) return false;
    const target = rankValue(member.rank);
    if (iAmLeader) return target < RANK_ORDER.length;
    return target === rankValue('member');
  }

  function close() {
    clanPanelOpen.set(false);
  }

  function toast(message) {
    if (overlayStore && overlayStore.pushMessage) overlayStore.pushMessage(message);
  }

  function fail(err) {
    const message = (err && err.message) || 'Something went wrong.';
    error = message;
    toast(message);
  }

  function normalizeMine(res) {
    const raw = res && typeof res === 'object' ? res : {};
    const pending = Array.isArray(raw.invites)
      ? raw.invites
      : Array.isArray(raw.pendingInvites)
        ? raw.pendingInvites
        : [];
    return {
      clan: raw.clan || null,
      members: Array.isArray(raw.members) ? raw.members : [],
      invites: pending,
    };
  }

  async function load() {
    if (!authToken || !myId) {
      error = 'Select a character before managing a clan.';
      return;
    }
    loading = true;
    error = '';
    try {
      const res = await getMyClan(authToken, myId);
      data = normalizeMine(res);
    } catch (err) {
      fail(err);
    } finally {
      loading = false;
    }
  }

  async function run(action, successMessage) {
    if (busy) return false;
    busy = true;
    error = '';
    try {
      await action();
      if (successMessage) toast(successMessage);
      await load();
      return true;
    } catch (err) {
      fail(err);
      return false;
    } finally {
      busy = false;
    }
  }

  function submitCreate() {
    const name = String(createName || '').trim();
    const tag = String(createTag || '').trim();
    if (!name || !tag) {
      error = 'Clan name and tag are required.';
      return;
    }
    run(
      () =>
        createClan(authToken, {
          characterId: myId,
          name,
          tag,
          description: String(createDescription || '').trim(),
        }),
      'Clan created.'
    ).then((ok) => {
      if (ok) {
        createName = '';
        createTag = '';
        createDescription = '';
      }
    });
  }

  function acceptInvite(invite) {
    if (!invite || !invite.id) return;
    run(() => acceptClanInvite(authToken, invite.id, myId), 'Invite accepted.');
  }

  function declineInvite(invite) {
    if (!invite || !invite.id) return;
    run(() => declineClanInvite(authToken, invite.id, myId), 'Invite declined.');
  }

  function submitInvite() {
    const target = String(inviteTarget || '').trim();
    if (!clan || !target) {
      error = 'Enter the character id to invite.';
      return;
    }
    run(
      () => inviteToClan(authToken, clan.id, { characterId: myId, targetCharacterId: target }),
      'Invite sent.'
    ).then((ok) => {
      if (ok) inviteTarget = '';
    });
  }

  function kickMember(member) {
    if (!clan || !member) return;
    const label = displayName(member.characterID);
    if (typeof window !== 'undefined' && !window.confirm(`Kick ${label} from the clan?`)) return;
    run(
      () => kickFromClan(authToken, clan.id, { characterId: myId, targetCharacterId: member.characterID }),
      'Member kicked.'
    );
  }

  function changeRank(member, rank) {
    if (!clan || !member || !rank) return;
    run(
      () =>
        changeClanRank(authToken, clan.id, {
          characterId: myId,
          targetCharacterId: member.characterID,
          rank,
        }),
      'Rank updated.'
    );
  }

  function submitDeposit() {
    if (!clan) return;
    const amount = Math.floor(Number(bankAmount));
    if (!Number.isFinite(amount) || amount <= 0) {
      error = 'Enter a positive amount.';
      return;
    }
    run(() => depositToClan(authToken, clan.id, { characterId: myId, amount }), 'Deposited.').then((ok) => {
      if (ok) bankAmount = '';
    });
  }

  function submitWithdraw() {
    if (!clan) return;
    const amount = Math.floor(Number(bankAmount));
    if (!Number.isFinite(amount) || amount <= 0) {
      error = 'Enter a positive amount.';
      return;
    }
    run(() => withdrawFromClan(authToken, clan.id, { characterId: myId, amount }), 'Withdrew.').then((ok) => {
      if (ok) bankAmount = '';
    });
  }

  function submitDisband() {
    if (!clan) return;
    if (typeof window !== 'undefined' && !window.confirm('Disband the clan for everyone?')) return;
    run(() => disbandClan(authToken, clan.id, myId), 'Clan disbanded.');
  }
</script>

<style>
  .clan-overlay {
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
  .clan-panel {
    width: min(560px, 100%);
    max-height: min(84vh, 700px);
    display: flex;
    flex-direction: column;
    background: var(--social-panel-bg, rgba(12, 16, 24, 0.97));
    border: 1px solid var(--social-border, rgba(212, 175, 55, 0.28));
    border-radius: 10px;
    overflow: hidden;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.55);
  }
  .clan-header {
    display: flex;
    align-items: flex-start;
    gap: 0.75em;
    padding: 0.75em 1em;
    border-bottom: 1px solid rgba(148, 163, 184, 0.2);
    background: var(--social-header-bg, rgba(20, 26, 36, 0.9));
    flex-shrink: 0;
  }
  .clan-heading { flex: 1; min-width: 0; }
  .clan-title {
    font-weight: 700;
    color: #f8fafc;
    display: flex;
    align-items: center;
    gap: 0.4em;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    font-size: 13px;
  }
  .clan-title i { color: #c4b5fd; font-size: 1.2em; }
  .clan-tag {
    font-size: 11px;
    color: #fbbf24;
    border: 1px solid rgba(251, 191, 36, 0.4);
    border-radius: 999px;
    padding: 1px 7px;
    letter-spacing: 0.08em;
  }
  .clan-subtitle {
    margin-top: 4px;
    color: var(--social-muted, #8a8070);
    font-size: 11px;
    letter-spacing: 0.03em;
  }
  .clan-close {
    border: none;
    background: transparent;
    color: #94a3b8;
    font-size: 1.4em;
    cursor: pointer;
    line-height: 1;
    min-width: 44px;
    min-height: 44px;
  }
  .clan-body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 0.5em 0.75em 0.75em;
  }
  .empty {
    color: var(--social-muted, #8a8070);
    font-size: 13px;
    padding: 1.2em 0.4em 0.6em;
    text-align: center;
    line-height: 1.45;
  }
  .section-label {
    margin: 0.6em 0 0.3em;
    color: #fbbf24;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 6px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }
  .who {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .who-name { color: var(--social-text, #f3ead4); font-weight: 700; font-size: 14px; }
  .you-tag {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: #1a1408;
    background: #fbbf24;
    border-radius: 999px;
    padding: 1px 6px;
    font-weight: 700;
    margin-left: 6px;
  }
  .who-meta { color: #9ca3af; font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .rank-badge {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    border-radius: 999px;
    padding: 3px 9px;
    font-weight: 700;
    flex-shrink: 0;
    background: rgba(107, 114, 128, 0.25);
    color: #9ca3af;
  }
  .rank-badge.officer { background: rgba(96, 165, 250, 0.15); color: #93c5fd; }
  .rank-badge.leader { background: rgba(251, 191, 36, 0.18); color: #fbbf24; }
  .actions { display: flex; gap: 6px; flex-shrink: 0; flex-wrap: wrap; justify-content: flex-end; }
  .act {
    border: 1px solid rgba(148, 163, 184, 0.3);
    background: rgba(255, 255, 255, 0.05);
    color: #e5e7eb;
    border-radius: 6px;
    min-height: 36px;
    min-width: 36px;
    padding: 0 10px;
    cursor: pointer;
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .act:disabled { opacity: 0.5; cursor: not-allowed; }
  .act.gold {
    background: var(--social-gold-fill, #c9a227);
    border-color: var(--social-gold, #d4af37);
    color: var(--social-ink, #1a1408);
  }
  .act.secondary { background: transparent; border-color: rgba(212, 175, 55, 0.55); color: #fbbf24; }
  .act.accept { background: rgba(34, 197, 94, 0.18); border-color: rgba(34, 197, 94, 0.45); color: #86efac; }
  .act.decline { border-color: rgba(248, 113, 113, 0.4); color: #fca5a5; }
  .act.danger { border-color: rgba(248, 113, 113, 0.4); color: #fca5a5; background: transparent; }
  .action-bar {
    flex-shrink: 0;
    border-top: 1px solid rgba(148, 163, 184, 0.15);
    background: rgba(10, 12, 18, 0.92);
    padding: 10px 12px 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 46%;
    overflow: auto;
  }
  .adder, .bank-row {
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .adder input, .bank-row input {
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
  .create-form { display: flex; flex-direction: column; gap: 8px; }
  .create-fields { display: flex; gap: 6px; }
  .create-fields input { flex: 1; min-width: 0; }
  .create-fields .tag-input { flex: 0 0 90px; }
  .error-banner {
    margin: 0 0 0.5em;
    padding: 0.5em 0.75em;
    border-radius: 6px;
    background: rgba(248, 113, 113, 0.12);
    border: 1px solid rgba(248, 113, 113, 0.4);
    color: #fca5a5;
    font-size: 12.5px;
  }
  .muted { color: #9ca3af; font-size: 11px; }
  @media (max-width: 768px) {
    .clan-overlay { align-items: flex-end; padding: 0; padding-bottom: env(safe-area-inset-bottom, 0px); }
    .clan-panel { width: 100%; max-height: min(94dvh, 760px); border-radius: 14px 14px 0 0; }
    .act { min-height: 44px; min-width: 44px; }
    .adder input, .bank-row input { min-height: 44px; }
    .actions { width: 100%; }
  }
</style>

{#if open}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="clan-overlay" role="dialog" aria-modal="true" aria-label="Clan" on:click={(e) => { if (e.target === e.currentTarget) close(); }}>
    <div class="clan-panel" on:click|stopPropagation>
      <div class="clan-header">
        <div class="clan-heading">
          <div class="clan-title">
            <i class="material-icons" aria-hidden="true">shield</i>
            {clan ? clan.name : 'Clan'}
            {#if clan && clan.tag}<span class="clan-tag">{clan.tag}</span>{/if}
          </div>
          <div class="clan-subtitle">
            {#if clan}
              {memberCount}{maxMembers ? `/${maxMembers}` : ''} members · Bank {bank}g
            {:else if loading}
              Loading…
            {:else}
              No clan yet
            {/if}
          </div>
        </div>
        <button class="clan-close" type="button" on:click={close} aria-label="Close clan panel">×</button>
      </div>

      <div class="clan-body">
        {#if error}
          <div class="error-banner" role="alert">{error}</div>
        {/if}

        {#if loading && !clan && invites.length === 0}
          <div class="empty">Loading clan…</div>
        {:else if clan}
          {#if members.length === 0}
            <div class="empty">No members listed.</div>
          {:else}
            {#each members as member (member.id || member.characterID)}
              <div class="row">
                <div class="who">
                  <span class="who-name">
                    {displayName(member.characterID)}{#if isYou(member)}<span class="you-tag">You</span>{/if}
                  </span>
                  <span class="who-meta">contribution {Number(member.contribution || 0)}</span>
                </div>
                <span class="rank-badge" class:officer={rankName(member.rank) === 'officer'} class:leader={rankName(member.rank) === 'leader'}>{rankName(member.rank)}</span>
                {#if canActOn(member)}
                  <div class="actions">
                    {#if iAmLeader && nextRank(member.rank)}
                      <button class="act secondary" type="button" disabled={busy} on:click={() => changeRank(member, nextRank(member.rank))}>Promote</button>
                    {/if}
                    {#if iAmLeader && prevRank(member.rank)}
                      <button class="act secondary" type="button" disabled={busy} on:click={() => changeRank(member, prevRank(member.rank))}>Demote</button>
                    {/if}
                    {#if canKick}
                      <button class="act danger" type="button" disabled={busy} on:click={() => kickMember(member)}>Kick</button>
                    {/if}
                  </div>
                {/if}
              </div>
            {/each}
          {/if}
        {:else}
          {#if invites.length}
            <div class="section-label">Pending invitations</div>
            {#each invites as invite (invite.id)}
              <div class="row">
                <div class="who">
                  <span class="who-name">Clan invite</span>
                  <span class="who-meta">from {displayName(invite.inviterID)} · clan {shortId(invite.clanID)}</span>
                </div>
                <div class="actions">
                  <button class="act accept" type="button" disabled={busy} on:click={() => acceptInvite(invite)}>Accept</button>
                  <button class="act decline" type="button" disabled={busy} on:click={() => declineInvite(invite)}>Decline</button>
                </div>
              </div>
            {/each}
          {:else}
            <div class="empty">
              You are not in a clan.
              <span class="muted">Found one below, or wait for an invitation.</span>
            </div>
          {/if}
        {/if}
      </div>

      {#if clan}
        <div class="action-bar">
          {#if canInvite}
            <form class="adder" on:submit|preventDefault={submitInvite}>
              <input
                bind:value={inviteTarget}
                placeholder="Invite by character id"
                aria-label="Invite character by id"
                maxlength="64"
              />
              <button class="act secondary" type="submit" disabled={busy}>Invite</button>
            </form>
          {/if}
          <form class="bank-row" on:submit|preventDefault={submitDeposit}>
            <input
              bind:value={bankAmount}
              type="number"
              min="1"
              step="1"
              placeholder="Gold amount"
              aria-label="Clan bank amount"
            />
            <button class="act gold" type="submit" disabled={busy}>Deposit</button>
            {#if iAmLeader}
              <button class="act secondary" type="button" disabled={busy} on:click={submitWithdraw}>Withdraw</button>
            {/if}
          </form>
          <div class="bank-row">
            <button class="act secondary" type="button" disabled={busy || loading} on:click={load}>Refresh</button>
            {#if iAmLeader}
              <button class="act danger" type="button" disabled={busy} on:click={submitDisband}>Disband</button>
            {/if}
          </div>
        </div>
      {:else}
        <div class="action-bar">
          <form class="create-form" on:submit|preventDefault={submitCreate}>
            <div class="create-fields">
              <input bind:value={createName} placeholder="Clan name" aria-label="Clan name" maxlength="40" />
              <input class="tag-input" bind:value={createTag} placeholder="Tag" aria-label="Clan tag" maxlength="5" />
            </div>
            <div class="create-fields">
              <input bind:value={createDescription} placeholder="Description (optional)" aria-label="Clan description" maxlength="160" />
              <button class="act gold" type="submit" disabled={busy}>Create</button>
            </div>
          </form>
          <div class="bank-row">
            <button class="act secondary" type="button" disabled={busy || loading} on:click={load}>Refresh</button>
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}
