<script>
  import { portraitBustSrc, onPortraitBustError } from '../portraitSrc.js';
  import { relationSummary } from '../relationFeedback.js';

  export let npcName = "";
  export let npcText = "";
  export let options = [];
  export let npcType = "npc";
  export let npc = null;
  export let relation = null;
  export let freeText = false;
  export let sendMessage;

  let freeTextDraft = "";

  const TRADE_TOKEN = "__trade__";

  $: relationLabel = relationSummary(relation, npcName);
  $: relationTone = relation && relation.direction ? relation.direction : "neutral";
  $: relationMark = relationTone === "up" ? "▲" : relationTone === "down" ? "▼" : "•";

  $: displayOptions = (() => {
    const opts = Array.isArray(options) ? options.slice() : [];
    if (npc?.isMerchant) {
      const hasTrade = opts.some((o) => /^(trade|shop)\b/i.test(String(o?.text || "").trim()));
      if (!hasTrade) {
        opts.push({ index: TRADE_TOKEN, text: "Trade / Shop" });
      }
    }
    return opts;
  })();

  function handleOption(option) {
    if (option?.index === TRADE_TOKEN || /^(trade|shop)\b/i.test(String(option?.text || "").trim())) {
      // ExactCommandKey: must be bare "list" / "trade" (no NPC name)
      sendMessage("list");
      return;
    }
    sendMessage(String(option.index));
  }

  function submitFreeText() {
    const text = String(freeTextDraft || "").trim();
    if (!text) return;
    sendMessage(text);
    freeTextDraft = "";
  }

  function isQuestOption(option) {
    return /^quest:/i.test(String(option?.kind || ""))
      || option?.text?.startsWith("[Quest]")
      || option?.text?.startsWith("[Turn In]");
  }

  function isTradeOption(option) {
    return option?.index === TRADE_TOKEN || /^(trade|shop)\b/i.test(String(option?.text || "").trim());
  }
</script>

