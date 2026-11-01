// NPC REST helpers (Creator Phase 5.3; template contract realigned in Batch 21).
//
// Backend contract (fixed; served by `pkg/server`, creator-gated writes):
//
//   TEMPLATE CRUD (Batch 21 — the contract the factory panel talks to)
//     GET    /api/npc-templates?name=&role=&limit=   list summary rows
//     GET    /api/npc-templates/:id                  one full template DTO
//     POST   /api/npc-templates                      create (creator only)
//     PUT    /api/npc-templates/:id                  update (creator only)
//     DELETE /api/npc-templates/:id                  delete (creator only)
//
//   The summary row is `{id,name,role,creatureType,level,maxHp,damageMin,
//   damageMax,aggressive,xpReward,goldMin,goldMax,spawnRoomId,updatedAt}`; the
//   full DTO adds `description`, `loot`, `behavior`, `scaling`,
//   `respawnSeconds`, `spawnCount`, `lootTableId`, `combatStyle`,
//   `difficulty`, `aggroRadius`, `isTemplate`. The mapping to the editor model
//   lives in `src/game/npcFactory.js` (`toTemplatePayload` /
//   `fromTemplatePayload`).
//
//   LEGACY INSTANCE ROUTES (deprecated; kept for spawn/inspect tooling)
//     GET    /api/npcs?roomID=&isTemplate=
//     GET    /api/npcs/templates
//     GET    /api/npcs/:id
//     POST   /api/npcs
//     PUT    /api/npcs/:id
//     DELETE /api/npcs/:id
//     POST   /api/npcs/:id/spawn
//
// Auth follows the same pattern as `dungeons.js`/`clans.js`: the session JWT
// travels as `Authorization: Bearer <token>`. Every thrown `Error` carries
// `.status` (HTTP status) and/or `.code === "network"` so callers can detect a
// missing route / missing role (`isNpcRouteError`) and degrade to localStorage.
//
// The factory panel (`ui/NpcFactory.svelte`) treats localStorage as the
// source of truth and uses these functions best-effort on top.
import { backend } from "./base.js";

/** True once the backend serves the /api/npc-templates CRUD routes. */
export const NPC_CRUD_AVAILABLE = true;

/**
 * Human-readable availability of every endpoint this module talks to, kept as
 * data so the UI / report can render which actions are backed by the server.
 */
export const NPC_ENDPOINT_STATUS = {
  listNpcTemplates: true,
  getNpcTemplate: true,
  createNpcTemplate: true,
  updateNpcTemplate: true,
  deleteNpcTemplate: true,
  // Legacy /api/npcs instance routes (deprecated).
  listNpcs: true,
  listNpcTemplatesLegacy: true,
  getNpc: true,
  createNpc: true,
  updateNpc: true,
  deleteNpc: true,
  spawnNpc: true,
};

/** HTTP statuses that mean "the route or the role is not available here". */
const ROUTE_STATUSES = new Set([401, 403, 404, 501]);

/**
 * True when the failure means the NPC template routes are not usable for this
 * session: endpoint missing (404), creator role missing (401/403),
 * not implemented (501) or the backend could not be reached at all.
 * Callers should fall back to localStorage in that case.
 * @param {Error & {status?:number, code?:string}} err
 */
function isNpcRouteError(err) {
  if (!err) return false;
  if (err.code === "network") return true;
  return ROUTE_STATUSES.has(err.status);
}

async function request(token, method, path, body) {
  const headers = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  const options = { method, credentials: "same-origin", headers };
  if (body !== undefined) {
    options.headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body || {});
  }

  let response;
  try {
    response = await fetch(`${backend}${path}`, options);
  } catch (err) {
    const error = new Error("Network error. Please check your connection and try again.");
    error.code = "network";
    throw error;
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
    const error = new Error(message);
    error.status = response.status;
    throw error;
  }

  return data;
}

/** Drop empty query params so optional filters never reach the backend as "". */
function withQuery(path, params) {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params || {})) {
    if (value === undefined || value === null || value === "") continue;
    qs.set(key, String(value));
  }
  const query = qs.toString();
  return query ? `${path}?${query}` : path;
}

