<script>
  // Class / Skill balance editor (Phase 5.2).
  //
  // The panel is opened from the game chrome (`classesEditorOpen`, shortcut L)
  // and edits the active class catalog. All validation and preview math comes
  // from the pure modules `../balanceRules.js` and `../classEditor.js`.
  //
  // Persistence: the backend serves the class CRUD routes, so the panel loads
  // and saves through `src/api/classes.js` with the creator token. It only
  // falls back to localStorage when the network itself is unavailable.
  import { onMount } from "svelte";
  import { classesEditorOpen } from "../uiChrome.js";
  import {
    SCHOOLS,
    COMPONENTS,
    ARMOR_TYPES,
    COMBAT_TYPES,
    TARGETS,
    PRIMARY_ATTRIBUTES,
    ATTRIBUTES,
  } from "../classEditor.js";
  import {
    loadClasses,
    saveClasses,
    resetClasses,
    emptyClass,
    emptySkill,
    duplicateClass,
    normalizeClass,
    uniqueId,
  } from "../classEditor.js";
  import {
    EFFECTS,
    validateClass,
    issueCounts,
    previewSkill,
    previewContextFor,
    estimatedMaxHp,
  } from "../balanceRules.js";
  import {
    CLASS_DEF_CRUD_AVAILABLE,
    CLASS_ENDPOINT_STATUS,
    listClassDefs,
    createClassDef,
    updateClassDef,
    deleteClassDef,
    duplicateClassDef,
  } from "../../api/classes.js";

  export let authToken = "";

  let classes = [];
  let selectedId = null;
  let selectedSkillIndex = -1;
  let search = "";
  let statusMsg = "";
  let statusKind = "info";
  let revision = 0; // bump on every inline edit so derived checks re-run
  let remoteIds = new Set(); // class ids known to exist on the server

  // Preview controls.
  let previewLevel = 1;
  let previewPrimary = 0;
  let previewMaxHp = 0;

  function touch() {
    revision += 1;
  }

  function setStatus(msg, kind = "info") {
    statusMsg = msg;
    statusKind = kind;
  }

  function applyCatalog(list, message) {
    classes = (list || []).map(normalizeClass);
    remoteIds = new Set(classes.map((c) => c.id));
    if (!selectedId || !classes.some((c) => c.id === selectedId)) {
      selectedId = classes.length ? classes[0].id : null;
    }
    selectedSkillIndex = -1;
    setStatus(message, "info");
  }

  function loadFromStorage() {
    classes = loadClasses();
    remoteIds = new Set();
    if (!selectedId || !classes.some((c) => c.id === selectedId)) {
      selectedId = classes.length ? classes[0].id : null;
    }
    selectedSkillIndex = -1;
    setStatus("Demo catalog loaded from localStorage.", "info");
  }

  function isNetworkError(err) {
    return /network error/i.test(String((err && err.message) || err || ""));
  }

  async function loadFromServer() {
    const data = await listClassDefs(authToken);
    const list = Array.isArray(data) ? data : data && Array.isArray(data.classes) ? data.classes : [];
    applyCatalog(list, "Remote catalog loaded.");
  }

  async function loadCatalog() {
    if (CLASS_DEF_CRUD_AVAILABLE && authToken) {
      try {
        await loadFromServer();
        return;
      } catch (err) {
        if (!isNetworkError(err)) {
          setStatus(`Could not load remote classes: ${(err && err.message) || err}`, "error");
          return;
        }
      }
    }
    loadFromStorage();
  }

  onMount(loadCatalog);

  $: if ($classesEditorOpen && classes.length === 0) loadCatalog();

  $: selected = classes.find((c) => c.id === selectedId) || null;
  $: selectedSkill =
    selected && selected.skills && selected.skills[selectedSkillIndex]
      ? selected.skills[selectedSkillIndex]
      : null;
  $: issues = revision >= 0 && selected ? validateClass(selected, classes) : [];
  $: counts = issueCounts(issues);
  $: classIssues = issues.filter((i) => !/^skills\[\d+\]\./.test(i.field));
  $: filteredClasses = classes.filter((c) => {
    const q = search.trim().toLowerCase();
    if (!q) return true;
    return `${c.id} ${c.name} ${c.school}`.toLowerCase().includes(q);
  });
  $: previewCtx =
    revision >= 0 && selected
      ? previewContextFor(selected, {
          level: previewLevel,
          primaryValue: previewPrimary,
          maxHp: previewMaxHp,
        })
      : null;
  $: preview = previewCtx && selectedSkill ? previewSkill(selectedSkill, previewCtx) : null;

  function issuesForSkill(index) {
    const prefix = `skills[${index}].`;
    return issues
      .filter((i) => i.field.startsWith(prefix))
      .map((i) => ({ ...i, shortField: i.field.slice(prefix.length) }));
  }

  function selectClass(cls) {
    if (!cls) return;
    selectedId = cls.id;
    selectedSkillIndex = -1;
    syncPreview(null, cls);
    setStatus(`Editing ${cls.name}.`, "info");
  }

  function selectSkill(index) {
    if (!selected) return;
    selectedSkillIndex = selectedSkillIndex === index ? -1 : index;
    syncPreview(selectedSkill, selected);
  }

  function syncPreview(skill, cls) {
    const classDef = cls || selected;
    const skillDef = skill || selectedSkill;
    previewLevel = skillDef && Number(skillDef.level) >= 1 ? Number(skillDef.level) : 1;
    const attr = (classDef && classDef.primary ? classDef.primary : "STR").toLowerCase();
    previewPrimary = classDef && classDef.attributes ? Number(classDef.attributes[attr] || 0) : 0;
    previewMaxHp = classDef ? estimatedMaxHp(classDef.attributes) : 0;
  }

  function blankClass() {
    const cls = emptyClass();
    cls.id = uniqueId("new_class", classes.map((c) => c.id));
    cls.name = "New Class";
    return cls;
  }

  function onNewClass() {
    const cls = blankClass();
    classes = [...classes, cls];
    selectClass(cls);
    setStatus("New class added. Remember to Save.", "success");
  }

  async function onDuplicateClass() {
    if (!selected) return;
    if (CLASS_DEF_CRUD_AVAILABLE && authToken) {
      try {
        const created = await duplicateClassDef(authToken, selected.id);
        await loadFromServer();
        const copy = classes.find((c) => c.id === (created && created.id));
        if (copy) selectClass(copy);
        setStatus(`Duplicated as ${(created && created.id) || "copy"}.`, "success");
        return;
      } catch (err) {
        if (!isNetworkError(err)) {
          setStatus(`Could not duplicate: ${(err && err.message) || err}`, "error");
          return;
        }
      }
    }
    const copy = duplicateClass(selected, classes.map((c) => c.id));
    classes = [...classes, copy];
    selectClass(copy);
    setStatus(`Duplicated as ${copy.id}.`, "success");
  }

  async function onDeleteClass() {
    if (!selected) return;
    if (typeof confirm === "function" && !confirm(`Delete class "${selected.name}"?`)) return;
    const target = selected;
    if (CLASS_DEF_CRUD_AVAILABLE && authToken && remoteIds.has(target.id)) {
      try {
        await deleteClassDef(authToken, target.id);
      } catch (err) {
        if (!isNetworkError(err)) {
          setStatus(`Could not delete: ${(err && err.message) || err}`, "error");
          return;
        }
      }
      remoteIds.delete(target.id);
    }
    classes = classes.filter((c) => c !== target);
    selectedId = classes.length ? classes[0].id : null;
    selectedSkillIndex = -1;
    setStatus(`Deleted ${target.id}.`, "success");
  }

  function onAddSkill() {
    if (!selected) return;
    const skill = emptySkill(selected.id, "slam");
    skill.id = uniqueId(`${selected.id}_skill`, selected.skills.map((s) => s.id));
    selected.skills = [...selected.skills, skill];
    selectedSkillIndex = selected.skills.length - 1;
    syncPreview(skill, selected);
    touch();
    setStatus("Skill added.", "success");
  }

  function onRemoveSkill(index) {
    if (!selected) return;
    const skill = selected.skills[index];
    selected.skills = selected.skills.filter((_, i) => i !== index);
    if (selectedSkillIndex >= selected.skills.length) {
      selectedSkillIndex = selected.skills.length - 1;
    }
    touch();
    setStatus(`Removed ${skill ? skill.name : "skill"}.`, "success");
  }

  function onMoveSkill(index, delta) {
    if (!selected) return;
    const target = index + delta;
    if (target < 0 || target >= selected.skills.length) return;
    const next = selected.skills.slice();
    const [row] = next.splice(index, 1);
    next.splice(target, 0, row);
    selected.skills = next;
    if (selectedSkillIndex === index) selectedSkillIndex = target;
    touch();
  }

  function toggleHotbar(skillId) {
    if (!selected) return;
    const list = Array.isArray(selected.hotbar) ? selected.hotbar.slice() : [];
    const index = list.indexOf(skillId);
    if (index >= 0) list.splice(index, 1);
    else if (list.length < Number(selected.hotbarCap || 4)) list.push(skillId);
    selected.hotbar = list;
    touch();
  }

  async function onSave() {
    if (!(CLASS_DEF_CRUD_AVAILABLE && authToken)) {
      const ok = saveClasses(classes);
      if (ok) setStatus("Saved to localStorage.", "success");
      else setStatus("Could not save locally (storage unavailable).", "error");
      return;
    }
    try {
      for (const cls of classes) {
        const payload = normalizeClass(cls);
        if (remoteIds.has(cls.id)) {
          await updateClassDef(authToken, cls.id, payload);
        } else {
          await createClassDef(authToken, payload);
        }
      }
      await loadFromServer();
      setStatus("Saved to server.", "success");
    } catch (err) {
      if (isNetworkError(err)) {
        const ok = saveClasses(classes);
        if (ok) setStatus("Network error. Saved to localStorage instead.", "info");
        else setStatus("Network error and local storage unavailable.", "error");
      } else {
        setStatus(`Could not save: ${(err && err.message) || err}`, "error");
      }
    }
  }

  function onReset() {
    if (typeof confirm === "function" && !confirm("Discard local edits and reload the class catalog?")) return;
    resetClasses();
    selectedId = null;
    if (CLASS_DEF_CRUD_AVAILABLE && authToken) loadCatalog();
    else loadFromStorage();
  }

  function focusIssue(item) {
    const match = /^skills\[(\d+)\]\./.exec(item.field);
    if (match) {
      selectedSkillIndex = Number(match[1]);
      if (selected) syncPreview(selectedSkill, selected);
    }
  }

  function close() {
    classesEditorOpen.set(false);
  }

  const missingEndpoints = Object.entries(CLASS_ENDPOINT_STATUS)
    .filter(([, ok]) => !ok)
    .map(([name]) => name);