<style>
  .dialog-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.85);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    z-index: 100;
    display: flex;
    flex-direction: column;
    padding: 1.5em;
    animation: fadeIn 0.3s ease-out;
    overflow-y: auto;
  }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  .dialog-header {
    display: flex;
    align-items: flex-start;
    gap: 1em;
    margin-bottom: 1em;
  }

  /* Large 1:1 bust in a rounded frame so the flat chest crop is clipped. */
  .npc-portrait-slot {
    width: clamp(100px, 22vw, 140px);
    aspect-ratio: 1 / 1;
    flex-shrink: 0;
    position: relative;
    overflow: hidden;
    border-radius: 12px;
    border: 1px solid rgba(255, 255, 255, 0.18);
    background: #111;
    box-sizing: border-box;
  }
  .npc-portrait-slot.enemy {
    border-color: rgba(239, 68, 68, 0.45);
  }
  .npc-portrait-slot.merchant {
    border-color: rgba(34, 197, 94, 0.45);
  }
  .npc-portrait-slot.quest {
    border-color: rgba(245, 158, 11, 0.45);
  }
  .npc-portrait-slot.npc {
    border-color: rgba(59, 130, 246, 0.4);
  }

  .npc-portrait {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    object-position: center center;
    image-rendering: pixelated;
    display: block;
    pointer-events: none;
  }

  .dialog-header-text {
    flex: 1;
    min-width: 0;
  }

  .npc-name {
    font-size: 1.3em;
    font-weight: 600;
    color: #e5e7eb;
    display: block;
    margin-bottom: 0.35em;
  }

  .dialog-text {
    font-size: 1.1em;
    line-height: 1.6;
    color: #d1d5db;
    margin-bottom: 0;
    padding: 1em;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 8px;
    border-left: 3px solid rgba(59, 130, 246, 0.5);
  }

  .dialog-text.quest {
    border-left-color: rgba(245, 158, 11, 0.65);
  }

  /* Subtle relationship-change cue (Batch 27.4). */
  .dialog-relation {
    display: flex;
    align-items: baseline;
    gap: 0.4em;
    margin-top: 0.55em;
    padding-left: 0.15em;
    font-size: 0.84em;
    letter-spacing: 0.02em;
    color: #9ca3af;
  }

  .dialog-relation.up {
    color: #86efac;
  }

  .dialog-relation.down {
    color: #fca5a5;
  }

  .relation-mark {
    font-size: 0.9em;
    line-height: 1;
  }

  .dialog-options {
    display: flex;
    flex-direction: column;
    gap: 0.5em;
    margin-top: 1em;
  }

  .dialog-option-btn {
    display: flex;
    align-items: center;
    gap: 0.75em;
    text-align: left;
    padding: 0.85em 1em;
    background: rgba(59, 130, 246, 0.15);
    border: 1px solid rgba(59, 130, 246, 0.3);
    border-radius: 8px;
    color: #93c5fd;
    font-size: 1em;
    cursor: pointer;
    transition: background 0.15s ease, border-color 0.15s ease;
  }

  .dialog-option-btn:hover,
  .dialog-option-btn:focus {
    background: rgba(59, 130, 246, 0.25);
    border-color: rgba(59, 130, 246, 0.5);
  }

  .dialog-option-btn.quest {
    background: rgba(245, 158, 11, 0.15);
    border-color: rgba(245, 158, 11, 0.35);
    color: #fcd34d;
  }

  .dialog-option-btn.quest:hover,
  .dialog-option-btn.quest:focus {
    background: rgba(245, 158, 11, 0.25);
    border-color: rgba(245, 158, 11, 0.55);
  }

  .dialog-option-btn.trade {
    background: rgba(34, 197, 94, 0.15);
    border-color: rgba(34, 197, 94, 0.35);
    color: #86efac;
  }

  .dialog-option-btn.trade:hover,
  .dialog-option-btn.trade:focus {
    background: rgba(34, 197, 94, 0.25);
    border-color: rgba(34, 197, 94, 0.55);
  }

  .dialog-option-btn.trade .option-index {
    background: rgba(34, 197, 94, 0.28);
  }

  .dialog-option-btn:active {
    transform: translateX(2px);
  }

  .option-index {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    background: rgba(59, 130, 246, 0.3);
    border-radius: 4px;
    font-weight: 600;
    font-size: 0.9em;
    flex-shrink: 0;
  }

  .dialog-option-btn.quest .option-index {
    background: rgba(245, 158, 11, 0.28);
  }

  .option-text {
    flex: 1;
  }

  .dialog-free-text {
    display: flex;
    gap: 0.5em;
    margin-top: 1em;
  }

  .dialog-free-text input {
    flex: 1;
    min-width: 0;
    padding: 0.75em;
    border: 1px solid rgba(255, 255, 255, 0.25);
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.08);
    color: #e5e7eb;
    font: inherit;
  }

  .dialog-free-text button {
    min-width: 5em;
    padding: 0.7em 1em;
    border: 1px solid rgba(59, 130, 246, 0.5);
    border-radius: 8px;
    background: rgba(59, 130, 246, 0.2);
    color: #bfdbfe;
    font: inherit;
    cursor: pointer;
  }

  .dialog-free-text button:disabled {
    opacity: 0.55;
    cursor: default;
  }

  @media screen and (max-width: 600px) {
    .dialog-overlay {
      padding: 1em;
    }

    .npc-portrait-slot {
      width: clamp(80px, 28vw, 110px);
    }

    .npc-name {
      font-size: 1.1em;
    }

    .dialog-text {
      font-size: 1em;
      padding: 0.75em;
    }

    .dialog-option-btn {
      padding: 0.7em 0.85em;
      font-size: 0.95em;
    }
  }
</style>

<div class="dialog-overlay">
  <div class="dialog-header">
    <div class="npc-portrait-slot {npcType}">
      <img
        class="npc-portrait"
        src={portraitBustSrc(npc)}
        alt=""
        on:error={(e) => onPortraitBustError(e, npc)}
      />
    </div>
    <div class="dialog-header-text">
      <span class="npc-name">{npcName}</span>
      <div class="dialog-text" class:quest={npcType === 'quest'}>
        {npcText}
      </div>
      {#if relationLabel}
        <div class="dialog-relation {relationTone}" title={relationLabel}>
          <span class="relation-mark" aria-hidden="true">{relationMark}</span>
          <span>{relationLabel}</span>
        </div>
      {/if}
    </div>
  </div>

  {#if displayOptions && displayOptions.length > 0}
    <div class="dialog-options">
      {#each displayOptions as option}
        <button
          class="dialog-option-btn"
          class:quest={isQuestOption(option)}
          class:trade={isTradeOption(option)}
          data-kind={option.kind || undefined}
          on:click={() => handleOption(option)}
        >
          <span class="option-index">{isTradeOption(option) ? '★' : option.index}</span>
          <span class="option-text">{option.text}</span>
        </button>
      {/each}
    </div>
  {/if}

  {#if freeText}
    <form class="dialog-free-text" on:submit|preventDefault={submitFreeText}>
      <input
        type="text"
        bind:value={freeTextDraft}
        placeholder="Escribe tu respuesta..."
        aria-label="Escribe tu respuesta..."
        autocomplete="off"
      />
      <button type="submit" disabled={!freeTextDraft.trim()}>Enviar</button>
    </form>
  {/if}
</div>
