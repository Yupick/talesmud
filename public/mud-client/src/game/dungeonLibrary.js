/**
 * Dungeon library model (FASE 5.1 - Batch 19).
 *
 * Pure module (no DOM, testable with `node --test`) that bridges the grid
 * editor (`dungeonBuilder.js`) with the persistence layer:
 *
 *   - descriptor <-> REST payload mapping (`buildDungeonPayload`,
 *     `gridFromEntry` / `definitionToGrid`)
 *   - normalization of the `/api/dungeons` list rows (`normalizeListRows`)
 *   - a localStorage-backed local library (load/save/upsert/remove) used as
 *     source of truth and as fallback when the server routes are missing or
 *     the session lacks the creator role.
 *
 * Local entries and server dungeons share one id space: an entry keeps its
 * `local_...` id until the first successful server create, after which it is
 * renamed to the server id so both sides can be updated/deleted together.
 */
import {
  DUNGEON_STORAGE_KEY,
  clone,
  fromJSON,
  autoValidate,
  exportDefinition,
} from './dungeonBuilder.js';

function intOr(value, fallback) {
  const n = typeof value === 'number' ? value : Number.parseFloat(value);
  return Number.isFinite(n) ? Math.trunc(n) : fallback;
}

function resolveStorage(storage) {
  if (storage) return storage;
  try {
    return typeof localStorage !== 'undefined' ? localStorage : null;
  } catch (err) {
    return null;
  }
}

/** True when `value` looks like a descriptor produced by `exportDefinition`. */
export function isDefinition(value) {
  return (
    !!value &&
    typeof value === 'object' &&
    Array.isArray(value.cells) &&
    (!!value.grid || (value.width !== undefined && value.height !== undefined))
  );
}

/**
 * Build the POST/PUT body from the current grid and the library metadata.
 * @param {{name:string, description?:string, author?:string, tags?:string[], grid:object}} entry
 * @returns {{name:string, description:string, gridWidth:number, gridHeight:number, definition:object, author?:string, tags?:string[]}|null}
 */
export function buildDungeonPayload({ name, description = '', author = '', tags = [], grid } = {}) {
  const trimmed = String(name ?? '').trim();
  if (!trimmed) return null;
  if (!grid || !Array.isArray(grid.cells) || !(grid.width > 0) || !(grid.height > 0)) {
    return null;
  }
  const definition = exportDefinition(grid);
  // Clone meta so the payload never aliases the live grid state.
  definition.meta = { ...(definition.meta || {}) };
  if (!definition.meta.name) definition.meta.name = trimmed;

  const payload = {
    name: trimmed,
    description: String(description ?? ''),
    gridWidth: grid.width,
    gridHeight: grid.height,
    definition,
  };
  const cleanAuthor = String(author ?? '').trim();
  if (cleanAuthor) payload.author = cleanAuthor;
  const cleanTags = (Array.isArray(tags) ? tags : [])
    .map((t) => String(t).trim())
    .filter(Boolean);
  if (cleanTags.length > 0) payload.tags = cleanTags;
  return payload;
}

/**
 * Convert a dungeon descriptor (or a raw grid) back into an editor grid.
 * Accepts both `{grid:{width,height}, cells, ...}` (exportDefinition output)
 * and plain grid objects so older/local shapes keep working.
 * @param {object} definition
 * @returns {object|null} grid usable by the editor, or null when unreadable
 */
export function definitionToGrid(definition) {
  if (!isDefinition(definition)) return null;
  const size = definition.grid && typeof definition.grid === 'object' ? definition.grid : definition;
  const width = intOr(size.width ?? definition.width, 0);
  const height = intOr(size.height ?? definition.height, 0);
  if (width <= 0 || height <= 0) return null;
  const shaped = { ...definition, width, height };
  const grid = fromJSON(shaped);
  if (!grid) return null;
  // Recompute entrances/exits from the cells so stale arrays cannot break the editor.
  autoValidate(grid);
  return grid;
}

/**
 * Grid for a library entry (local entry or `GET /api/dungeons/:id` response).
 * @param {{definition?:object, grid?:object}|object} entry
 * @returns {object|null}
 */