// ---------------------------------------------------------------------------
// Template CRUD (Batch 21 contract)
// ---------------------------------------------------------------------------

/**
 * GET /api/npc-templates — list template summaries.
 * @param {string} token session JWT
 * @param {{name?:string, role?:string, limit?:number}} [params]
 * @returns {Promise<Array<{id:string, name:string, role:string, creatureType:string, level:number, maxHp:number, damageMin:number, damageMax:number, aggressive:boolean, xpReward:number, goldMin:number, goldMax:number, spawnRoomId:string, updatedAt:string}>>}
 */
function listNpcTemplates(token, params = {}) {
  return request(token, "GET", withQuery("/npc-templates", params));
}

/**
 * GET /api/npc-templates/:id — one full template DTO (incl. description, loot,
 * behavior, scaling, respawnSeconds, spawnCount, lootTableId, combatStyle,
 * difficulty, aggroRadius, isTemplate).
 * @param {string} token session JWT
 * @param {string} id
 */
function getNpcTemplate(token, id) {
  return request(token, "GET", `/npc-templates/${encodeURIComponent(id)}`);
}

/**
 * POST /api/npc-templates — create (creator-gated). Body is the full template
 * payload produced by `npcFactory.toTemplatePayload`.
 * @param {string} token session JWT
 * @param {object} payload
 */
function createNpcTemplate(token, payload) {
  return request(token, "POST", "/npc-templates", payload);
}

/**
 * PUT /api/npc-templates/:id — update (creator-gated).
 * @param {string} token session JWT
 * @param {string} id
 * @param {object} payload same shape as create
 */
function updateNpcTemplate(token, id, payload) {
  return request(token, "PUT", `/npc-templates/${encodeURIComponent(id)}`, payload);
}

/**
 * DELETE /api/npc-templates/:id (creator-gated).
 * @param {string} token session JWT
 * @param {string} id
 */
function deleteNpcTemplate(token, id) {
  return request(token, "DELETE", `/npc-templates/${encodeURIComponent(id)}`);
}

// ---------------------------------------------------------------------------
// Legacy /api/npcs instance routes (deprecated — kept for spawn/inspect tools)
// ---------------------------------------------------------------------------

/** @deprecated Use the template CRUD above. GET /api/npcs — all NPCs. */
function listNpcs(token, params = {}) {
  return request(token, "GET", withQuery("/npcs", params));
}

/** @deprecated Use `listNpcTemplates`. GET /api/npcs/templates — legacy list. */
function listNpcTemplatesLegacy(token, params = {}) {
  return request(token, "GET", withQuery("/npcs/templates", params));
}

/** @deprecated Use `getNpcTemplate`. GET /api/npcs/:id */
function getNpc(token, id) {
  return request(token, "GET", `/npcs/${encodeURIComponent(id)}`);
}

/** @deprecated Use `createNpcTemplate`. POST /api/npcs */
function createNpc(token, npc) {
  return request(token, "POST", "/npcs", npc);
}

/** @deprecated Use `updateNpcTemplate`. PUT /api/npcs/:id */
function updateNpc(token, id, npc) {
  return request(token, "PUT", `/npcs/${encodeURIComponent(id)}`, npc);
}

/** @deprecated Use `deleteNpcTemplate`. DELETE /api/npcs/:id */
function deleteNpc(token, id) {
  return request(token, "DELETE", `/npcs/${encodeURIComponent(id)}`);
}

/** POST /api/npcs/:id/spawn — spawn a template instance. */
function spawnNpc(token, id, body = {}) {
  return request(token, "POST", `/npcs/${encodeURIComponent(id)}/spawn`, body);
}

export {
  listNpcTemplates,
  getNpcTemplate,
  createNpcTemplate,
  updateNpcTemplate,
  deleteNpcTemplate,
  isNpcRouteError,
  listNpcs,
  listNpcTemplatesLegacy,
  getNpc,
  createNpc,
  updateNpc,
  deleteNpc,
  spawnNpc,
  withQuery,
};
