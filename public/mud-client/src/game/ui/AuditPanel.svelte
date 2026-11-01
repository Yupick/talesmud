<script>
  // Audit Log viewer (Creator Phase 5.4).
  //
  // Opened from the game chrome (`auditOpen`, shortcut A) and from the account
  // menu for creator/admin users. Reads the recent window from
  // `GET /api/audit` (creator-gated) and does every filter, the free-text
  // search, pagination and CSV export in the browser — the backend exposes no
  // server-side filtering or export for this endpoint. See `../../api/audit.js`
  // for the endpoint contract and `../auditView.js` for the pure helpers.
  import { onDestroy } from "svelte";
  import { auditOpen } from "../uiChrome.js";
  import { listAudit, AUDIT_MAX_LIMIT, AUDIT_ENDPOINT_STATUS } from "../../api/audit.js";
  import {
    DEFAULT_PAGE_SIZE,
    filterEvents,
    uniqueActions,
    uniqueEntityTypes,
    paginate,
    toCSV,
    summarizeEvent,
  } from "../auditView.js";

  export let authToken = "";

  const SEARCH_DEBOUNCE_MS = 250;

  let events = [];
  let loading = false;
  let loaded = false;
  let error = "";

  // Filters
  let actionFilter = "";
  let actorFilter = "";
  let entityFilter = "";
  let fromDate = "";
  let toDate = "";

  // Search (debounced into `query`)
  let searchInput = "";
  let query = "";
  let searchTimer = null;

  let page = 1;
  let pageSize = DEFAULT_PAGE_SIZE;

  function close() {
    auditOpen.set(false);
  }

  async function load() {
    loading = true;
    error = "";
    try {
      const rows = await listAudit(authToken, { limit: AUDIT_MAX_LIMIT });
      events = Array.isArray(rows) ? rows : [];
      loaded = true;
    } catch (err) {
      error = (err && err.message) || "Failed to load the audit log.";
    } finally {
      loading = false;
    }
  }

  function retry() {
    load();
  }

  // Load the first time the overlay opens; keep cached rows afterwards.
  $: if ($auditOpen && !loaded && !loading) load();

  // Debounce the free-text box into the criteria the filter actually uses.
  $: {
    const value = searchInput;
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      query = value;
    }, SEARCH_DEBOUNCE_MS);
  }

  onDestroy(() => {
    if (searchTimer) clearTimeout(searchTimer);
  });

  // Any criteria change restarts at page 1.
  $: {
    actionFilter;
    actorFilter;
    entityFilter;
    fromDate;
    toDate;
    query;
    page = 1;
  }

  $: actionOptions = uniqueActions(events);
  $: entityOptions = uniqueEntityTypes(events);
  $: filtered = filterEvents(events, {
    action: actionFilter,
    actor: actorFilter,
    entity: entityFilter,
    from: fromDate,
    to: toDate,
    query,
  });
  $: pageData = paginate(filtered, page, pageSize);
  $: activeFilters = !!(actionFilter || actorFilter || entityFilter || fromDate || toDate || query);

  function clearFilters() {
    actionFilter = "";
    actorFilter = "";
    entityFilter = "";
    fromDate = "";
    toDate = "";
    searchInput = "";
    query = "";
  }

  function goTo(next) {
    if (next < 1 || next > pageData.pages) return;
    page = next;
  }

  function changePageSize(event) {
    const next = Number(event.target.value);
    pageSize = next > 0 ? next : DEFAULT_PAGE_SIZE;
    page = 1;
  }

  function formatTime(value) {
    const d = new Date(value);
    return Number.isFinite(d.getTime()) ? d.toLocaleString() : String(value || "");
  }

  function exportCSV() {
    const csv = toCSV(filtered);
    const stamp = new Date().toISOString().slice(0, 10);
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `audit-log-${stamp}.csv`;
    link.rel = "noopener";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }
</script>

