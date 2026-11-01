/**
 * Command history + Tab autocomplete for the terminal widget (Fase 3.2).
 *
 * Pure helpers only — no Svelte, no DOM. Persistence is delegated through an
 * injected `Storage`-like object so tests can pass a fake and SSR/browser
 * quota failures stay silent.
 *
 * Pieces:
 *   - pushHistory: append + consecutive dedup + cap
 *   - createHistoryState/historyPrev/historyNext: ArrowUp/ArrowDown browsing
 *     that preserves the in-progress draft when you leave history
 *   - filterCompletions/autocompleteSource/tabComplete: Tab completion of
 *     known commands and room player names
 *   - loadHistory/saveHistory: localStorage round-trip (~50 entries)
 */

export const HISTORY_LIMIT = 50;
export const HISTORY_STORAGE_KEY = 'talesmud_command_history';

/**
 * Static list of the main game commands (plus movement directions and global
 * chat channels). Kept here so autocomplete works before any server payload
 * and without importing the widget/HUD graph. The terminal merges in extra
 * server-known commands if available.
 */
export const GAME_COMMANDS = [
  // Movement
  'north', 'south', 'east', 'west',
  'northeast', 'northwest', 'southeast', 'southwest',
  'up', 'down',
  // Perception / info
  'look', 'examine', 'inventory', 'equipment', 'character', 'skills', 'spells',
  'who', 'help', 'recipes', 'quests', 'quest', 'score', 'time',
  // Social
  'say', 'tell', 'whisper', 'reply', 'shout', 'chat', 'talk', 'emote',
  'general', 'lfg', 'trade', 'roleplay', 'admin',
  // Combat
  'attack', 'kill', 'defend', 'flee', 'cast', 'focus',
  // Items / world interaction
  'use', 'drop', 'take', 'get', 'pickup', 'equip', 'wear', 'remove', 'wield',
  'buy', 'sell', 'shop', 'gather', 'craft', 'bind', 'recall',
  // Party / follow
  'follow', 'unfollow', 'party', 'invite', 'leave',
];

function storageOrNull(storage) {
  if (storage !== undefined) return storage || null;
  return typeof localStorage !== 'undefined' ? localStorage : null;
}

function normalizeEntry(entry) {
  return String(entry ?? '').replace(/[\r\n]+/g, ' ').trim();
}

/**
 * Append a command to the history.
 * - empty/whitespace-only entries are ignored
 * - consecutive duplicates collapse
 * - the list is capped to `limit` most-recent entries
 * Always returns a new array (never mutates the input).
 */
export function pushHistory(history, entry, limit = HISTORY_LIMIT) {
  const list = Array.isArray(history) ? history.slice() : [];
  const cmd = normalizeEntry(entry);
  if (!cmd) return list;
  if (list.length && normalizeEntry(list[list.length - 1]) === cmd) return list;
  list.push(cmd);
  const cap = Number.isFinite(limit) && limit > 0 ? Math.floor(limit) : HISTORY_LIMIT;
  return list.length > cap ? list.slice(list.length - cap) : list;
}

/**
 * Browsing state. `cursor === list.length` means "not browsing" — the input
 * shows the live draft. `draft` preserves whatever the player had typed
 * before they started walking the history.
 */
export function createHistoryState(history) {
  const list = Array.isArray(history)
    ? history.map(normalizeEntry).filter(Boolean)
    : [];
  return { list, cursor: list.length, draft: '' };
}

/** ArrowUp: older entry. At the oldest entry, stay put. */
export function historyPrev(state, currentText = '') {
  const list = Array.isArray(state?.list) ? state.list : [];
  const cursor = Number.isInteger(state?.cursor) ? state.cursor : list.length;
  const draft = typeof state?.draft === 'string' ? state.draft : '';
  if (!list.length) return { list, cursor: list.length, draft, text: currentText };
  if (cursor <= 0) return { list, cursor: 0, draft, text: list[0] };
  const nextCursor = cursor >= list.length ? list.length - 1 : cursor - 1;
  const nextDraft = cursor >= list.length ? currentText : draft;
  return { list, cursor: nextCursor, draft: nextDraft, text: list[nextCursor] };
}

