<script>
  import { settingsStore } from '../SettingsStore.js';
  import { overlayStore } from './overlayStore.js';
  import { spellbookOpen } from '../uiChrome.js';
  import { onItemArtError } from '../itemArtSrc.js';
  import {
    bindSkillToFirstEmptyHotbar,
    characterClassId,
    formatSkillCost,
    formatSkillEffects,
    maxSkillSlots,
    normalizeHotbarBinds,
    skillGenericArtUrl,
  } from '../hudPrefs.js';
  import { buildSpellbookRows } from '../spellbook.js';

  export let store = null;
  export let sendMessage = null;

  let stats = {};
  let character = null;

  $: if (store) {
    stats = $store.characterStats || {};
    character = $store.character || null;
  }

  $: classId = characterClassId(character);
  $: level = Number(stats.level || character?.level || 0);
  $: equippedSkills = Array.isArray(stats.equippedSkills)
    ? stats.equippedSkills
    : (Array.isArray(character?.equippedSkills) ? character.equippedSkills : []);
  $: inCombat = !!(stats.inCombat || (store && $store.inCombat));
  $: hotbarBinds = normalizeHotbarBinds($settingsStore.interface?.hotbarBinds);
  $: computedMaxSlots = Number(stats.maxSkillSlots) > 0
    ? Number(stats.maxSkillSlots)
    : maxSkillSlots(classId, level);
  $: rows = buildSpellbookRows({ classId, level, equippedIds: equippedSkills, hotbarBinds });
  $: equippedCount = rows.equipped.length;
  $: slotsFull = equippedCount >= computedMaxSlots;

  function skillName(skill) {
    return skill?.name || skill?.id || 'Skill';
  }

  function close() {
    spellbookOpen.set(false);
  }

  function toast(text) {
    if (overlayStore?.pushMessage) overlayStore.pushMessage(text);
  }

  function equipSkill(skill) {
    if (!sendMessage || !skill) return;
    if (inCombat) {
      toast('You cannot change skills during combat.');
      return;
    }
    if (slotsFull) {
      toast(`All ${computedMaxSlots} skill slots are full. Unequip a skill first.`);
      return;
    }
    sendMessage('skills equip ' + skillName(skill));
  }

  function unequipSkill(skill) {
    if (!sendMessage || !skill) return;
    if (inCombat) {
      toast('You cannot change skills during combat.');
      return;
    }
    sendMessage('skills unequip ' + skillName(skill));
  }

  function addToHotbar(skill) {
    if (!skill) return;
    const result = bindSkillToFirstEmptyHotbar(hotbarBinds, skill.id || skill.name);
    if (result.status === 'already') {
      toast(`${skillName(skill)} is already on the hotbar.`);
      return;
    }
    if (result.status === 'full') {
      toast('Hotbar is full — right-click a slot to replace it.');
      return;
    }
    if (result.status !== 'bound') {
      toast('Could not bind that skill.');
      return;
    }
    settingsStore.setSetting('interface', 'hotbarBinds', result.binds);
    toast(`${skillName(skill)} bound to hotbar slot ${result.index + 1}.`);
  }
</script>

