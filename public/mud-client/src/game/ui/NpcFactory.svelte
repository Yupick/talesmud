<script>
  // Mob / NPC factory (Creator Phase 5.3; remote sync realigned in Batch 21).
  //
  // Opened from the game chrome (`npcFactoryOpen`, shortcut N) and edits a local
  // catalog of NPC/mob templates. All validation and serialization comes from
  // the pure module `../npcFactory.js`.
  //
  // Persistence: templates are stored in localStorage. The backend serves the
  // Batch 21 template CRUD routes (see `../../api/npcs.js`, `/api/npc-templates`),
  // so the panel also offers a best-effort list/push/pull against them when a
  // creator token is present. Route/role failures (401/403/404/501/network)
  // degrade to localStorage with a visible notice, mirroring DungeonBuilder.
  import { onMount } from "svelte";
  import { npcFactoryOpen } from "../uiChrome.js";
  import {
    NPC_ROLES,
    CREATURE_TYPES,
    COMBAT_STYLES,
    DIFFICULTIES,
    LLM_RELATION_KINDS,
    emptyTemplate,
    emptyLootEntry,
    emptyLLMConfig,
    emptyLLMPersonality,
    emptyLLMRelation,
    loadTemplates,
    saveTemplates,
    resetTemplates,
    validateTemplate,
    issueCounts,
    uniqueId,
    duplicateTemplate,
    toTemplatePayload,
    fromTemplatePayload,
  } from "../npcFactory.js";
  import {
    validateBehavior,
    normalizeBehavior,
    scaleTemplate,
    previewScaling,
    toLuaBehavior,
    emptyTransition,
    emptyAbility,
  } from "../npcBehavior.js";
  import {
    NPC_CRUD_AVAILABLE,
    NPC_ENDPOINT_STATUS,
    listNpcTemplates,
    getNpcTemplate,
    createNpcTemplate,
    updateNpcTemplate,
    deleteNpcTemplate,
    isNpcRouteError,
  } from "../../api/npcs.js";

  export let authToken = "";

  let templates = [];
  let selectedId = null;
  let search = "";
  let statusMsg = "";
  let statusKind = "info";
  let revision = 0; // bump on every inline edit so derived checks re-run
  let remoteState = { kind: "idle", msg: "" };
  let remoteBusy = false;
  let serverNotice = ""; // degradation banner (routes missing / no creator role)
  let remoteIds = new Set(); // template ids known to exist on the server
  let scaleTarget = 10; // "Scale to level N" input
  let luaText = ""; // last "Export Lua" output

  function touch() {
    revision += 1;
  }

  function setStatus(msg, kind = "info") {
    statusMsg = msg;
    statusKind = kind;
  }

  function loadFromStorage() {
    templates = loadTemplates();
    if (!selectedId || !templates.some((t) => t.id === selectedId)) {
      selectedId = templates.length ? templates[0].id : null;
    }
    setStatus("Demo catalog loaded from localStorage.", "info");
  }

  onMount(loadFromStorage);

  $: if ($npcFactoryOpen && templates.length === 0) loadFromStorage();

  $: selected = templates.find((t) => t.id === selectedId) || null;
  // Behavior/scaling are optional on legacy templates; normalize on the fly so
  // the reactive checks always see a full shape (persistence does the same via
  // normalizeTemplate).
  $: if (selected && !selected.behavior) selected.behavior = normalizeBehavior();
  $: if (selected && !selected.scaling) selected.scaling = { targetLevel: null };
  $: issues =
    revision >= 0 && selected
      ? [
          ...validateTemplate(selected, {
            otherIds: templates.filter((t) => t !== selected).map((t) => t.id),
          }),
          ...validateBehavior(selected.behavior),
        ]
      : [];
  $: counts = issueCounts(issues);
  $: errorFields = new Set(issues.filter((i) => i.level === "error").map((i) => i.field));
  $: warnFields = new Set(issues.filter((i) => i.level === "warn").map((i) => i.field));
  // Reactive so the template re-evaluates these classes when validation changes.
  $: fieldKind = (field) =>
    errorFields.has(field) ? "has-error" : warnFields.has(field) ? "has-warn" : "";
  $: filteredTemplates = templates.filter((t) => {
    const q = search.trim().toLowerCase();
    if (!q) return true;
    return `${t.id} ${t.name} ${t.role} ${t.creatureType}`.toLowerCase().includes(q);
  });
  $: preview = selected ? buildPreview(selected) : null;
  $: scalePreview =
    selected && Number(scaleTarget) > 0
      ? previewScaling(selected, [Number(selected.level), Number(scaleTarget)])
      : [];

  // Draft text for the comma-separated aggro conditions list.
  let conditionsDraft = "";
  $: if (selected) {
    conditionsDraft = ((selected.behavior && selected.behavior.aggro.conditions) || []).join(", ");
  }

  function selectTemplate(tpl) {
    if (!tpl) return;
    selectedId = tpl.id;
    setStatus(`Editing ${tpl.name}.`, "info");
  }

  function blankTemplate() {
    const tpl = emptyTemplate();
    tpl.id = uniqueId("new_npc", templates.map((t) => t.id));
    tpl.name = "New NPC";
    return tpl;
  }

  function onNew() {
    const tpl = blankTemplate();
    templates = [...templates, tpl];
    selectTemplate(tpl);
    setStatus("New template added. Remember to Save.", "success");
  }

  function onDuplicate() {
    if (!selected) return;
    const copy = duplicateTemplate(selected, templates.map((t) => t.id));
    templates = [...templates, copy];
    selectTemplate(copy);
    setStatus(`Duplicated as ${copy.id}.`, "success");
  }

  async function onDelete() {
    if (!selected) return;
    if (typeof confirm === "function" && !confirm(`Delete template "${selected.name}"?`)) return;
    const id = selected.id;
    const wasRemote = remoteIds.has(id);
    templates = templates.filter((t) => t !== selected);
    selectedId = templates.length ? templates[0].id : null;
    saveTemplates(templates);

    // Best-effort server delete when we know the row exists there.
    if (!wasRemote) {
      setStatus(`Deleted ${id}.`, "success");
      return;
    }
    if (!authToken) {
      setStatus(`Deleted ${id} locally; the server copy was kept (no token).`, "info");
      return;
    }
    remoteBusy = true;
    try {
      await deleteNpcTemplate(authToken, id);
      remoteIds.delete(id);
      serverNotice = "";
      setStatus(`Deleted ${id} from the server and localStorage.`, "success");
    } catch (err) {
      const msg = (err && err.message) || "server delete failed";
      if (isNpcRouteError(err)) {
        markServerUnavailable(
          `Server templates unavailable (${msg}). Removed ${id} from localStorage only.`
        );
      } else {
        setStatus(`Removed ${id} locally; server delete failed (${msg}).`, "error");
      }
    } finally {
      remoteBusy = false;
    }
  }

  function onAddLoot() {
    if (!selected) return;
    selected.loot = [...(selected.loot || []), emptyLootEntry()];
    touch();
  }

  function onRemoveLoot(index) {
    if (!selected) return;
    selected.loot = selected.loot.filter((_, i) => i !== index);
    touch();
  }

  // -------------------------------------------------------------------------
  // LLM / Dialogue
  // -------------------------------------------------------------------------

  function ensureLLM() {
    if (!selected) return null;
    if (!selected.llm || typeof selected.llm !== "object") {
      selected.llm = emptyLLMConfig();
    }
    if (!selected.llm.personality || typeof selected.llm.personality !== "object") {
      selected.llm.personality = emptyLLMPersonality();
    }
    if (!Array.isArray(selected.llm.goals)) selected.llm.goals = [];
    if (!Array.isArray(selected.llm.secrets)) selected.llm.secrets = [];
    if (!Array.isArray(selected.llm.relations)) selected.llm.relations = [];
    return selected.llm;
  }

  function onToggleLLM() {
    if (!selected) return;
    const llm = ensureLLM();
    if (!llm) return;
    llm.enabled = !llm.enabled;
    touch();
  }

  function onEnableLLMChange(e) {
    if (!selected) return;
    const llm = ensureLLM();
    if (!llm) return;
    llm.enabled = !!e.target.checked;
    touch();
  }

  function onAddGoal() {
    const llm = ensureLLM();
    if (!llm) return;
    llm.goals = [...(llm.goals || []), ""];
    touch();
  }

  function onRemoveGoal(index) {
    const llm = ensureLLM();
    if (!llm) return;
    llm.goals = (llm.goals || []).filter((_, i) => i !== index);
    touch();
  }

  function onAddSecret() {
    const llm = ensureLLM();
    if (!llm) return;
    llm.secrets = [...(llm.secrets || []), ""];
    touch();
  }

  function onRemoveSecret(index) {
    const llm = ensureLLM();
    if (!llm) return;
    llm.secrets = (llm.secrets || []).filter((_, i) => i !== index);
    touch();
  }

  function onAddRelation() {
    const llm = ensureLLM();
    if (!llm) return;
    llm.relations = [...(llm.relations || []), emptyLLMRelation()];
    touch();
  }

  function onRemoveRelation(index) {
    const llm = ensureLLM();
    if (!llm) return;
    llm.relations = (llm.relations || []).filter((_, i) => i !== index);
    touch();
  }

  // -------------------------------------------------------------------------
  // Behavior & scaling (Phase 5.3 behavior lite)
  // -------------------------------------------------------------------------

  function ensureBehavior() {
    if (!selected) return null;
    if (!selected.behavior) selected.behavior = normalizeBehavior();
    return selected.behavior;
  }

  function onAddTransition() {
    const behavior = ensureBehavior();
    if (!behavior) return;
    behavior.stateMachine = [...behavior.stateMachine, emptyTransition()];
    touch();
  }

  function onRemoveTransition(index) {
    const behavior = ensureBehavior();
    if (!behavior) return;
    behavior.stateMachine = behavior.stateMachine.filter((_, i) => i !== index);
    touch();
  }

  function onAddAbility() {
    const behavior = ensureBehavior();
    if (!behavior) return;
    behavior.abilities = [...behavior.abilities, emptyAbility()];
    touch();
  }

  function onRemoveAbility(index) {
    const behavior = ensureBehavior();
    if (!behavior) return;
    behavior.abilities = behavior.abilities.filter((_, i) => i !== index);
    touch();
  }

  function onConditionsBlur() {
    const behavior = ensureBehavior();
    if (!behavior) return;
    behavior.aggro.conditions = conditionsDraft
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    touch();
  }

  function onScaleToLevel() {
    if (!selected) return;
    const target = Number(scaleTarget);
    if (!Number.isFinite(target) || target < 1) {
      setStatus("Enter a target level ≥ 1 to scale.", "error");
      return;
    }
    const scaled = scaleTemplate(selected, target);
    // Copy the scaled stats back so the reactive `selected` reference is kept.
    selected.level = scaled.level;
    selected.maxHp = scaled.maxHp;
    selected.damageMin = scaled.damageMin;
    selected.damageMax = scaled.damageMax;
    selected.xpReward = scaled.xpReward;
    selected.goldMin = scaled.goldMin;
    selected.goldMax = scaled.goldMax;
    selected.scaling = { targetLevel: scaled.level };
    touch();
    setStatus(`Scaled ${selected.name} to level ${scaled.level}. Remember to Save.`, "success");
  }

  function onExportLua() {
    if (!selected) return;
    luaText = toLuaBehavior(selected.behavior, {
      npcId: selected.id,
      npcName: selected.name,
    });
    setStatus("Lua preview generated (export only; the server does not run it yet).", "info");
  }

  function onSave() {
    const ok = saveTemplates(templates);
    if (ok) setStatus("Saved to localStorage.", "success");
    else setStatus("Could not save locally (storage unavailable).", "error");
  }

  function onReset() {
    if (typeof confirm === "function" && !confirm("Discard local edits and reload the demo catalog?")) return;
    resetTemplates();
    selectedId = null;
    loadFromStorage();
  }

  function buildPreview(tpl) {
    const hostile = tpl.role === "enemy" || tpl.role === "boss";
    const avg = Math.round((Number(tpl.damageMin) + Number(tpl.damageMax)) / 2);
    const rounds = avg > 0 ? Math.ceil(Number(tpl.maxHp) / avg) : 0;
    return {
      hostile,
      avgDamage: avg,
      hpPerLevel: tpl.level > 0 ? Math.round(Number(tpl.maxHp) / tpl.level) : 0,
      roundsToKill: rounds,
      lootCount: (tpl.loot || []).length,
    };
  }

  // -------------------------------------------------------------------------
  // Best-effort remote sync (Batch 21: /api/npc-templates; only runs on demand
  // with a token). Route/role failures degrade to localStorage with a notice.
  // -------------------------------------------------------------------------

  function markServerUnavailable(msg) {
    serverNotice = msg;
    remoteState = { kind: "error", msg };
  }

  /** Best-effort pull: list summaries, then hydrate full DTOs per row. */
  async function onPullRemote() {
    if (!authToken) {
      remoteState = { kind: "error", msg: "No creator session token; remote pull is unavailable." };
      return;
    }
    remoteBusy = true;
    remoteState = { kind: "idle", msg: "" };
    try {
      const rows = await listNpcTemplates(authToken, { limit: 100 });
      const list = Array.isArray(rows) ? rows : [];
      // The list endpoint returns summaries without loot/behavior; hydrate the
      // full DTO per row, keeping the summary if a single fetch fails.
      const full = await Promise.all(
        list.map(async (row) => {
          try {
            return await getNpcTemplate(authToken, row.id);
          } catch (err) {
            if (isNpcRouteError(err)) throw err; // routes vanished mid-pull
            return row;
          }
        })
      );
      templates = mergeRemote(full);
      remoteIds = new Set(full.map((row) => row && row.id).filter(Boolean));
      if (!templates.some((t) => t.id === selectedId)) {
        selectedId = templates.length ? templates[0].id : null;
      }
      serverNotice = "";
      remoteState = { kind: "success", msg: `Pulled ${full.length} template(s) from the server.` };
    } catch (err) {
      const msg = (err && err.message) || "Remote pull failed.";
      if (isNpcRouteError(err)) {
        markServerUnavailable(
          `Server templates unavailable (${msg}). Using localStorage only.`
        );
      } else {
        remoteState = { kind: "error", msg };
      }
    } finally {
      remoteBusy = false;
    }
  }

  function mergeRemote(rows) {
    const local = templates.slice();
    const byId = new Map(local.map((t) => [t.id, t]));
    for (const row of rows) {
      const tpl = fromTemplatePayload(row);
      if (!tpl.id) continue;
      byId.set(tpl.id, tpl);
    }
    return Array.from(byId.values());
  }

  async function onPushSelected() {
    if (!selected) return;
    if (!authToken) {
      remoteState = { kind: "error", msg: "No creator session token; remote push is unavailable." };
      return;
    }
    const errors = issues.filter((i) => i.level === "error");
    if (errors.length) {
      remoteState = { kind: "error", msg: "Fix validation errors before pushing." };
      return;
    }
    remoteBusy = true;
    remoteState = { kind: "idle", msg: "" };
    const payload = toTemplatePayload(selected);
    try {
      if (remoteIds.has(selected.id)) {
        await updateNpcTemplate(authToken, selected.id, payload);
        remoteState = { kind: "success", msg: `Updated ${selected.id} on the server.` };
      } else {
        const created = await createNpcTemplate(authToken, payload);
        const remoteId = created && created.id ? String(created.id) : selected.id;
        remoteIds.add(remoteId);
        remoteState = { kind: "success", msg: `Created ${selected.id} on the server.` };
      }
      serverNotice = "";
      setStatus("Remote push complete. Remember to Save locally too.", "success");
    } catch (err) {
      const msg = (err && err.message) || "Remote push failed.";
      if (isNpcRouteError(err)) {
        markServerUnavailable(
          `Server templates unavailable (${msg}). Use Save to keep editing in localStorage.`
        );
      } else {
        remoteState = { kind: "error", msg };
      }
    } finally {
      remoteBusy = false;
    }
  }

  function close() {
    npcFactoryOpen.set(false);
  }

  const missingEndpoints = Object.entries(NPC_ENDPOINT_STATUS)
    .filter(([, ok]) => !ok)
    .map(([name]) => name);
