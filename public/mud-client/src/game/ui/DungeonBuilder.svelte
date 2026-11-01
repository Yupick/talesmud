<script>
  // Dungeon Builder (FASE 5.1, initial cut; Library added in Batch 19).
  // Visual grid-based editor following NpcFactory pattern.
  //
  // Library persistence (Batch 19): localStorage is the source of truth via
  // `../dungeonLibrary.js`; the `/api/dungeons` CRUD routes
  // (`../../api/dungeons.js`) are used best-effort on top. Missing routes or a
  // missing creator role degrade to localStorage with a visible notice.
  import { onMount } from "svelte";
  import { dungeonBuilderOpen } from "../uiChrome.js";
  import {
    createGrid,
    CELL_TYPES,
    setCell,
    paintCell,
    addRoom,
    addCorridor,
    autoValidate,
    addEncounter,
    addLootTable,
    serialize,
    deserialize,
    exportDefinition,
  } from "../dungeonBuilder.js";
  import {
    buildDungeonPayload,
    gridFromEntry,
    normalizeListRows,
    nextLocalId,
    upsertLocalEntry,
    removeLocalEntry,
    loadLocalEntries,
    saveLocalEntries,
  } from "../dungeonLibrary.js";
  import {
    listDungeons,
    getDungeon,
    createDungeon,
    updateDungeon,
    deleteDungeon,
    isDungeonRouteError,
  } from "../../api/dungeons.js";

  export let authToken = "";

  let grid = createGrid(16, 16);
  let tool = "empty"; // empty/room/corridor/door/entrance/exit/paint
  let roomW = 3;
  let roomH = 3;
  let issues = [];
  let showTesting = false;
  let testingPath = [];
  let lootTableName = "basic";
  let lootEntries = [{ itemId: "", chance: 50, qty: 1 }];

  // Library state (Batch 19)
  let entries = []; // local library (localStorage-backed)
  let remoteRows = []; // metadata from GET /api/dungeons
  let currentId = null; // id of the loaded entry (local_* until first server save)
  let libName = "";
  let libDescription = "";
  let libStatusMsg = "";
  let libStatusKind = "info";
  let libBusy = false;
  let serverNotice = ""; // degradation banner (routes missing / no creator role)

  const cellSize = 24;

  $: issues = autoValidate(grid);
  // Merged view: local entries (cached definitions) + server-only rows.
  $: remoteIds = new Set(remoteRows.map((r) => r.id));
  $: libraryRows = [
    ...entries.map((e) => ({
      id: e.id,
      name: e.name,
      onServer: remoteIds.has(e.id),
      entry: e,
    })),
    ...remoteRows
      .filter((r) => !entries.some((e) => e.id === r.id))
      .map((r) => ({
        id: r.id,
        name: r.name,
        onServer: true,
        entry: { id: r.id, name: r.name, description: r.description, definition: null },
      })),
  ].sort((a, b) => a.name.localeCompare(b.name));
  // Reload the local library on every panel open (edge-triggered so an empty
  // library cannot loop the reactive statement).
  let libWasOpen = false;
  $: {
    if ($dungeonBuilderOpen && !libWasOpen) {
      libWasOpen = true;
      entries = loadLocalEntries();
    } else if (!$dungeonBuilderOpen) {
      libWasOpen = false;
    }
  }

  function onCellClick(x, y, e) {
    if (tool === "room") {
      addRoom(grid, { x, y, w: roomW, h: roomH });
      grid = grid;
      issues = autoValidate(grid);
      return;
    }
    if (tool === "corridor") {
      // simple click adds point - but for quick drawing, just set
      setCell(grid, x, y, "corridor");
      grid = grid;
      issues = autoValidate(grid);
      return;
    }
    if (tool === "paint") {
      paintCell(grid, x, y, "room");
      grid = grid;
      issues = autoValidate(grid);
      return;
    }
    setCell(grid, x, y, tool);
    grid = grid;
    issues = autoValidate(grid);
  }

  function clearGrid() {
    grid = createGrid(grid.width, grid.height);
    issues = autoValidate(grid);
  }

  function resizeGrid() {
    grid = createGrid(grid.width, grid.height);
    issues = autoValidate(grid);
  }

  function runValidate() {
    issues = autoValidate(grid);
  }

  function toggleTesting() {
    showTesting = !showTesting;
    if (showTesting) {
      // find first entrance->exit path
      const ents = grid.entrances;
      const exs = grid.exits;
      testingPath = [];
      if (ents.length > 0 && exs.length > 0) {
        for (const ent of ents) {
          for (const ex of exs) {
            const p = requirePath(ent, ex);
            if (p && p.length > 0) {
              testingPath = p;
              break;
            }
          }
          if (testingPath.length > 0) break;
        }
      }
    }
  }

  function requirePath(start, end) {
    const { findPathBFS } = awaitImport();
    return findPathBFS(grid, start, end);
  }

  async function awaitImport() {
    const mod = await import("../dungeonBuilder.js");
    return mod;
  }

  let findPathBFSMod = null;
  onMount(async () => {
    const mod = await import("../dungeonBuilder.js");
    findPathBFSMod = mod.findPathBFS;
    entries = loadLocalEntries();
  });

  function testPath() {
    if (!findPathBFSMod) return;
    const ents = grid.entrances;
    const exs = grid.exits;
    if (ents.length > 0 && exs.length > 0) {
      for (const ent of ents) {
        for (const ex of exs) {
          const p = findPathBFSMod(grid, ent, ex);
          if (p && p.length > 0) {
            testingPath = p;
            return;
          }
        }
      }
    }
    testingPath = [];
  }

  function exportJSON() {
    const blob = new Blob([serialize(grid)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "dungeon.json";
    a.click();
    URL.revokeObjectURL(url);
  }

  // -------------------------------------------------------------------------
  // Library (Batch 19): localStorage first, /api/dungeons best-effort on top
  // -------------------------------------------------------------------------

  function setLibStatus(msg, kind = "info") {
    libStatusMsg = msg;
    libStatusKind = kind;
  }

  function markServerUnavailable(msg) {
    serverNotice = msg;
    setLibStatus(msg, "warn");
  }

  function mergeServerRow(row) {
    const norm = normalizeListRows([row])[0];
    if (!norm) return;
    remoteRows = [...remoteRows.filter((r) => r.id !== norm.id), norm];
  }

  /** Best-effort list of server dungeons (manual "Refresh" action). */
  async function onRefreshLibrary() {
    entries = loadLocalEntries();
    if (!authToken) {
      remoteRows = [];
      setLibStatus("No session token — showing the local library only.", "info");
      return;
    }
    libBusy = true;
    try {
      const rows = await listDungeons(authToken, { limit: 100 });
      remoteRows = normalizeListRows(rows);
      serverNotice = "";
      setLibStatus(`Loaded ${remoteRows.length} dungeon(s) from the server.`, "success");
    } catch (err) {
      remoteRows = [];
      const msg = (err && err.message) || "list failed";
      if (isDungeonRouteError(err)) {
        markServerUnavailable(`Server library unavailable (${msg}). Using localStorage only.`);
      } else {
        setLibStatus(`Could not reach the server (${msg}).`, "error");
      }
    }
    libBusy = false;
  }

  /** Load a library row (local cache or full fetch from the server). */
  async function onLoadEntry(row) {
    if (!row) return;
    libBusy = true;
    try {
      let entry = row.entry;
      if (!entry || !entry.definition) {
        // Server-only row: pull the full definition.
        if (!authToken) {
          setLibStatus("No session token — cannot load a server dungeon.", "error");
          libBusy = false;
          return;
        }
        const full = await getDungeon(authToken, row.id);
        const meta = normalizeListRows([full])[0] || { id: row.id };
        entry = { ...meta, definition: full && full.definition };
        // Cache it locally so the next load works offline, and remember the
        // server id so the next Save updates (PUT) instead of duplicating (POST).
        const next = upsertLocalEntry(entries, entry);
        if (saveLocalEntries(next)) entries = next;
        mergeServerRow(meta);
      }
      const loaded = gridFromEntry(entry);
      if (!loaded) {
        setLibStatus("That dungeon has no readable definition.", "error");
        libBusy = false;
        return;
      }
      grid = loaded;
      issues = autoValidate(grid);
      testingPath = [];
      currentId = row.id;
      libName = entry.name || row.name || "";
      libDescription = entry.description || "";
      setLibStatus(`Loaded "${libName || row.id}".`, "success");
    } catch (err) {
      const msg = (err && err.message) || "load failed";
      if (isDungeonRouteError(err)) {
        markServerUnavailable(`Server library unavailable (${msg}).`);
      } else {
        setLibStatus(`Load failed (${msg}).`, "error");
      }
    }
    libBusy = false;
  }

  /** Save the current grid: localStorage always, server when possible. */
  async function onSaveEntry() {
    const payload = buildDungeonPayload({
      name: libName,
      description: libDescription,
      grid,
    });
    if (!payload) {
      setLibStatus("Give the dungeon a name before saving.", "error");
      return;
    }
    libBusy = true;
    // 1) Local write — the source of truth, works with no backend at all.
    const id = currentId || nextLocalId(entries);
    const stored = upsertLocalEntry(entries, {
      id,
      name: payload.name,
      description: payload.description,
      author: payload.author,
      tags: payload.tags,
      updatedAt: new Date().toISOString(),
      definition: payload.definition,
    });
    if (!saveLocalEntries(stored)) {
      setLibStatus("Could not write to localStorage (storage unavailable).", "error");
      libBusy = false;
      return;
    }
    entries = stored;
    currentId = id;

    // 2) Best-effort remote write.
    if (!authToken) {
      setLibStatus("Saved to localStorage only (no session token).", "info");
      libBusy = false;
      return;
    }
    try {
      if (remoteIds.has(id)) {
        await updateDungeon(authToken, id, payload);
        mergeServerRow({ id, name: payload.name, description: payload.description });
        setLibStatus(`Saved locally and updated on the server (id ${id}).`, "success");
      } else {
        const created = await createDungeon(authToken, payload);
        const remoteId = created && created.id ? String(created.id) : null;
        if (remoteId && remoteId !== id) {
          // Rename the local entry to the server id so both sides share it.
          const base = entries.find((e) => e.id === id);
          const renamed = upsertLocalEntry(removeLocalEntry(entries, id), {
            ...base,
            id: remoteId,
          });
          if (saveLocalEntries(renamed)) entries = renamed;
          currentId = remoteId;
          mergeServerRow({ ...(created || {}), id: remoteId, name: payload.name });
          setLibStatus(`Saved locally and on the server (id ${remoteId}).`, "success");
        } else {
          mergeServerRow(created || { id, name: payload.name });
          setLibStatus(`Saved locally and on the server (id ${id}).`, "success");
        }
      }
    } catch (err) {
      const msg = (err && err.message) || "server save failed";
      if (isDungeonRouteError(err)) {
        markServerUnavailable(
          `Server library unavailable (${msg}). Saved to localStorage only.`
        );
      } else {
        setLibStatus(`Saved locally; server save failed (${msg}).`, "error");
      }
    }
    libBusy = false;
  }

  /** Delete a library row from localStorage and, when present, the server. */
  async function onDeleteEntry(row) {
    if (!row) return;
    if (typeof confirm === "function" && !confirm(`Delete "${row.name || row.id}"?`)) {
      return;
    }
    libBusy = true;
    const next = removeLocalEntry(entries, row.id);
    if (saveLocalEntries(next)) entries = next;
    if (currentId === row.id) currentId = null;

    if (!row.onServer) {
      setLibStatus("Removed from localStorage.", "success");
      libBusy = false;
      return;
    }
    if (!authToken) {
      setLibStatus("Removed from localStorage; the server copy was kept (no token).", "info");
      libBusy = false;
      return;
    }
    try {
      await deleteDungeon(authToken, row.id);
      remoteRows = remoteRows.filter((r) => r.id !== row.id);
      setLibStatus("Deleted from the server and localStorage.", "success");
    } catch (err) {
      const msg = (err && err.message) || "server delete failed";
      if (isDungeonRouteError(err)) {
        markServerUnavailable(
          `Server library unavailable (${msg}). Removed from localStorage only.`
        );
      } else {
        setLibStatus(`Removed locally; server delete failed (${msg}).`, "error");
      }
    }
    libBusy = false;
  }

  function onNewEntry() {
    currentId = null;
    libName = "";
    libDescription = "";
    setLibStatus("New entry — edit the grid, name it and press Save.", "info");
  }

  function addEncounterAt(x, y) {
    addEncounter(grid, { x, y, npcTemplateId: "", count: 1, level: 1 });
    grid = grid;
  }

  function addLootTableToGrid() {
    addLootTable(grid, { name: lootTableName, entries: lootEntries });
    grid = grid;
    lootTableName = `table_${grid.lootTables.length}`;
    lootEntries = [{ itemId: "", chance: 50, qty: 1 }];
  }
</script>
{#if $dungeonBuilderOpen}
  <div class="panel-backdrop">
    <div class="panel">
      <div class="header">
        <h2>Dungeon Builder</h2>
        <button on:click={() => dungeonBuilderOpen.set(false)}>Close</button>
      </div>
      <div class="toolbar">
        <label>Tool:</label>
        <select bind:value={tool}>
          {#each CELL_TYPES as t}
            <option value={t}>{t}</option>
          {/each}
          <option value="paint">paint</option>
        </select>
        <label>Room W:</label>
        <input type="number" bind:value={roomW} min="1" max="10" />
        <label>H:</label>
        <input type="number" bind:value={roomH} min="1" max="10" />
        <label>Grid:</label>
        <input type="number" bind:value={grid.width} min="5" max="40" on:change={resizeGrid} />
        <input type="number" bind:value={grid.height} min="5" max="40" on:change={resizeGrid} />
        <button on:click={clearGrid}>Clear</button>
        <button on:click={runValidate}>Validate</button>
        <button on:click={toggleTesting}>{showTesting ? 'Hide Testing' : 'Testing Mode'}</button>
        <button on:click={testPath}>Show Path</button>
        <button on:click={exportJSON}>Export JSON</button>
      </div>
      <div class="content">
        <div class="grid-container">
          <div
            class="grid"
            style="grid-template-columns: repeat({grid.width}, {cellSize}px); grid-template-rows: repeat({grid.height}, {cellSize}px);"
          >
            {#each grid.cells as row, y}
              {#each row as cell, x}
                <div
                  class="cell {cell.type}"
                  on:click={(e) => onCellClick(x, y, e)}
                  class:path={testingPath.some((p) => p.x === x && p.y === y)}
                />
              {/each}
            {/each}
          </div>
        </div>
        <div class="sidebar">
          <div class="library">
            <h3>Library</h3>
            <div class="lib-form">
              <input placeholder="name" bind:value={libName} disabled={libBusy} />
              <input placeholder="description" bind:value={libDescription} disabled={libBusy} />
              <button on:click={onSaveEntry} disabled={libBusy}>Save</button>
              <button on:click={onRefreshLibrary} disabled={libBusy}>Refresh</button>
              <button on:click={onNewEntry} disabled={libBusy}>New</button>
            </div>
            {#if serverNotice}
              <div class="notice">{serverNotice}</div>
            {/if}
            {#if libStatusMsg}
              <div class="lib-msg {libStatusKind}">{libStatusMsg}</div>
            {/if}
            <div class="lib-list">
              {#each libraryRows as row (row.id)}
                <div class="lib-row" class:active={row.id === currentId}>
                  <button class="lib-load" on:click={() => onLoadEntry(row)} disabled={libBusy}>
                    <span class="lib-name">{row.name}</span>
                    <span class="badge {row.onServer ? 'remote' : 'local'}">
                      {row.onServer ? 'server' : 'local'}
                    </span>
                  </button>
                  <button class="lib-del" title="Delete" on:click={() => onDeleteEntry(row)} disabled={libBusy}>x</button>
                </div>
              {:else}
                <div class="lib-empty">
                  No dungeons yet — name the current grid and press Save. Saves always land in
                  localStorage; the server copy is best-effort.
                </div>
              {/each}
            </div>
          </div>
          <div class="issues">
            <h3>Issues ({issues.length})</h3>
            {#if issues.length === 0}
              <div class="ok">No issues</div>
            {/if}
            {#each issues as issue}
              <div class="issue {issue.level}">{issue.msg}</div>
            {/each}
          </div>
          <div class="rooms">
            <h3>Rooms ({grid.rooms.length})</h3>
            {#each grid.rooms as r}
              <div>{r.id}: {r.x},{r.y} {r.w}x{r.h}</div>
            {/each}
          </div>
          <div class="encounters">
            <h3>Encounters ({grid.encounters.length})</h3>
            {#each grid.encounters as enc}
              <div>{enc.id} at {enc.cell.x},{enc.cell.y}: {enc.npcTemplateId} x{enc.count}</div>
            {/each}
          </div>
          <div class="loot">
            <h3>Loot Tables ({grid.lootTables.length})</h3>
            {#each grid.lootTables as lt}
              <div>{lt.name} ({lt.entries.length} entries)</div>
            {/each}
            <div class="loot-add">
              <input placeholder="name" bind:value={lootTableName} />
              <button on:click={addLootTableToGrid}>Add Table</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .panel-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 1000;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .panel {
    background: #1a1a1a;
    color: #fff;
    width: 90vw;
    height: 90vh;
    border-radius: 8px;
    padding: 1rem;
    display: flex;
    flex-direction: column;
  }
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }
  .toolbar {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    flex-wrap: wrap;
    margin-bottom: 1rem;
  }
  .content {
    display: flex;
    flex: 1;
    overflow: hidden;
    gap: 1rem;
  }
  .grid-container {
    flex: 1;
    overflow: auto;
  }
  .grid {
    display: grid;
    gap: 1px;
  }
  .cell {
    width: 24px;
    height: 24px;
    border: 1px solid #333;
    cursor: pointer;
    background: #222;
  }
  .cell.empty { background: #1a1a1a; }
  .cell.room { background: #4a90e2; }
  .cell.corridor { background: #9b59b6; }
  .cell.door { background: #f39c12; }
  .cell.entrance { background: #2ecc71; }
  .cell.exit { background: #e74c3c; }
  .cell.path { box-shadow: inset 0 0 0 2px #fff; }
  .sidebar {
    width: 300px;
    overflow-y: auto;
  }
  .library {
    border: 1px solid #333;
    border-radius: 4px;
    padding: 0.5rem;
    margin-bottom: 0.75rem;
  }
  .library h3 {
    margin: 0 0 0.5rem;
  }
  .lib-form {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
    margin-bottom: 0.5rem;
  }
  .lib-form input {
    flex: 1 1 100%;
    min-width: 0;
  }
  .lib-form button {
    flex: 1 1 auto;
  }
  .lib-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-height: 200px;
    overflow-y: auto;
  }
  .lib-row {
    display: flex;
    gap: 2px;
    align-items: stretch;
  }
  .lib-row.active .lib-load {
    outline: 1px solid #4a90e2;
  }
  .lib-load {
    flex: 1;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    text-align: left;
  }
  .lib-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .badge {
    font-size: 0.7rem;
    border-radius: 3px;
    padding: 0 4px;
    flex: none;
  }
  .badge.local {
    background: #2c3e50;
    color: #95a5a6;
  }
  .badge.remote {
    background: #1e3a2f;
    color: #2ecc71;
  }
  .lib-empty {
    font-size: 0.8rem;
    color: #999;
  }
  .lib-msg {
    font-size: 0.8rem;
    margin-bottom: 0.4rem;
  }
  .lib-msg.info { color: #95a5a6; }
  .lib-msg.success { color: #2ecc71; }
  .lib-msg.warn { color: #f39c12; }
  .lib-msg.error { color: #e74c3c; }
  .notice {
    background: #3a2f1b;
    color: #f39c12;
    border: 1px solid #7a5b1e;
    border-radius: 4px;
    padding: 0.3rem 0.4rem;
    font-size: 0.78rem;
    margin-bottom: 0.4rem;
  }
  .issue.error { color: #e74c3c; }
  .issue.warn { color: #f39c12; }
  .ok { color: #2ecc71; }
</style>
