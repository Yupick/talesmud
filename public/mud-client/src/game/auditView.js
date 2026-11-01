// Pure helpers for the Audit Log viewer (Creator Phase 5.4).
//
// The backend `GET /api/audit` (handler.ListAudit) only understands
// `entityType`, `entityId` and `limit` query parameters. Everything else that
// the viewer offers — action/actor/date filters, free-text search, pagination
// and CSV export — is applied client-side on the already-fetched rows.
//
// Keeping the math here (instead of inside the Svelte component) makes it
// testable with plain `node --test`.

/** Rows per page by default. */
export const DEFAULT_PAGE_SIZE = 25;

/** Column order used by `toCSV`, also handy for a header row in the UI. */
export const CSV_COLUMNS = [
  { key: "time", label: "time" },
  { key: "actor", label: "actor" },
  { key: "action", label: "action" },
  { key: "entityType", label: "entityType" },
  { key: "entityId", label: "entityId" },
  { key: "source", label: "source" },
  { key: "summary", label: "summary" },
];

function text(value) {
  return value === undefined || value === null ? "" : String(value);
}

/** The display label for an audit row: explicit summary, else action+entity. */
export function summarizeEvent(event) {
  if (!event) return "";
  if (text(event.summary).trim()) return text(event.summary);
  return [event.action, event.entityType, event.entityId].map(text).filter(Boolean).join(" ");
}

/** Milliseconds for an audit row's `time`, or 0 when unparsable. */
export function eventTimestamp(event) {
  const raw = event && event.time;
  if (raw instanceof Date) {
    const t = raw.getTime();
    return Number.isFinite(t) ? t : 0;
  }
  const t = Date.parse(text(raw));
  return Number.isFinite(t) ? t : 0;
}

/** Start of a `YYYY-MM-DD` day in local time, or null when empty/invalid. */
export function dayStart(value) {
  const s = text(value).trim();
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  if (!m) return null;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 0, 0, 0, 0).getTime();
}

/** End of a `YYYY-MM-DD` day in local time, or null when empty/invalid. */
export function dayEnd(value) {
  const s = text(value).trim();
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  if (!m) return null;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 23, 59, 59, 999).getTime();
}

/**
 * Filter audit rows.
 *
 * @param {Array<object>} events raw rows from the backend
 * @param {object} [criteria]
 * @param {string} [criteria.action] exact action/type (case-insensitive)
 * @param {string} [criteria.actor] substring over actor name / user id
 * @param {string} [criteria.entity] substring over `entityType entityId`
 * @param {string} [criteria.from] inclusive `YYYY-MM-DD` lower bound
 * @param {string} [criteria.to] inclusive `YYYY-MM-DD` upper bound
 * @param {string} [criteria.query] free text over actor/action/entity/summary
 * @returns {Array<object>} rows that match every provided criterion
 */
export function filterEvents(events, criteria = {}) {
  const list = Array.isArray(events) ? events : [];
  const c = criteria || {};
  const action = text(c.action).trim().toLowerCase();
  const actor = text(c.actor).trim().toLowerCase();
  const entity = text(c.entity).trim().toLowerCase();
  const query = text(c.query).trim().toLowerCase();
  const from = dayStart(c.from);
  const to = dayEnd(c.to);

  return list.filter((event) => {
    if (!event) return false;

    if (action && text(event.action).toLowerCase() !== action) return false;

    if (actor) {
      const haystack = `${text(event.actorName)} ${text(event.actorUserId)}`.toLowerCase();
      if (!haystack.includes(actor)) return false;
    }

    if (entity) {
      const haystack = `${text(event.entityType)} ${text(event.entityId)}`.toLowerCase();
      if (!haystack.includes(entity)) return false;
    }

    if (query) {
      const haystack = [
        text(event.actorName),
        text(event.actorUserId),
        text(event.action),
        text(event.entityType),
        text(event.entityId),
        text(event.summary),
      ]
        .join(" ")
        .toLowerCase();
      if (!haystack.includes(query)) return false;
    }

    if (from !== null || to !== null) {
      const ts = eventTimestamp(event);
      if (from !== null && ts < from) return false;
      if (to !== null && ts > to) return false;
    }

    return true;
  });
}

/** Distinct, sorted actions present in the data — used to fill the filter. */
export function uniqueActions(events) {
  const list = Array.isArray(events) ? events : [];
  const seen = new Set();
  for (const event of list) {
    const action = text(event && event.action).trim();
    if (action) seen.add(action);
  }
  return Array.from(seen).sort((a, b) => a.localeCompare(b));
}

/** Distinct, sorted entity types present in the data. */
export function uniqueEntityTypes(events) {
  const list = Array.isArray(events) ? events : [];
  const seen = new Set();
  for (const event of list) {
    const type = text(event && event.entityType).trim();
    if (type) seen.add(type);
  }
  return Array.from(seen).sort((a, b) => a.localeCompare(b));
}

/**
 * Slice `items` into a page descriptor.
 *
 * @param {Array} items
 * @param {number} [page] 1-based page number (clamped to range)
 * @param {number} [pageSize]
 * @returns {{items:Array, page:number, pageSize:number, pages:number, total:number, hasPrev:boolean, hasNext:boolean}}
 */
export function paginate(items, page = 1, pageSize = DEFAULT_PAGE_SIZE) {
  const list = Array.isArray(items) ? items : [];
  const size = Math.max(1, Math.floor(Number(pageSize) || DEFAULT_PAGE_SIZE));
  const pages = Math.max(1, Math.ceil(list.length / size));
  let current = Math.floor(Number(page) || 1);
  if (!Number.isFinite(current) || current < 1) current = 1;
  if (current > pages) current = pages;
  const start = (current - 1) * size;
  return {
    items: list.slice(start, start + size),
    page: current,
    pageSize: size,
    pages,
    total: list.length,
    hasPrev: current > 1,
    hasNext: current < pages,
  };
}

/** RFC 4180 escaping: quote fields containing comma, quote or newline. */
export function csvCell(value) {
  const s = text(value);
  if (/[",\n\r]/.test(s)) return `"${s.replace(/"/g, '""')}"`;
  return s;
}

function csvValue(event, key) {
  if (!event) return "";
  switch (key) {
    case "actor":
      return text(event.actorName || event.actorUserId);
    case "summary":
      return summarizeEvent(event);
    default:
      return text(event[key]);
  }
}

/**
 * Render rows as CSV text (header included). Always returns the header line
 * even when there are no rows, so an empty export still opens cleanly.
 */
export function toCSV(events) {
  const list = Array.isArray(events) ? events : [];
  const header = CSV_COLUMNS.map((col) => col.label).join(",");
  const lines = list.map((event) =>
    CSV_COLUMNS.map((col) => csvCell(csvValue(event, col.key))).join(","),
  );
  return [header, ...lines].join("\n");
}