export function gridFromEntry(entry) {
  if (!entry || typeof entry !== 'object') return null;
  return definitionToGrid(entry.definition || entry.grid || entry);
}

/**
 * Normalize the rows returned by GET /api/dungeons into a stable shape.
 * Rows without an id are dropped.
 * @param {any} rows
 * @returns {Array<{id:string, name:string, description:string, gridWidth:number, gridHeight:number, author:string, tags:string[], updatedAt:any}>}
 */
export function normalizeListRows(rows) {
  if (!Array.isArray(rows)) return [];
  const out = [];
  for (const row of rows) {
    if (!row || typeof row !== 'object') continue;
    const id = String(row.id ?? '').trim();
    if (!id) continue;
    out.push({
      id,
      name: String(row.name ?? '').trim() || id,
      description: String(row.description ?? ''),
      gridWidth: intOr(row.gridWidth, 0),
      gridHeight: intOr(row.gridHeight, 0),
      author: String(row.author ?? ''),
      tags: Array.isArray(row.tags) ? row.tags.map((t) => String(t)) : [],
      updatedAt: row.updatedAt ?? null,
    });
  }
  return out;
}

/** Normalize a local entry (same fields plus the optional `definition`). */
export function normalizeLocalEntry(raw = {}) {
  const base = normalizeListRows([raw])[0];
  if (!base) return null;
  base.definition =
    raw.definition && typeof raw.definition === 'object' ? clone(raw.definition) : null;
  return base;
}

/** Fresh local id for an entry that has not been created on the server yet. */
export function nextLocalId(entries = []) {
  return `local_${Date.now().toString(36)}_${entries.length + 1}`;
}

/**
 * Insert or replace an entry by id. Entries without an id get a fresh
 * `local_...` id. Returns a NEW array (immutability keeps Svelte happy).
 */
export function upsertLocalEntry(entries, entry) {
  const list = Array.isArray(entries) ? entries.slice() : [];
  const id = String(entry?.id ?? '').trim() || nextLocalId(list);
  const normalized = normalizeLocalEntry({ ...entry, id });
  if (!normalized) return list;
  const idx = list.findIndex((e) => e.id === id);
  if (idx >= 0) list[idx] = normalized;
  else list.push(normalized);
  return list;
}

/** Remove an entry by id. Returns a NEW array. */
export function removeLocalEntry(entries, id) {
  const key = String(id ?? '');
  return (Array.isArray(entries) ? entries : []).filter((e) => e.id !== key);
}

/**
 * Read the local library from storage. Corrupt or missing data yields [].
 * @param {Storage} [storage]
 * @returns {Array<object>}
 */
export function loadLocalEntries(storage) {
  const store = resolveStorage(storage);
  if (!store) return [];
  try {
    const raw = store.getItem(DUNGEON_STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const out = [];
    for (const rawEntry of parsed) {
      const entry = normalizeLocalEntry(rawEntry);
      if (entry) out.push(entry);
    }
    return out;
  } catch (err) {
    return [];
  }
}

/**
 * Persist the local library. Returns true on success.
 * @param {Array<object>} entries
 * @param {Storage} [storage]
 */
export function saveLocalEntries(entries, storage) {
  const store = resolveStorage(storage);
  if (!store) return false;
  try {
    store.setItem(DUNGEON_STORAGE_KEY, JSON.stringify(Array.isArray(entries) ? entries : []));
    return true;
  } catch (err) {
    return false;
  }
}

/** Drop the local library. Returns true on success. */
export function clearLocalEntries(storage) {
  const store = resolveStorage(storage);
  if (!store) return false;
  try {
    store.removeItem(DUNGEON_STORAGE_KEY);
    return true;
  } catch (err) {
    return false;
  }
}

export default {
  isDefinition,
  buildDungeonPayload,
  definitionToGrid,
  gridFromEntry,
  normalizeListRows,
  normalizeLocalEntry,
  nextLocalId,
  upsertLocalEntry,
  removeLocalEntry,
  loadLocalEntries,
  saveLocalEntries,
  clearLocalEntries,
  DUNGEON_STORAGE_KEY,
};
