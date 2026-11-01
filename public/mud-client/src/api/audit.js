// Audit log REST helpers (Creator Phase 5.4).
//
// Backend status — read this before wiring the viewer to the network:
//
//   EXISTING
//     GET /api/audit   creator-gated, `handler.ListAudit` (pkg/server/server.go:562)
//                      query params understood by the server:
//                        entityType  exact match
//                        entityId    exact match
//                        limit       0..500 (server defaults to 100, clamps at 500)
//                      Responds with a JSON array of audit.Entry objects:
//                        { id, time, actorUserId, actorName, action, entityType,
//                          entityId, before?, after?, source, undoable,
//                          undoneBy?, summary?, inverse? }
//
//   PENDING / NOT SERVED
//     GET  /api/audit/export      server-side CSV export   (client generates it instead)
//     action / actor / from / to / q query filters         (applied client-side instead)
//     POST /api/audit/:id/undo    exists but is ADMIN-only and out of scope here
//
// Because no server-side filtering or export exists, `AuditPanel.svelte` fetches
// the recent window (limit=500) and uses `src/game/auditView.js` to filter,
// search, paginate and export in the browser.
import { backend } from "./base.js";

/** Human-readable status of every audit capability the viewer assumes. */
export const AUDIT_ENDPOINT_STATUS = {
  listAudit: true, // GET /api/audit — creator-gated, handler.ListAudit
  exportAudit: false, // no server-side export; generated client-side
  filterByAction: false, // not a server query param
  filterByActor: false, // not a server query param
  filterByDate: false, // not a server query param
  search: false, // not a server query param
};

/** Which `GET /api/audit` query parameters the backend actually reads. */
export const AUDIT_QUERY_SUPPORT = {
  entityType: true,
  entityId: true,
  limit: true,
  action: false,
  actor: false,
  from: false,
  to: false,
  q: false,
};

/** Backend clamps `limit` to 500; the viewer loads the recent window. */
export const AUDIT_MAX_LIMIT = 500;

function withQuery(path, params) {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params || {})) {
    if (value === undefined || value === null || value === "") continue;
    qs.set(key, String(value));
  }
  const query = qs.toString();
  return query ? `${path}?${query}` : path;
}

/**
 * List recent audit rows (GET /api/audit — exists).
 *
 * Only `entityType`, `entityId` and `limit` are forwarded; the server ignores
 * anything else. Filtering by action/actor/date and free-text search happen in
 * the client.
 *
 * @param {string} token session JWT (creator or admin)
 * @param {{entityType?:string, entityId?:string, limit?:number}} [params]
 * @returns {Promise<Array<object>>}
 */
async function listAudit(token, params = {}) {
  const clean = {};
  for (const key of ["entityType", "entityId", "limit"]) {
    if (params[key] !== undefined && params[key] !== null && params[key] !== "") {
      clean[key] = params[key];
    }
  }
  const url = withQuery("/audit", clean);

  const headers = {};
  if (token) headers.Authorization = `Bearer ${token}`;

  let response;
  try {
    response = await fetch(`${backend}${url}`, {
      method: "GET",
      credentials: "same-origin",
      headers,
    });
  } catch (err) {
    throw new Error("Network error. Please check your connection and try again.");
  }

  let data = null;
  try {
    data = await response.json();
  } catch (err) {
    data = null;
  }

  if (!response.ok) {
    const message =
      (data && (data.error || data.message)) || `Request failed (${response.status})`;
    throw new Error(message);
  }

  return Array.isArray(data) ? data : [];
}

/**
 * PENDING backend: GET /api/audit/export. Not served yet — the viewer builds
 * the CSV in the browser. Kept so the contract is documented in one place.
 * @param {string} token
 * @param {object} [params]
 */
function exportAudit(token, params = {}) {
  const headers = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  return fetch(`${backend}${withQuery("/audit/export", params)}`, {
    method: "GET",
    credentials: "same-origin",
    headers,
  });
}

export { listAudit, exportAudit, withQuery };
