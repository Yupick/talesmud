// Dungeon REST helpers (FASE 5.1 - Batch 19).
//
// Backend contract (fixed; served by `pkg/server`, creator-gated writes):
//
//   GET    /api/dungeons?name=&author=&limit=   list metadata rows
//   GET    /api/dungeons/:id                    one dungeon incl. `definition`
//   POST   /api/dungeons                        create (creator only)
//   PUT    /api/dungeons/:id                    update (creator only)
//   DELETE /api/dungeons/:id                    delete (creator only)
//
// `definition` is the descriptor produced by `src/game/dungeonBuilder.js`
// `exportDefinition(grid)`: `{version, grid:{width,height}, cells, rooms,
// corridors, entrances, exits, encounters, lootTables, meta}`.
//
// Auth follows the same pattern as `npcs.js`/`clans.js`: the session JWT travels
// as `Authorization: Bearer <token>`. Every thrown `Error` carries `.status`
// (HTTP status) and/or `.code === "network"` so callers can detect a missing
// route / missing role (`isDungeonRouteError`) and degrade to localStorage.
//
// The dungeon panel (`ui/DungeonBuilder.svelte`) treats localStorage as the
// source of truth and uses these functions best-effort on top.
import { backend } from "./base.js";

/** True once the backend serves the /api/dungeons CRUD routes. */
export const DUNGEON_CRUD_AVAILABLE = true;

/**
 * Human-readable availability of every endpoint this module talks to, kept as
 * data so the UI / report can render which actions are backed by the server.
 */
export const DUNGEON_ENDPOINT_STATUS = {
  listDungeons: true,
  getDungeon: true,
  createDungeon: true,
  updateDungeon: true,
  deleteDungeon: true,
};

/** HTTP statuses that mean "the route or the role is not available here". */
const ROUTE_STATUSES = new Set([401, 403, 404, 501]);

/**
 * True when the failure means the dungeon routes are not usable for this
 * session: endpoint missing (404), creator role missing (401/403),
 * not implemented (501) or the backend could not be reached at all.
 * Callers should fall back to localStorage in that case.
 * @param {Error & {status?:number, code?:string}} err
 */
function isDungeonRouteError(err) {
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

/**
 * GET /api/dungeons — list dungeon metadata (no definitions).
 * @param {string} token session JWT
 * @param {{name?:string, author?:string, limit?:number}} [params]
 * @returns {Promise<Array<{id:string, name:string, description:string, gridWidth:number, gridHeight:number, author:string, tags:string[], updatedAt:string}>>}
 */
function listDungeons(token, params = {}) {
  return request(token, "GET", withQuery("/dungeons", params));
}

/**
 * GET /api/dungeons/:id — one dungeon including its full `definition`.
 * @param {string} token session JWT
 * @param {string} id
 */
function getDungeon(token, id) {
  return request(token, "GET", `/dungeons/${encodeURIComponent(id)}`);
}

/**
 * POST /api/dungeons — create (creator-gated).
 * @param {string} token session JWT
 * @param {{name:string, description?:string, gridWidth:number, gridHeight:number, definition:object, author?:string, tags?:string[]}} payload
 */
function createDungeon(token, payload) {
  return request(token, "POST", "/dungeons", payload);
}

/**
 * PUT /api/dungeons/:id — update (creator-gated).
 * @param {string} token session JWT
 * @param {string} id
 * @param {object} payload same shape as create
 */
function updateDungeon(token, id, payload) {
  return request(token, "PUT", `/dungeons/${encodeURIComponent(id)}`, payload);
}

/**
 * DELETE /api/dungeons/:id (creator-gated).
 * @param {string} token session JWT
 * @param {string} id
 */
function deleteDungeon(token, id) {
  return request(token, "DELETE", `/dungeons/${encodeURIComponent(id)}`);
}

export {
  listDungeons,
  getDungeon,
  createDungeon,
  updateDungeon,
  deleteDungeon,
  isDungeonRouteError,
  withQuery,
};