/** ArrowDown: newer entry; past the newest, restore the saved draft. */
export function historyNext(state, currentText = '') {
  const list = Array.isArray(state?.list) ? state.list : [];
  const cursor = Number.isInteger(state?.cursor) ? state.cursor : list.length;
  const draft = typeof state?.draft === 'string' ? state.draft : '';
  if (!list.length || cursor >= list.length) {
    return { list, cursor: list.length, draft, text: currentText };
  }
  const nextCursor = cursor + 1;
  if (nextCursor >= list.length) {
    return { list, cursor: list.length, draft, text: draft };
  }
  return { list, cursor: nextCursor, draft, text: list[nextCursor] };
}

/** Reset after submit or when the user edits the input while browsing. */
export function resetHistoryCursor(state) {
  const list = Array.isArray(state?.list) ? state.list : [];
  return { list, cursor: list.length, draft: '' };
}

/**
 * Case-insensitive prefix filter with stable de-duplication (first spelling
 * wins) and alphabetical ordering. `values` may be strings or `{ command }`.
 */
export function filterCompletions(values, prefix) {
  const p = String(prefix ?? '').toLowerCase().trim();
  const out = [];
  const seen = new Set();
  for (const raw of values || []) {
    const value = typeof raw === 'string' ? raw : raw && (raw.command || raw.name);
    if (!value) continue;
    const v = String(value);
    const lower = v.toLowerCase();
    if (seen.has(lower)) continue;
    seen.add(lower);
    if (!p || lower.startsWith(p)) out.push(v);
  }
  out.sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
  return out;
}

/**
 * Decide what a Tab press should complete.
 * - first word (no space yet): complete command names
 * - after a space: complete the current token from room player names
 * Returns `{ prefix, token, values, kind }` where `prefix` is the literal
 * text kept before the token.
 */
export function autocompleteSource(input, { commands = [], players = [] } = {}) {
  const text = String(input ?? '');
  const spaceIdx = text.indexOf(' ');
  if (spaceIdx === -1 && !/\s$/.test(text)) {
    return {
      prefix: '',
      token: text,
      values: filterCompletions(commands, text),
      kind: 'command',
    };
  }
  const lastSpace = text.lastIndexOf(' ');
  const token = text.slice(lastSpace + 1);
  return {
    prefix: text.slice(0, lastSpace + 1),
    token,
    values: filterCompletions(players, token),
    kind: 'player',
  };
}

/**
 * Advance a Tab completion cycle.
 * Pass the `cycle` returned by a previous call to keep cycling the same
 * candidate list; omit it for a fresh completion from the current input.
 * Returns `{ input, cursor, cycle, matched }`.
 */
export function tabComplete(input, opts = {}) {
  const cycle = Array.isArray(opts.cycle) && opts.cycle.length ? opts.cycle : null;
  const cursor = Number.isInteger(opts.cursor) && opts.cursor >= 0 ? opts.cursor : 0;

  if (cycle) {
    const index = cursor % cycle.length;
    return { input: cycle[index], cursor: index + 1, cycle, matched: true };
  }

  const source = autocompleteSource(input, {
    commands: opts.commands || [],
    players: opts.players || [],
  });
  if (!source.values.length) {
    return { input, cursor: 0, cycle: null, matched: false };
  }
  const candidates = source.values.map((value) => source.prefix + value);
  return {
    input: candidates[0],
    cursor: 1,
    cycle: candidates,
    matched: true,
  };
}

/** Read persisted history, tolerating malformed JSON / private mode. */
export function loadHistory(storage, limit = HISTORY_LIMIT) {
  const store = storageOrNull(storage);
  if (!store) return [];
  try {
    const raw = store.getItem(HISTORY_STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const cleaned = [];
    for (const entry of parsed) {
      const cmd = normalizeEntry(entry);
      if (cmd) cleaned.push(cmd);
    }
    const cap = Number.isFinite(limit) && limit > 0 ? Math.floor(limit) : HISTORY_LIMIT;
    return cleaned.length > cap ? cleaned.slice(cleaned.length - cap) : cleaned;
  } catch (err) {
    return [];
  }
}

/** Persist history. Never throws (quota / private mode). */
export function saveHistory(history, storage) {
  const store = storageOrNull(storage);
  if (!store) return false;
  try {
    const list = Array.isArray(history) ? history.map(normalizeEntry).filter(Boolean) : [];
    store.setItem(HISTORY_STORAGE_KEY, JSON.stringify(list.slice(-HISTORY_LIMIT)));
    return true;
  } catch (err) {
    return false;
  }
}