{#if $auditOpen}
  <div class="audit-overlay" role="dialog" aria-modal="true" aria-label="Audit Log">
    <div class="audit-panel">
      <div class="audit-header">
        <div class="audit-title"><i class="material-icons">history</i>Audit Log</div>
        {#if AUDIT_ENDPOINT_STATUS.listAudit}
          <span class="badge ok">/api/audit</span>
        {/if}
        {#if !AUDIT_ENDPOINT_STATUS.exportAudit}
          <span class="badge local">CSV in browser</span>
        {/if}
        <span class="badge">{pageData.total} event{pageData.total === 1 ? "" : "s"}</span>
        <div class="spacer"></div>
        <button
          class="btn primary"
          type="button"
          on:click={exportCSV}
          disabled={loading || filtered.length === 0}
          title={AUDIT_ENDPOINT_STATUS.exportAudit ? "Export from server" : "Export the filtered rows as CSV"}
        >
          <i class="material-icons">download</i>Export CSV
        </button>
        <button class="audit-close" type="button" on:click={close} aria-label="Close audit log">&times;</button>
      </div>

      <div class="audit-filters">
        <label class="field">
          <span>Action</span>
          <select bind:value={actionFilter} aria-label="Filter by action">
            <option value="">All actions</option>
            {#each actionOptions as action}
              <option value={action}>{action}</option>
            {/each}
          </select>
        </label>

        <label class="field">
          <span>Entity</span>
          <input
            type="text"
            list="audit-entity-types"
            placeholder="type or id…"
            bind:value={entityFilter}
            aria-label="Filter by entity"
          />
          <datalist id="audit-entity-types">
            {#each entityOptions as type}
              <option value={type}></option>
            {/each}
          </datalist>
        </label>

        <label class="field">
          <span>Actor</span>
          <input type="text" placeholder="name or id…" bind:value={actorFilter} aria-label="Filter by actor" />
        </label>

        <label class="field">
          <span>From</span>
          <input type="date" bind:value={fromDate} aria-label="Filter from date" />
        </label>

        <label class="field">
          <span>To</span>
          <input type="date" bind:value={toDate} aria-label="Filter to date" />
        </label>

        <label class="field grow">
          <span>Search</span>
          <input
            type="search"
            placeholder="search actor, action, detail…"
            bind:value={searchInput}
            aria-label="Search audit events"
          />
        </label>

        <button class="btn ghost" type="button" on:click={clearFilters} disabled={!activeFilters}>Clear</button>
      </div>

      <div class="audit-body">
        {#if loading}
          <div class="state loading" role="status">
            <div class="spinner" aria-hidden="true"></div>
            <span>Loading audit events…</span>
          </div>
        {:else if error}
          <div class="state error" role="alert">
            <i class="material-icons">error_outline</i>
            <span>{error}</span>
            <button class="btn" type="button" on:click={retry}>Retry</button>
          </div>
        {:else if filtered.length === 0}
          <div class="state empty" role="status">
            <i class="material-icons">inbox</i>
            <span>{events.length === 0 ? "No audit events yet." : "No events match these filters."}</span>
            {#if activeFilters}
              <button class="btn ghost" type="button" on:click={clearFilters}>Clear filters</button>
            {/if}
          </div>
        {:else}
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Actor</th>
                  <th>Action</th>
                  <th>Entity</th>
                  <th>Detail</th>
                </tr>
              </thead>
              <tbody>
                {#each pageData.items as event (event.id || `${event.time}-${event.action}-${event.entityId}`)}
                  <tr>
                    <td class="time">{formatTime(event.time)}</td>
                    <td class="actor">{event.actorName || event.actorUserId || "—"}</td>
                    <td><span class="tag action">{event.action || "—"}</span></td>
                    <td class="entity">
                      <span class="tag source">{event.entityType || "—"}</span>
                      <span class="entity-id">{event.entityId || ""}</span>
                    </td>
                    <td class="detail" title={summarizeEvent(event)}>{summarizeEvent(event)}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>

      <div class="audit-footer">
        <label class="page-size">
          Rows
          <select value={String(pageSize)} on:change={changePageSize} aria-label="Rows per page">
            <option value="10">10</option>
            <option value="25">25</option>
            <option value="50">50</option>
            <option value="100">100</option>
          </select>
        </label>
        <div class="spacer"></div>
        <button class="btn" type="button" on:click={() => goTo(pageData.page - 1)} disabled={!pageData.hasPrev}>Prev</button>
        <span class="page-label">Page {pageData.page} / {pageData.pages}</span>
        <button class="btn" type="button" on:click={() => goTo(pageData.page + 1)} disabled={!pageData.hasNext}>Next</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .audit-overlay {
    position: fixed;
    inset: 0;
    z-index: 12070;
    background: rgba(6, 8, 12, 0.86);
    display: flex;
    align-items: stretch;
    justify-content: center;
    padding: 24px;
    animation: auditFadeIn 0.15s ease-out;
  }
  @keyframes auditFadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  .audit-panel {
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
  .audit-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 18px;
    border-bottom: 1px solid #232b39;
    background: rgba(255, 255, 255, 0.02);
  }
  .audit-title {
    font-size: 1.05rem;
    font-weight: 600;
    letter-spacing: 0.02em;
  }
  .audit-title i {
    color: #60a5fa;
    margin-right: 8px;
    vertical-align: middle;
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
  .badge.ok { border-color: #15803d; color: #4ade80; }
  .badge.local { border-color: #b45309; color: #fbbf24; }
  .spacer { flex: 1; }
  .audit-close {
    background: transparent;
    border: none;
    color: #94a3b8;
    font-size: 1.4rem;
    line-height: 1;
    cursor: pointer;
  }
  .audit-close:hover { color: #e2e8f0; }

  .btn {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    background: #1f2937;
    color: #e2e8f0;
    border: 1px solid #374151;
    border-radius: 7px;
    padding: 6px 11px;
    font-size: 0.78rem;
    cursor: pointer;
  }
  .btn:hover:not(:disabled) { background: #273244; }
  .btn:disabled { opacity: 0.45; cursor: not-allowed; }
  .btn i { font-size: 1rem; }
  .btn.primary {
    background: rgba(37, 99, 235, 0.85);
    border-color: #1d4ed8;
    color: #eff6ff;
  }
  .btn.primary:hover:not(:disabled) { background: #2563eb; }
  .btn.ghost { background: transparent; }

  .audit-filters {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 10px;
    padding: 12px 18px;
    border-bottom: 1px solid #232b39;
    background: rgba(255, 255, 255, 0.01);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: #8b9bb4;
  }
  .field.grow { flex: 1; min-width: 180px; }
  .field input,
  .field select {
    background: #0d1117;
    border: 1px solid #303a4a;
    border-radius: 6px;
    color: #e2e8f0;
    padding: 6px 8px;
    font-size: 0.82rem;
    text-transform: none;
    letter-spacing: normal;
  }
  .field input:focus,
  .field select:focus { outline: none; border-color: #2563eb; }

  .audit-body {
    flex: 1;
    min-height: 180px;
    overflow: auto;
    position: relative;
  }

  .state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    min-height: 200px;
    padding: 32px;
    color: #93a3b8;
    text-align: center;
  }
  .state i { font-size: 2rem; opacity: 0.7; }
  .state.error { color: #fca5a5; }
  .state.error i { color: #ef4444; }
  .spinner {
    width: 26px;
    height: 26px;
    border-radius: 50%;
    border: 3px solid rgba(96, 165, 250, 0.25);
    border-top-color: #60a5fa;
    animation: auditSpin 0.8s linear infinite;
  }
  @keyframes auditSpin {
    to { transform: rotate(360deg); }
  }

  .table-wrap { overflow: auto; }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }
  thead th {
    position: sticky;
    top: 0;
    z-index: 1;
    text-align: left;
    padding: 9px 12px;
    background: #131923;
    border-bottom: 1px solid #2b3444;
    color: #9fb3c8;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  tbody td {
    padding: 8px 12px;
    border-bottom: 1px solid #1c2430;
    vertical-align: top;
  }
  tbody tr:hover { background: rgba(255, 255, 255, 0.025); }
  td.time { white-space: nowrap; color: #a7b6c9; font-variant-numeric: tabular-nums; }
  td.actor { white-space: nowrap; font-weight: 600; color: #dbe4f0; }
  td.entity { white-space: nowrap; }
  td.detail {
    max-width: 460px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: #b9c6d6;
  }
  .tag {
    display: inline-block;
    border-radius: 5px;
    padding: 1px 7px;
    font-size: 0.72rem;
    border: 1px solid #374151;
    background: rgba(255, 255, 255, 0.03);
  }
  .tag.action { color: #93c5fd; border-color: #1e40af; }
  .tag.source { color: #a7f3d0; border-color: #065f46; }
  .entity-id { margin-left: 6px; color: #8b9bb4; }

  .audit-footer {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 18px;
    border-top: 1px solid #232b39;
    background: rgba(255, 255, 255, 0.02);
    font-size: 0.8rem;
  }
  .page-size {
    display: flex;
    align-items: center;
    gap: 6px;
    color: #93a3b8;
  }
  .page-size select {
    background: #0d1117;
    border: 1px solid #303a4a;
    border-radius: 6px;
    color: #e2e8f0;
    padding: 4px 6px;
  }
  .page-label { color: #a7b6c9; font-variant-numeric: tabular-nums; }

  @media (max-width: 720px) {
    .audit-overlay { padding: 8px; }
    td.detail { max-width: 180px; }
  }
</style>