<style>
  .spellbook-overlay {
    position: fixed;
    inset: 0;
    z-index: 12050;
    background: rgba(0, 0, 0, 0.72);
    backdrop-filter: blur(6px);
    -webkit-backdrop-filter: blur(6px);
    display: flex;
    flex-direction: column;
    padding: 1em;
    overflow: hidden;
    animation: spellbookFadeIn 0.15s ease-out;
  }

  @keyframes spellbookFadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .spellbook-panel {
    flex: 1;
    min-height: 0;
    max-width: 640px;
    width: 100%;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    background: rgba(12, 16, 24, 0.97);
    border: 1px solid rgba(212, 175, 55, 0.35);
    border-radius: 10px;
    overflow: hidden;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.55);
  }

  .spellbook-header {
    display: flex;
    align-items: center;
    gap: 0.75em;
    padding: 0.75em 1em;
    border-bottom: 1px solid rgba(148, 163, 184, 0.2);
    background: rgba(20, 26, 36, 0.9);
    flex-shrink: 0;
  }

  .spellbook-title {
    flex: 1;
    font-weight: 700;
    color: #f8fafc;
    letter-spacing: 0.02em;
    display: flex;
    align-items: center;
    gap: 0.4em;
  }

  .spellbook-title i { color: #c084fc; font-size: 1.2em; }

  .spellbook-close {
    border: none;
    background: transparent;
    color: #94a3b8;
    font-size: 1.4em;
    cursor: pointer;
    line-height: 1;
    padding: 0.15em 0.3em;
  }

  .spellbook-close:hover { color: #e2e8f0; }

  .spellbook-hint {
    padding: 0.5em 1em 0;
    color: #94a3b8;
    font-size: 0.8em;
    display: flex;
    justify-content: space-between;
    gap: 0.75em;
  }

  .spellbook-hint .lock { color: #fca5a5; }

  .spellbook-list {
    flex: 1;
    min-height: 0;
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 0.45em;
    padding: 0.7em 0.85em 1em;
  }

  .section-label {
    font-size: 0.72em;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: #64748b;
    font-weight: 700;
    margin: 0.5em 0 0.1em;
  }

  .spell-row {
    display: grid;
    grid-template-columns: 52px 1fr;
    gap: 0.65em;
    align-items: start;
    padding: 0.55em 0.65em;
    border-radius: 8px;
    border: 1px solid rgba(148, 163, 184, 0.18);
    background: rgba(255, 255, 255, 0.03);
  }

  .spell-row.locked { opacity: 0.62; }

  .spell-icon {
    width: 52px;
    height: 52px;
    border-radius: 6px;
    background: #0b1119;
    border: 1px solid rgba(148, 163, 184, 0.25);
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .spell-icon img {
    width: 44px;
    height: 44px;
    object-fit: contain;
    image-rendering: pixelated;
  }

  .spell-body { min-width: 0; display: flex; flex-direction: column; gap: 0.3em; }

  .spell-name {
    font-weight: 700;
    font-size: 0.92em;
    color: #f8fafc;
  }

  .spell-desc {
    font-size: 0.78em;
    color: #94a3b8;
    line-height: 1.35;
  }

  .spell-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3em;
  }

  .chip {
    border-radius: 999px;
    padding: 0.12em 0.5em;
    border: 1px solid rgba(148, 163, 184, 0.28);
    color: #cbd5e1;
    background: rgba(255, 255, 255, 0.04);
    font-size: 0.72em;
  }

  .chip.cost { color: #93c5fd; border-color: rgba(59, 130, 246, 0.3); background: rgba(59, 130, 246, 0.12); }
  .chip.slot { color: #e8dcc8; border-color: rgba(212, 164, 74, 0.35); background: rgba(212, 164, 74, 0.12); }
  .chip.hotbar { color: #86efac; border-color: rgba(34, 197, 94, 0.35); background: rgba(34, 197, 94, 0.12); }
  .chip.lock { color: #fcd34d; border-color: rgba(245, 158, 11, 0.4); background: rgba(245, 158, 11, 0.12); }

  .spell-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35em;
    margin-top: 0.1em;
  }

  .spell-btn {
    min-height: 34px;
    padding: 0.35em 0.7em;
    border-radius: 6px;
    border: 1px solid rgba(212, 164, 74, 0.4);
    background: rgba(212, 164, 74, 0.14);
    color: #f5d78c;
    font: inherit;
    font-size: 0.78em;
    font-weight: 700;
    cursor: pointer;
  }

  .spell-btn:hover:not(:disabled) { background: rgba(212, 164, 74, 0.26); }

  .spell-btn.ghost {
    background: transparent;
    color: #cbd5e1;
    border-color: rgba(148, 163, 184, 0.3);
  }

  .spell-btn.danger {
    color: #fca5a5;
    border-color: rgba(239, 68, 68, 0.4);
    background: rgba(239, 68, 68, 0.12);
  }

  .spell-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .spellbook-empty {
    text-align: center;
    color: #94a3b8;
    padding: 2em 1em;
  }

  @media screen and (max-width: 520px) {
    .spellbook-panel { max-width: none; }
  }
</style>

{#if $spellbookOpen}
  <div class="spellbook-overlay" role="dialog" aria-label="Spellbook">
    <div class="spellbook-panel">
      <div class="spellbook-header">
        <div class="spellbook-title">
          <i class="material-icons">auto_awesome</i>
          Spellbook
        </div>
        <button class="spellbook-close" type="button" on:click={close} aria-label="Close spellbook">×</button>
      </div>

      <div class="spellbook-hint">
        <span>Equipped {equippedCount}/{computedMaxSlots}</span>
        {#if inCombat}
          <span class="lock">Skill slots are locked during combat.</span>
        {:else}
          <span>Equip into a slot, then bind it to the hotbar (1–{hotbarBinds.length}).</span>
        {/if}
      </div>

      <div class="spellbook-list">
        {#if equippedCount === 0 && rows.available.length === 0 && rows.locked.length === 0}
          <div class="spellbook-empty">No skills known for this character yet.</div>
        {/if}

        {#if rows.equipped.length > 0}
          <div class="section-label">Equipped</div>
          {#each rows.equipped as skill (skill.id || skill.name)}
            <div class="spell-row">
              <div class="spell-icon">
                <img src={skillGenericArtUrl(skill.id || skill.name)} alt="" on:error={(e) => onItemArtError(e, { type: 'default' })} />
              </div>
              <div class="spell-body">
                <div class="spell-name">{skillName(skill)}</div>
                {#if skill.description}<div class="spell-desc">{skill.description}</div>{/if}
                <div class="spell-chips">
                  <span class="chip slot">Skill slot {skill.equippedSlot}</span>
                  {#if skill.hotbarSlot > 0}
                    <span class="chip hotbar">Hotbar {skill.hotbarSlot}</span>
                  {/if}
                  <span class="chip cost">{formatSkillCost(skill)}</span>
                  {#each formatSkillEffects(skill) as chip}
                    <span class="chip">{chip}</span>
                  {/each}
                </div>
                <div class="spell-actions">
                  <button
                    type="button"
                    class="spell-btn danger"
                    disabled={inCombat}
                    on:click={() => unequipSkill(skill)}
                  >Unequip</button>
                  <button
                    type="button"
                    class="spell-btn ghost"
                    disabled={inCombat || skill.hotbarSlot > 0}
                    on:click={() => addToHotbar(skill)}
                  >{skill.hotbarSlot > 0 ? 'On hotbar' : 'Add to hotbar'}</button>
                </div>
              </div>
            </div>
          {/each}
        {/if}

        {#if rows.available.length > 0}
          <div class="section-label">Available</div>
          {#each rows.available as skill (skill.id)}
            <div class="spell-row">
              <div class="spell-icon">
                <img src={skillGenericArtUrl(skill.id)} alt="" on:error={(e) => onItemArtError(e, { type: 'default' })} />
              </div>
              <div class="spell-body">
                <div class="spell-name">{skillName(skill)}</div>
                {#if skill.description}<div class="spell-desc">{skill.description}</div>{/if}
                <div class="spell-chips">
                  <span class="chip">Lv {skill.levelRequired}</span>
                  {#if skill.hotbarSlot > 0}
                    <span class="chip hotbar">Hotbar {skill.hotbarSlot}</span>
                  {/if}
                  <span class="chip cost">{formatSkillCost(skill)}</span>
                  {#each formatSkillEffects(skill) as chip}
                    <span class="chip">{chip}</span>
                  {/each}
                </div>
                <div class="spell-actions">
                  <button
                    type="button"
                    class="spell-btn"
                    disabled={inCombat || slotsFull}
                    title={slotsFull ? 'All skill slots are full' : 'Equip into an empty slot'}
                    on:click={() => equipSkill(skill)}
                  >Equip</button>
                  <button
                    type="button"
                    class="spell-btn ghost"
                    disabled={inCombat || skill.hotbarSlot > 0}
                    on:click={() => addToHotbar(skill)}
                  >{skill.hotbarSlot > 0 ? 'On hotbar' : 'Add to hotbar'}</button>
                </div>
              </div>
            </div>
          {/each}
        {/if}

        {#if rows.locked.length > 0}
          <div class="section-label">Locked</div>
          {#each rows.locked as skill (skill.id)}
            <div class="spell-row locked">
              <div class="spell-icon">
                <img src={skillGenericArtUrl(skill.id)} alt="" on:error={(e) => onItemArtError(e, { type: 'default' })} />
              </div>
              <div class="spell-body">
                <div class="spell-name">{skillName(skill)}</div>
                {#if skill.description}<div class="spell-desc">{skill.description}</div>{/if}
                <div class="spell-chips">
                  <span class="chip lock">Unlocks at L{skill.levelRequired}</span>
                  <span class="chip cost">{formatSkillCost(skill)}</span>
                  {#each formatSkillEffects(skill) as chip}
                    <span class="chip">{chip}</span>
                  {/each}
                </div>
              </div>
            </div>
          {/each}
        {/if}
      </div>
    </div>
  </div>
{/if}