</script>

<style>
  .classes-overlay {
    position: fixed;
    inset: 0;
    z-index: 12060;
    background: rgba(6, 8, 12, 0.86);
    display: flex;
    align-items: stretch;
    justify-content: center;
    padding: 24px;
    animation: classesFadeIn 0.15s ease-out;
  }
  @keyframes classesFadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  .classes-panel {
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
  .classes-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 18px;
    border-bottom: 1px solid #232b39;
    background: rgba(255, 255, 255, 0.02);
  }
  .classes-title {
    font-size: 1.05rem;
    font-weight: 600;
    letter-spacing: 0.02em;
  }
  .classes-title i {
    color: #c084fc;
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
  .classes-close {
    background: transparent;
    border: none;
    color: #94a3b8;
    font-size: 1.5rem;
    line-height: 1;
    cursor: pointer;
  }
  .classes-close:hover { color: #e2e8f0; }

  .banner {
    padding: 8px 18px;
    font-size: 0.78rem;
    border-bottom: 1px solid #232b39;
    background: rgba(251, 191, 36, 0.06);
    color: #fcd34d;
  }
  .banner code {
    color: #fde68a;
    background: rgba(0, 0, 0, 0.3);
    border-radius: 3px;
    padding: 0 4px;
  }

  .classes-body {
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
  button.btn.primary { background: #4c1d95; border-color: #6d28d9; color: #ede9fe; }
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
  input:focus, select:focus, textarea:focus { outline: none; border-color: #7c3aed; }
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
  .grid5 { display: grid; grid-template-columns: repeat(5, 1fr); gap: 6px; }

  .class-row {
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
  .class-row:hover { background: rgba(255, 255, 255, 0.04); }
  .class-row.active { background: rgba(124, 58, 237, 0.16); border-color: #6d28d9; }
  .class-row .name { font-size: 0.85rem; font-weight: 600; }
  .class-row .meta { font-size: 0.68rem; color: #7f8ea3; }

  .section-title {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: #8b9bb4;
    margin: 14px 0 8px;
  }
  .section-title:first-child { margin-top: 0; }

  .skill-card {
    border: 1px solid #263041;
    border-radius: 8px;
    margin-bottom: 8px;
    background: rgba(255, 255, 255, 0.015);
    overflow: hidden;
  }
  .skill-card.selected { border-color: #6d28d9; }
  .skill-card.has-error { border-color: #b91c1c; }
  .skill-head {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 7px 9px;
    cursor: pointer;
    background: rgba(255, 255, 255, 0.02);
  }
  .skill-head .grow { flex: 1; min-width: 0; }
  .skill-head input, .skill-head select { padding: 4px 6px; }
  .skill-head .icon-btn {
    border: none;
    background: transparent;
    color: #94a3b8;
    cursor: pointer;
    font-size: 0.9rem;
    padding: 2px 4px;
  }
  .skill-head .icon-btn:hover { color: #e2e8f0; }
  .skill-body {
    padding: 10px;
    border-top: 1px solid #232b39;
    display: grid;
    gap: 8px;
  }
  .check-row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.76rem;
    color: #cbd5e1;
  }
  .check-row input { width: auto; }

  .issue {
    font-size: 0.72rem;
    border-radius: 5px;
    padding: 5px 7px;
    margin-bottom: 5px;
    cursor: pointer;
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
    background: rgba(124, 58, 237, 0.08);
  }
  .preview .tokens { font-size: 1.2rem; font-weight: 700; color: #e9d5ff; }
  .preview .notes { font-size: 0.72rem; color: #9fb3c8; margin-top: 5px; }
  .preview .notes div { margin-top: 2px; }

  .status {
    padding: 6px 18px;
    border-top: 1px solid #232b39;
    font-size: 0.74rem;
    color: #9fb3c8;
  }
  .status.success { color: #4ade80; }
  .status.error { color: #f87171; }

  @media (max-width: 980px) {
    .classes-body { grid-template-columns: 1fr; grid-template-rows: auto auto auto; }
    .col { overflow: visible; }
    .col.list, .col.check { border: none; }
  }
</style>

{#if $classesEditorOpen}
  <div class="classes-overlay" role="dialog" aria-label="Classes and Skills Editor">
    <div class="classes-panel">
      <div class="classes-header">
        <div class="classes-title"><i class="material-icons">auto_awesome</i>Classes &amp; Skills Editor</div>
        {#if CLASS_DEF_CRUD_AVAILABLE}
          <span class="badge ok">remote</span>
        {:else}
          <span class="badge local">localStorage</span>
        {/if}
        {#if counts.errors > 0}
          <span class="badge err">{counts.errors} error{counts.errors === 1 ? "" : "s"}</span>
        {/if}
        {#if counts.warnings > 0}
          <span class="badge warn">{counts.warnings} warn{counts.warnings === 1 ? "" : "s"}</span>
        {/if}
        <div class="spacer"></div>
        <button class="btn primary" type="button" on:click={onSave}>Save</button>
        <button class="classes-close" type="button" on:click={close} aria-label="Close editor">&times;</button>
      </div>

      {#if !CLASS_DEF_CRUD_AVAILABLE}
        <div class="banner">
          Class CRUD endpoints are pending. Edits are saved to localStorage and the demo fixture loads on first open.
          Missing: {missingEndpoints.map((name) => `${name}`).join(", ")} — see
          <code>src/api/classes.js</code>. Skill routes (<code>/api/skills</code>) already exist in the backend.
        </div>
      {/if}

      <div class="classes-body">
        <!-- Class list -->
        <div class="col list">
          <div class="toolbar">
            <button class="btn" type="button" on:click={onNewClass}>+ New</button>
            <button class="btn" type="button" on:click={onDuplicateClass} disabled={!selected}>Duplicate</button>
            <button class="btn danger" type="button" on:click={onDeleteClass} disabled={!selected}>Delete</button>
            <button class="btn ghost" type="button" on:click={onReset}>Reset demo</button>
          </div>
          <input type="text" placeholder="Search classes…" bind:value={search} />
          <div class="section-title">{filteredClasses.length} class{filteredClasses.length === 1 ? "" : "es"}</div>
          {#each filteredClasses as cls}
            <button
              type="button"
              class="class-row"
              class:active={cls.id === selectedId}
              on:click={() => selectClass(cls)}
            >
              <div class="name">{cls.name || "(unnamed)"}</div>
              <div class="meta">{cls.id} · {cls.school} · {cls.caster ? "caster" : "physical"}</div>
            </button>
          {/each}
        </div>

        <!-- Class + skill form -->
        <div class="col form">
          {#if !selected}
            <div class="issues-empty">No class selected. Create one to begin.</div>
          {:else}
            <div class="section-title">Class</div>
            <div class="grid2">
              <label class="field">
                <span class="field-label">Id</span>
                <input type="text" bind:value={selected.id} on:input={touch} />
              </label>
              <label class="field">
                <span class="field-label">Name</span>
                <input type="text" bind:value={selected.name} on:input={touch} />
              </label>
            </div>
            <label class="field">
              <span class="field-label">Description</span>
              <textarea bind:value={selected.description} on:input={touch}></textarea>
            </label>
            <div class="grid3">
              <label class="field">
                <span class="field-label">School</span>
                <select bind:value={selected.school} on:change={touch}>
                  {#each SCHOOLS as school}
                    <option value={school.id}>{school.name}</option>
                  {/each}
                </select>
              </label>
              <label class="field">
                <span class="field-label">Armor</span>
                <select bind:value={selected.armorType} on:change={touch}>
                  {#each ARMOR_TYPES as armor}
                    <option value={armor}>{armor}</option>
                  {/each}
                </select>
              </label>
              <label class="field">
                <span class="field-label">Combat</span>
                <select bind:value={selected.combatType} on:change={touch}>
                  {#each COMBAT_TYPES as combat}
                    <option value={combat}>{combat}</option>
                  {/each}
                </select>
              </label>
            </div>
            <div class="grid3">
              <label class="field">
                <span class="field-label">Primary</span>
                <select bind:value={selected.primary} on:change={touch}>
                  {#each PRIMARY_ATTRIBUTES as attr}
                    <option value={attr}>{attr}</option>
                  {/each}
                </select>
              </label>
              <label class="field">
                <span class="field-label">HP multiplier</span>
                <input type="number" step="0.05" min="0.1" bind:value={selected.hpMultiplier} on:input={touch} />
              </label>
              <label class="field">
                <span class="field-label">Hotbar cap</span>
                <input type="number" step="1" min="1" max="8" bind:value={selected.hotbarCap} on:input={touch} />
              </label>
            </div>
            <div class="grid2">
              <label class="field check-row">
                <input type="checkbox" bind:checked={selected.caster} on:change={touch} />
                <span>Caster (uses mana)</span>
              </label>
              <label class="field">
                <span class="field-label">Starting mana</span>
                <input type="number" step="1" min="0" bind:value={selected.mana} on:input={touch} />
              </label>
            </div>

            <div class="section-title">Attributes</div>
            <div class="grid5">
              {#each ATTRIBUTES as attr}
                <label class="field">
                  <span class="field-label">{attr.toUpperCase()}</span>
                  <input type="number" step="1" min="0" bind:value={selected.attributes[attr]} on:input={touch} />
                </label>
              {/each}
            </div>

            <div class="section-title">Base hotbar</div>
            <div class="grid3">
              {#each selected.skills as skill}
                <label class="check-row">
                  <input
                    type="checkbox"
                    checked={(selected.hotbar || []).includes(skill.id)}
                    on:change={() => toggleHotbar(skill.id)}
                  />
                  <span>{skill.name || skill.id}</span>
                </label>
              {/each}
            </div>

            <div class="section-title">Skills</div>
            <div class="toolbar">
              <button class="btn" type="button" on:click={onAddSkill}>+ Add skill</button>
            </div>

            {#each selected.skills as skill, index (index)}
              <div
                class="skill-card"
                class:selected={index === selectedSkillIndex}
                class:has-error={issuesForSkill(index).some((i) => i.level === "error")}
              >
                <div class="skill-head" on:click={() => selectSkill(index)} role="button" tabindex="0" on:keydown={() => {}}>
                  <span class="grow">
                    <input type="text" bind:value={skill.name} on:input={touch} on:click|stopPropagation placeholder="Skill name" />
                  </span>
                  <select bind:value={skill.effect} on:change={touch} on:click|stopPropagation>
                    {#each EFFECTS as effect}
                      <option value={effect}>{effect}</option>
                    {/each}
                  </select>
                  <input
                    type="number"
                    min="1"
                    style="width:56px"
                    bind:value={skill.level}
                    on:input={touch}
                    on:click|stopPropagation
                    title="Unlock level"
                  />
                  <button class="icon-btn" type="button" title="Move up" on:click|stopPropagation={() => onMoveSkill(index, -1)}>↑</button>
                  <button class="icon-btn" type="button" title="Move down" on:click|stopPropagation={() => onMoveSkill(index, 1)}>↓</button>
                  <button class="icon-btn" type="button" title="Remove" on:click|stopPropagation={() => onRemoveSkill(index)}>✕</button>
                </div>

                {#each issuesForSkill(index) as item}
                  <div class={`issue ${item.level}`} on:click={() => focusIssue(item)} role="button" tabindex="0" on:keydown={() => {}}>
                    <span class="field">{item.shortField}</span> — {item.msg}
                  </div>
                {/each}

                {#if index === selectedSkillIndex}
                  <div class="skill-body">
                    <div class="grid2">
                      <label class="field">
                        <span class="field-label">Skill id</span>
                        <input type="text" bind:value={skill.id} on:input={touch} />
                      </label>
                      <label class="field">
                        <span class="field-label">School override</span>
                        <select bind:value={skill.school} on:change={touch}>
                          <option value="">(inherit {selected.school})</option>
                          {#each SCHOOLS as school}
                            <option value={school.id}>{school.name}</option>
                          {/each}
                        </select>
                      </label>
                    </div>
                    <div class="grid3">
                      <label class="field">
                        <span class="field-label">Mana cost</span>
                        <input type="number" min="0" bind:value={skill.manaCost} on:input={touch} />
                      </label>
                      <label class="field">
                        <span class="field-label">Component</span>
                        <input type="text" list="component-list" bind:value={skill.component} on:input={touch} />
                      </label>
                      <label class="field">
                        <span class="field-label">Cooldown</span>
                        <input type="number" min="0" bind:value={skill.cooldown} on:input={touch} />
                      </label>
                    </div>
                    <div class="grid3">
                      <label class="field">
                        <span class="field-label">Multiplier</span>
                        <input type="number" step="0.05" min="0" bind:value={skill.multiplier} on:input={touch} />
                      </label>
                      <label class="field">
                        <span class="field-label">Duration (rounds)</span>
                        <input type="number" min="0" bind:value={skill.duration} on:input={touch} />
                      </label>
                      <label class="field">
                        <span class="field-label">Target</span>
                        <select bind:value={skill.target} on:change={touch}>
                          {#each TARGETS as target}
                            <option value={target}>{target}</option>
                          {/each}
                        </select>
                      </label>
                    </div>
                    <div class="grid2">
                      <label class="check-row">
                        <input type="checkbox" bind:checked={skill.oncePerFight} on:change={touch} />
                        <span>Once per fight</span>
                      </label>
                      <label class="check-row">
                        <input type="checkbox" bind:checked={skill.keepsSwing} on:change={touch} />
                        <span>Keeps swing</span>
                      </label>
                    </div>
                    <label class="field">
                      <span class="field-label">Tooltip</span>
                      <textarea bind:value={skill.tooltip} on:input={touch}></textarea>
                    </label>
                    <button class="btn primary" type="button" on:click={() => { syncPreview(skill, selected); }}>Preview this skill</button>
                  </div>
                {/if}
              </div>
            {/each}
          {/if}
        </div>

        <!-- Validation + preview -->
        <div class="col check">
          <div class="section-title">Balance checks</div>
          {#if !selected}
            <div class="issues-empty">—</div>
          {:else if issues.length === 0}
            <div class="issues-empty">No issues. This class is balanced.</div>
          {:else}
            {#each classIssues as item}
              <div class={`issue ${item.level}`} on:click={() => focusIssue(item)} role="button" tabindex="0" on:keydown={() => {}}>
                <span class="field">{item.field}</span> — {item.msg}
              </div>
            {/each}
          {/if}

          <div class="section-title">Preview</div>
          {#if !selectedSkill || !preview}
            <div class="issues-empty">Select a skill to preview its damage / heal.</div>
          {:else}
            <label class="field">
              <span class="field-label">Preview level</span>
              <input type="number" min="1" bind:value={previewLevel} on:input={touch} />
            </label>
            <div class="grid2">
              <label class="field">
                <span class="field-label">Primary value</span>
                <input type="number" min="0" bind:value={previewPrimary} on:input={touch} />
              </label>
              <label class="field">
                <span class="field-label">Max HP</span>
                <input type="number" min="1" bind:value={previewMaxHp} on:input={touch} />
              </label>
            </div>
            <div class="preview">
              <div class="tokens">{preview.tokens}</div>
              <div class="notes">
                {#each preview.notes as note}
                  <div>{note}</div>
                {/each}
                <div>Effect: {selectedSkill.effect} · Target: {selectedSkill.target}</div>
              </div>
            </div>
          {/if}
        </div>
      </div>

      <div class={`status ${statusKind}`}>{statusMsg}</div>
    </div>
  </div>

  <datalist id="component-list">
    {#each COMPONENTS as component}
      <option value={component}></option>
    {/each}
  </datalist>
{/if}