</script>

<style>
  .npc-overlay {
    position: fixed;
    inset: 0;
    z-index: 12060;
    background: rgba(6, 8, 12, 0.86);
    display: flex;
    align-items: stretch;
    justify-content: center;
    padding: 24px;
    animation: npcFadeIn 0.15s ease-out;
  }
  @keyframes npcFadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  .npc-panel {
    width: min(1240px, 100%);
    max-height: 100%;
    display: flex;
    flex-direction: column;
    background: linear-gradient(180deg, #161b26 0%, #10141c 100%);
    border: 1px solid #2b3444;
    border-radius: 12px;
    box-shadow: 0 24px 60px rgba(0, 0, 0, 0.55);
    overflow: hidden;
    color: #d8dee9;
  }
  .npc-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 18px;
    border-bottom: 1px solid #232b39;
    background: rgba(255, 255, 255, 0.02);
  }
  .npc-title {
    font-size: 1.05rem;
    font-weight: 600;
    letter-spacing: 0.02em;
  }
  .npc-title i {
    color: #38bdf8;
    margin-right: 8px;
  }
  .badge {
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    border-radius: 999px;
    padding: 3px 8px;
    border: 1px solid #39465a;
    color: #9fb3c8;
  }
  .badge.local { border-color: #b45309; color: #fbbf24; }
  .badge.ok { border-color: #15803d; color: #4ade80; }
  .badge.warn { border-color: #b45309; color: #fbbf24; }
  .badge.err { border-color: #b91c1c; color: #f87171; }
  .spacer { flex: 1; }
  .npc-close {
    background: transparent;
    border: none;
    color: #94a3b8;
    font-size: 1.5rem;
    line-height: 1;
    cursor: pointer;
  }
  .npc-close:hover { color: #e2e8f0; }

  .banner {
    padding: 8px 18px;
    font-size: 0.78rem;
    border-bottom: 1px solid #232b39;
    background: rgba(56, 189, 248, 0.06);
    color: #7dd3fc;
  }
  .banner.warn {
    background: rgba(180, 83, 9, 0.12);
    color: #fbbf24;
  }
  .banner code {
    color: #bae6fd;
    background: rgba(0, 0, 0, 0.3);
    border-radius: 3px;
    padding: 0 4px;
  }

  .npc-body {
    flex: 1;
    display: grid;
    grid-template-columns: 260px minmax(0, 1fr) 300px;
    min-height: 0;
  }
  .col {
    min-height: 0;
    overflow-y: auto;
    padding: 14px;
  }
  .col.list { border-right: 1px solid #232b39; }
  .col.form { background: rgba(0, 0, 0, 0.12); }
  .col.check { border-left: 1px solid #232b39; }

  .toolbar {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 10px;
  }
  button.btn {
    font-size: 0.75rem;
    border-radius: 6px;
    border: 1px solid #39465a;
    background: #1b2230;
    color: #cbd5e1;
    padding: 5px 9px;
    cursor: pointer;
  }
  button.btn:hover { background: #232c3d; border-color: #4b5a71; }
  button.btn.primary { background: #0c4a6e; border-color: #0284c7; color: #e0f2fe; }
  button.btn.danger { border-color: #7f1d1d; color: #fca5a5; }
  button.btn.ghost { background: transparent; }
  button.btn:disabled { opacity: 0.45; cursor: default; }

  input[type="text"], input[type="number"], select, textarea {
    width: 100%;
    box-sizing: border-box;
    background: #0d1119;
    border: 1px solid #2b3444;
    border-radius: 6px;
    color: #e2e8f0;
    font-size: 0.78rem;
    padding: 5px 7px;
  }
  textarea { resize: vertical; min-height: 42px; }
  input:focus, select:focus, textarea:focus { outline: none; border-color: #0ea5e9; }
  label.field { display: block; margin-bottom: 8px; }
  .field-label {
    display: block;
    font-size: 0.66rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: #7f8ea3;
    margin-bottom: 3px;
  }
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .grid3 { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
  .grid4 { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
  .grid5 { display: grid; grid-template-columns: repeat(5, 1fr); gap: 8px; }

  .row-card {
    width: 100%;
    text-align: left;
    border: 1px solid transparent;
    background: transparent;
    color: #cbd5e1;
    border-radius: 8px;
    padding: 8px 10px;
    cursor: pointer;
    margin-bottom: 4px;
  }
  .row-card:hover { background: rgba(255, 255, 255, 0.04); }
  .row-card.active { background: rgba(14, 165, 233, 0.16); border-color: #0284c7; }
  .row-card .name { font-size: 0.85rem; font-weight: 600; }
  .row-card .meta { font-size: 0.68rem; color: #7f8ea3; }

  .section-title {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: #8b9bb4;
    margin: 14px 0 8px;
  }
  .section-title:first-child { margin-top: 0; }

  .has-error input, .has-error select { border-color: #b91c1c; }
  .has-warn input, .has-warn select { border-color: #b45309; }
  .loot-item.has-error { border-color: #b91c1c; }
  .loot-item.has-warn { border-color: #b45309; }
  .field-inline {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.76rem;
    color: #cbd5e1;
  }
  .field-inline input[type="checkbox"] { width: auto; }

  .loot-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 72px 72px auto;
    gap: 6px;
    align-items: center;
    margin-bottom: 6px;
  }
  .icon-btn {
    border: none;
    background: transparent;
    color: #94a3b8;
    cursor: pointer;
    font-size: 0.9rem;
    padding: 2px 4px;
  }
  .icon-btn:hover { color: #e2e8f0; }

  .behavior-note {
    font-size: 0.7rem;
    color: #7f8ea3;
    margin-bottom: 8px;
  }
  .sub-title {
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: #64748b;
    margin: 10px 0 5px;
  }
  .transition-row, .ability-row {
    display: grid;
    gap: 6px;
    align-items: center;
    margin-bottom: 6px;
  }
  .transition-row { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr) auto; }
  .ability-row { grid-template-columns: minmax(0, 1fr) 72px 72px auto; }
  .llm-list-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 6px;
    align-items: center;
    margin-bottom: 6px;
  }
  .llm-rel-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 110px 90px auto;
    gap: 6px;
    align-items: center;
    margin-bottom: 6px;
  }
  .transition-row input.has-error, .ability-row input.has-error { border-color: #b91c1c; }
  .scale-row {
    display: flex;
    align-items: flex-end;
    gap: 8px;
    margin-bottom: 8px;
  }
  .scale-row .field { flex: 0 0 110px; }
  .scale-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.74rem;
    margin-bottom: 8px;
  }
  .scale-table th, .scale-table td {
    border-bottom: 1px solid #263041;
    padding: 3px 6px;
    text-align: left;
    color: #cbd5e1;
  }
  .scale-table th { color: #7f8ea3; font-weight: 600; text-transform: uppercase; font-size: 0.64rem; }
  textarea.lua-out {
    font-family: "SFMono-Regular", Consolas, monospace;
    font-size: 0.68rem;
    background: #0a0e15;
    min-height: 160px;
  }

  .issue {
    font-size: 0.72rem;
    border-radius: 5px;
    padding: 5px 7px;
    margin-bottom: 5px;
    border-left: 3px solid transparent;
    background: rgba(255, 255, 255, 0.03);
  }
  .issue.error { border-left-color: #ef4444; color: #fca5a5; }
  .issue.warn { border-left-color: #f59e0b; color: #fcd34d; }
  .issue .field { font-family: monospace; color: #94a3b8; }
  .issues-empty { color: #4ade80; font-size: 0.78rem; }

  .preview {
    border: 1px solid #263041;
    border-radius: 8px;
    padding: 10px;
    margin-top: 10px;
    background: rgba(14, 165, 233, 0.08);
  }
  .preview .stat { font-size: 0.78rem; color: #cbd5e1; display: flex; justify-content: space-between; }
  .preview .stat b { color: #e0f2fe; }

  .remote {
    border: 1px solid #263041;
    border-radius: 8px;
    padding: 10px;
    margin-top: 10px;
    background: rgba(255, 255, 255, 0.02);
  }
  .remote .note { font-size: 0.72rem; color: #9fb3c8; margin-bottom: 6px; }
  .remote .msg { font-size: 0.74rem; margin-top: 6px; }
  .remote .msg.success { color: #4ade80; }
  .remote .msg.error { color: #f87171; }

  .status {
    padding: 6px 18px;
    border-top: 1px solid #232b39;
    font-size: 0.74rem;
    color: #9fb3c8;
  }
  .status.success { color: #4ade80; }
  .status.error { color: #f87171; }

  @media (max-width: 980px) {
    .npc-body { grid-template-columns: 1fr; grid-template-rows: auto auto auto; }
    .col { overflow: visible; }
    .col.list, .col.check { border: none; }
  }
</style>

{#if $npcFactoryOpen}
  <div class="npc-overlay" role="dialog" aria-label="Mob and NPC Factory">
    <div class="npc-panel">
      <div class="npc-header">
        <div class="npc-title"><i class="material-icons">smart_toy</i>Mob &amp; NPC Factory</div>
        <span class="badge local">localStorage</span>
        {#if NPC_CRUD_AVAILABLE}
          <span class="badge ok">server CRUD</span>
        {/if}
        {#if counts.errors > 0}
          <span class="badge err">{counts.errors} error{counts.errors === 1 ? "" : "s"}</span>
        {/if}
        {#if counts.warnings > 0}
          <span class="badge warn">{counts.warnings} warn{counts.warnings === 1 ? "" : "s"}</span>
        {/if}
        <div class="spacer"></div>
        <button class="btn primary" type="button" on:click={onSave}>Save</button>
        <button class="npc-close" type="button" on:click={close} aria-label="Close factory">&times;</button>
      </div>

      {#if serverNotice}
        <div class="banner warn">{serverNotice}</div>
      {:else if !NPC_CRUD_AVAILABLE && missingEndpoints.length}
        <div class="banner">
          Some NPC endpoints are pending. Missing: {missingEndpoints.join(", ")} — see
          <code>src/api/npcs.js</code>. Templates are kept in localStorage.
        </div>
      {/if}

      <div class="npc-body">
        <!-- Template list -->
        <div class="col list">
          <div class="toolbar">
            <button class="btn" type="button" on:click={onNew}>+ New</button>
            <button class="btn" type="button" on:click={onDuplicate} disabled={!selected}>Duplicate</button>
            <button class="btn danger" type="button" on:click={onDelete} disabled={!selected || remoteBusy}>Delete</button>
            <button class="btn ghost" type="button" on:click={onReset}>Reset demo</button>
          </div>
          <input type="text" placeholder="Search templates…" bind:value={search} />
          <div class="section-title">{filteredTemplates.length} template{filteredTemplates.length === 1 ? "" : "s"}</div>
          {#each filteredTemplates as tpl}
            <button
              type="button"
              class="row-card"
              class:active={tpl.id === selectedId}
              on:click={() => selectTemplate(tpl)}
            >
              <div class="name">{tpl.name || "(unnamed)"}</div>
              <div class="meta">{tpl.id} · {tpl.role} · lvl {tpl.level}</div>
            </button>
          {/each}
        </div>

        <!-- Template form -->
        <div class="col form">
          {#if !selected}
            <div class="issues-empty">No template selected. Create one to begin.</div>
          {:else}
            <div class="section-title">Identity</div>
            <div class="grid2">
              <label class="field {fieldKind("id")}">
                <span class="field-label">Id</span>
                <input type="text" bind:value={selected.id} on:input={touch} />
              </label>
              <label class="field {fieldKind("name")}">
                <span class="field-label">Name</span>
                <input type="text" bind:value={selected.name} on:input={touch} />
              </label>
            </div>
            <label class="field">
              <span class="field-label">Description</span>
              <textarea bind:value={selected.description} on:input={touch}></textarea>
            </label>
            <label class="field field-inline">
              <input type="checkbox" bind:checked={selected.isTemplate} on:change={touch} />
              <span>Template (spawnable)</span>
            </label>

            <div class="section-title">Role &amp; type</div>
            <div class="grid2">
              <label class="field {fieldKind("role")}">
                <span class="field-label">Role</span>
                <select bind:value={selected.role} on:change={touch}>
                  {#each NPC_ROLES as role}
                    <option value={role.id}>{role.label}</option>
                  {/each}
                </select>
              </label>
              <label class="field {fieldKind("creatureType")}">
                <span class="field-label">Creature type</span>
                <select bind:value={selected.creatureType} on:change={touch}>
                  {#each CREATURE_TYPES as type}
                    <option value={type}>{type}</option>
                  {/each}
                </select>
              </label>
            </div>
            <div class="grid2">
              <label class="field {fieldKind("combatStyle")}">
                <span class="field-label">Combat style</span>
                <select bind:value={selected.combatStyle} on:change={touch}>
                  {#each COMBAT_STYLES as style}
                    <option value={style}>{style}</option>
                  {/each}
                </select>
              </label>
              <label class="field {fieldKind("difficulty")}">
                <span class="field-label">Difficulty</span>
                <select bind:value={selected.difficulty} on:change={touch}>
                  {#each DIFFICULTIES as diff}
                    <option value={diff}>{diff}</option>
                  {/each}
                </select>
              </label>
            </div>

            <div class="section-title">Combat stats</div>
            <div class="grid4">
              <label class="field {fieldKind("level")}">
                <span class="field-label">Level</span>
                <input type="number" step="1" min="1" bind:value={selected.level} on:input={touch} />
              </label>
              <label class="field {fieldKind("maxHp")}">
                <span class="field-label">Max HP</span>
                <input type="number" step="1" min="1" bind:value={selected.maxHp} on:input={touch} />
              </label>
              <label class="field {fieldKind("damageMin")}">
                <span class="field-label">Damage min</span>
                <input type="number" step="1" min="0" bind:value={selected.damageMin} on:input={touch} />
              </label>
              <label class="field {fieldKind("damageMax")}">
                <span class="field-label">Damage max</span>
                <input type="number" step="1" min="0" bind:value={selected.damageMax} on:input={touch} />
              </label>
            </div>
            <div class="grid3">
              <label class="field {fieldKind("xpReward")}">
                <span class="field-label">XP reward</span>
                <input type="number" step="1" min="0" bind:value={selected.xpReward} on:input={touch} />
              </label>
              <label class="field {fieldKind("goldMin")}">
                <span class="field-label">Gold min</span>
                <input type="number" step="1" min="0" bind:value={selected.goldMin} on:input={touch} />
              </label>
              <label class="field {fieldKind("goldMax")}">
                <span class="field-label">Gold max</span>
                <input type="number" step="1" min="0" bind:value={selected.goldMax} on:input={touch} />
              </label>
            </div>
            <div class="grid2">
              <label class="field field-inline">
                <input type="checkbox" bind:checked={selected.aggressive} on:change={touch} />
                <span>Aggressive (attacks on sight)</span>
              </label>
              <label class="field {fieldKind("aggroRadius")}">
                <span class="field-label">Aggro radius (rooms)</span>
                <input type="number" step="1" min="0" bind:value={selected.aggroRadius} on:input={touch} />
              </label>
            </div>

            <div class="section-title">Spawn area</div>
            <div class="grid3">
              <label class="field">
                <span class="field-label">Spawn room id</span>
                <input type="text" bind:value={selected.spawnRoomId} on:input={touch} placeholder="room_id" />
              </label>
              <label class="field {fieldKind("spawnCount")}">
                <span class="field-label">Spawn count</span>
                <input type="number" step="1" min="0" bind:value={selected.spawnCount} on:input={touch} />
              </label>
              <label class="field {fieldKind("respawnSeconds")}">
                <span class="field-label">Respawn (s)</span>
                <input type="number" step="1" min="0" bind:value={selected.respawnSeconds} on:input={touch} />
              </label>
            </div>

            <div class="section-title">Loot</div>
            <div class="grid2">
              <label class="field">
                <span class="field-label">Loot table id (optional)</span>
                <input type="text" bind:value={selected.lootTableId} on:input={touch} placeholder="loot_table_id" />
              </label>
            </div>
            <div class="toolbar">
              <button class="btn" type="button" on:click={onAddLoot}>+ Add drop</button>
            </div>
            {#each selected.loot || [] as entry, index (index)}
              <div class="loot-row">
                <input
                  type="text"
                  class="loot-item"
                  class:has-error={errorFields.has(`loot[${index}].itemId`)}
                  class:has-warn={warnFields.has(`loot[${index}].itemId`)}
                  bind:value={entry.itemId}
                  on:input={touch}
                  placeholder="item id"
                />
                <input type="number" step="1" min="0" max="100" bind:value={entry.chance} on:input={touch} title="Drop chance %" />
                <input type="number" step="1" min="1" bind:value={entry.quantity} on:input={touch} title="Quantity" />
                <button class="icon-btn" type="button" title="Remove drop" on:click={() => onRemoveLoot(index)}>✕</button>
              </div>
            {/each}

            <div class="section-title">Dialogue / LLM</div>
            <label class="field field-inline">
              <input
                type="checkbox"
                checked={!!(selected.llm && selected.llm.enabled)}
                on:change={onEnableLLMChange}
              />
              <span>Enable free-form dialogue (ask)</span>
            </label>
            {#if selected.llm && selected.llm.enabled}
              <div class="grid2">
                <label class="field">
                  <span class="field-label">Persona</span>
                  <textarea bind:value={selected.llm.persona} on:input={touch}></textarea>
                </label>
                <label class="field">
                  <span class="field-label">Voice</span>
                  <input type="text" bind:value={selected.llm.voice} on:input={touch} placeholder="brusco, anciano..." />
                </label>
              </div>
              <div class="grid3">
                <label class="field">
                  <span class="field-label">Temperature (0-1)</span>
                  <input type="number" step="0.1" min="0" max="1" bind:value={selected.llm.temperature} on:input={touch} />
                </label>
                <label class="field">
                  <span class="field-label">MemoryTurns</span>
                  <input type="number" step="1" min="0" bind:value={selected.llm.memoryTurns} on:input={touch} />
                </label>
                <label class="field">
                  <span class="field-label">CooldownSeconds</span>
                  <input type="number" step="1" min="0" bind:value={selected.llm.cooldownSeconds} on:input={touch} />
                </label>
              </div>
              <div class="grid2">
                <label class="field">
                  <span class="field-label">FallbackDialogId (optional)</span>
                  <input type="text" bind:value={selected.llm.fallbackDialogId} on:input={touch} placeholder="dialog_id" />
                </label>
              </div>

              <div class="sub-title">Big5 Personality (0-1)</div>
              <div class="grid5">
                <label class="field">
                  <span class="field-label">Openness</span>
                  <input type="number" step="0.1" min="0" max="1" bind:value={selected.llm.personality.openness} on:input={touch} />
                </label>
                <label class="field">
                  <span class="field-label">Conscientiousness</span>
                  <input type="number" step="0.1" min="0" max="1" bind:value={selected.llm.personality.conscientiousness} on:input={touch} />
                </label>
                <label class="field">
                  <span class="field-label">Extraversion</span>
                  <input type="number" step="0.1" min="0" max="1" bind:value={selected.llm.personality.extraversion} on:input={touch} />
                </label>
                <label class="field">
                  <span class="field-label">Agreeableness</span>
                  <input type="number" step="0.1" min="0" max="1" bind:value={selected.llm.personality.agreeableness} on:input={touch} />
                </label>
                <label class="field">
                  <span class="field-label">Neuroticism</span>
                  <input type="number" step="0.1" min="0" max="1" bind:value={selected.llm.personality.neuroticism} on:input={touch} />
                </label>
              </div>

              <div class="sub-title">Goals</div>
              <div class="toolbar">
                <button class="btn" type="button" on:click={onAddGoal}>+ Add goal</button>
              </div>
              {#each selected.llm.goals || [] as goal, index (index)}
                <div class="llm-list-row">
                  <input type="text" bind:value={goal} on:input={touch} placeholder="goal" />
                  <button class="icon-btn" type="button" title="Remove goal" on:click={() => onRemoveGoal(index)}>✕</button>
                </div>
              {/each}

              <div class="sub-title">Secrets</div>
              <div class="toolbar">
                <button class="btn" type="button" on:click={onAddSecret}>+ Add secret</button>
              </div>
              {#each selected.llm.secrets || [] as secret, index (index)}
                <div class="llm-list-row">
                  <input type="text" bind:value={secret} on:input={touch} placeholder="secret" />
                  <button class="icon-btn" type="button" title="Remove secret" on:click={() => onRemoveSecret(index)}>✕</button>
                </div>
              {/each}

              <div class="sub-title">Relations</div>
              <div class="toolbar">
                <button class="btn" type="button" on:click={onAddRelation}>+ Add relation</button>
              </div>
              {#each selected.llm.relations || [] as rel, index (index)}
                <div class="llm-rel-row">
                  <input type="text" bind:value={rel.target} on:input={touch} placeholder="target (NPC/faction/player)" />
                  <select bind:value={rel.kind} on:change={touch}>
                    {#each LLM_RELATION_KINDS as kind}
                      <option value={kind}>{kind}</option>
                    {/each}
                  </select>
                  <input type="number" step="0.1" min="-1" max="1" bind:value={rel.sentiment} on:input={touch} placeholder="-1..1" />
                  <button class="icon-btn" type="button" title="Remove relation" on:click={() => onRemoveRelation(index)}>✕</button>
                </div>
              {/each}
            {/if}

            <div class="section-title">Behavior &amp; Scaling</div>
            <div class="behavior-note">
              Behavior is a data model for the future Lua AI (state machine + aggro + abilities).
              "Export Lua" produces a commented preview only — the server does not run it yet.
            </div>

            <div class="sub-title">State machine transitions</div>
            <div class="toolbar">
              <button class="btn" type="button" on:click={onAddTransition}>+ Add transition</button>
            </div>
            {#each (selected.behavior && selected.behavior.stateMachine) || [] as tr, index (index)}
              <div class="transition-row">
                <input
                  type="text"
                  class:has-error={errorFields.has(`behavior.stateMachine[${index}].from`)}
                  bind:value={tr.from}
                  on:input={touch}
                  placeholder="from"
                  title="Source state"
                />
                <input
                  type="text"
                  class:has-error={errorFields.has(`behavior.stateMachine[${index}].event`)}
                  bind:value={tr.event}
                  on:input={touch}
                  placeholder="event"
                  title="Triggering event"
                />
                <input
                  type="text"
                  class:has-error={errorFields.has(`behavior.stateMachine[${index}].to`)}
                  bind:value={tr.to}
                  on:input={touch}
                  placeholder="to"
                  title="Target state"
                />
                <button class="icon-btn" type="button" title="Remove transition" on:click={() => onRemoveTransition(index)}>✕</button>
              </div>
            {/each}

            <div class="grid2">
              <label class="field {fieldKind('behavior.aggro.radius')}">
                <span class="field-label">Behavior aggro radius</span>
                <input
                  type="number"
                  step="1"
                  min="0"
                  bind:value={selected.behavior.aggro.radius}
                  on:input={touch}
                />
              </label>
              <label class="field">
                <span class="field-label">Aggro conditions (comma-sep)</span>
                <input
                  type="text"
                  bind:value={conditionsDraft}
                  on:change={onConditionsBlur}
                  placeholder="hp_below_50, target_is_healer"
                />
              </label>
            </div>

            <div class="sub-title">Abilities</div>
            <div class="toolbar">
              <button class="btn" type="button" on:click={onAddAbility}>+ Add ability</button>
            </div>
            {#each (selected.behavior && selected.behavior.abilities) || [] as ability, index (index)}
              <div class="ability-row">
                <input
                  type="text"
                  class:has-error={errorFields.has(`behavior.abilities[${index}].id`)}
                  bind:value={ability.id}
                  on:input={touch}
                  placeholder="ability id"
                  title="Ability id"
                />
                <input
                  type="number"
                  step="1"
                  min="0"
                  class:has-error={errorFields.has(`behavior.abilities[${index}].cooldown`)}
                  bind:value={ability.cooldown}
                  on:input={touch}
                  title="Cooldown (s)"
                  placeholder="cd"
                />
                <input
                  type="number"
                  step="1"
                  min="0"
                  class:has-error={errorFields.has(`behavior.abilities[${index}].range`)}
                  bind:value={ability.range}
                  on:input={touch}
                  title="Range"
                  placeholder="range"
                />
                <button class="icon-btn" type="button" title="Remove ability" on:click={() => onRemoveAbility(index)}>✕</button>
              </div>
            {/each}

            <div class="sub-title">Level scaling</div>
            <div class="scale-row">
              <label class="field" style="margin-bottom:0">
                <span class="field-label">Target level</span>
                <input type="number" step="1" min="1" max="100" bind:value={scaleTarget} />
              </label>
              <button class="btn primary" type="button" on:click={onScaleToLevel} disabled={!selected}>
                Scale to level {scaleTarget}
              </button>
            </div>
            {#if scalePreview.length === 2}
              <table class="scale-table">
                <thead>
                  <tr><th>Lvl</th><th>HP</th><th>Dmg</th><th>XP</th><th>Gold</th></tr>
                </thead>
                <tbody>
                  {#each scalePreview as row, i (i)}
                    <tr>
                      <td>{row.level}</td>
                      <td>{row.maxHp}</td>
                      <td>{row.damageMin}–{row.damageMax}</td>
                      <td>{row.xpReward}</td>
                      <td>{row.goldMin}–{row.goldMax}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}
            <div class="toolbar">
              <button class="btn" type="button" on:click={onExportLua} disabled={!selected}>Export Lua</button>
            </div>
            {#if luaText}
              <textarea class="lua-out" readonly rows="10" value={luaText}></textarea>
            {/if}
          {/if}
        </div>

        <!-- Validation + preview + remote -->
        <div class="col check">
          <div class="section-title">Validation</div>
          {#if !selected}
            <div class="issues-empty">—</div>
          {:else if issues.length === 0}
            <div class="issues-empty">No issues. This template is valid.</div>
          {:else}
            {#each issues as item}
              <div class={`issue ${item.level}`}>
                <span class="field">{item.field}</span> — {item.msg}
              </div>
            {/each}
          {/if}

          <div class="section-title">Preview</div>
          {#if !selected || !preview}
            <div class="issues-empty">—</div>
          {:else}
            <div class="preview">
              <div class="stat"><span>Hostile</span><b>{preview.hostile ? "yes" : "no"}</b></div>
              <div class="stat"><span>Avg damage</span><b>{preview.avgDamage}</b></div>
              <div class="stat"><span>HP per level</span><b>{preview.hpPerLevel}</b></div>
              <div class="stat"><span>Rounds to kill</span><b>{preview.roundsToKill || "—"}</b></div>
              <div class="stat"><span>Loot drops</span><b>{preview.lootCount}</b></div>
            </div>
          {/if}

          <div class="section-title">Server (best-effort)</div>
          <div class="remote">
            <div class="note">
              {NPC_CRUD_AVAILABLE
                ? "Template CRUD routes exist (/api/npc-templates). Push/pull/delete is optional; localStorage stays the source of truth."
                : "Template CRUD routes are pending; local only."}
            </div>
            <div class="toolbar">
              <button class="btn" type="button" on:click={onPullRemote} disabled={remoteBusy || !authToken}>Pull templates</button>
              <button class="btn" type="button" on:click={onPushSelected} disabled={remoteBusy || !selected || !authToken}>Push selected</button>
            </div>
            {#if !authToken}
              <div class="msg error">No creator session token; remote actions are disabled.</div>
            {:else if remoteState.msg}
              <div class={`msg ${remoteState.kind}`}>{remoteState.msg}</div>
            {/if}
          </div>
        </div>
      </div>

      <div class={`status ${statusKind}`}>{statusMsg}</div>
    </div>
  </div>
{/if}
