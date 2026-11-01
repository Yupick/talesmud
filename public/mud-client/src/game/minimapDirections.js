/**
 * Exit legend for the minimap widget (Fase 3.4).
 *
 * Pure helpers that turn the *server-emitted* room exits into a compact
 * direction strip:
 *   - direction name → canonical compass/vertical key + glyph
 *   - each exit → { explored, label } using the atlas discovery flags
 *     (`places[].discovered`) or locally visited rooms
 *   - undiscovered / unknown targets render as "?"
 *
 * No invented data: anything the server has not disclosed stays hidden.
 */

export const DIRECTION_ORDER = [
  'north', 'northeast', 'east', 'southeast',
  'south', 'southwest', 'west', 'northwest',
  'up', 'down', 'in', 'out',
];

export const DIRECTION_GLYPHS = {
  north: 'N',
  northeast: 'NE',
  east: 'E',
  southeast: 'SE',
  south: 'S',
  southwest: 'SW',
  west: 'W',
  northwest: 'NW',
  up: '\u2191',
  down: '\u2193',
  in: 'In',
  out: 'Out',
};

const DIRECTION_ALIASES = {
  n: 'north', north: 'north',
  ne: 'northeast', northeast: 'northeast',
  e: 'east', east: 'east',
  se: 'southeast', southeast: 'southeast',
  s: 'south', south: 'south',
  sw: 'southwest', southwest: 'southwest',
  w: 'west', west: 'west',
  nw: 'northwest', northwest: 'northwest',
  u: 'up', up: 'up', upstairs: 'up',
  d: 'down', down: 'down', downstairs: 'down',
  in: 'in', enter: 'in', inside: 'in',
  out: 'out', exit: 'out', outside: 'out', leave: 'out',
};

/** Canonicalise an exit/direction name, or null when it is not a direction. */
export function normalizeDirection(name) {
  const key = String(name ?? '').trim().toLowerCase();
  return DIRECTION_ALIASES[key] || null;
}

/** Short compass glyph. Falls back to the first initial of the raw name. */
export function directionGlyph(name) {
  const dir = normalizeDirection(name);
  if (dir && DIRECTION_GLYPHS[dir]) return DIRECTION_GLYPHS[dir];
  const raw = String(name ?? '').trim();
  return raw ? raw.charAt(0).toUpperCase() : '?';
}

function directionRank(dir) {
  const idx = DIRECTION_ORDER.indexOf(dir);
  return idx === -1 ? DIRECTION_ORDER.length : idx;
}

function rawName(exit) {
  return String(exit?.name || exit?.Name || exit?.command || exit?.Command || '').trim();
}

function isHidden(exit) {
  return !!(exit && (exit.hidden || exit.Hidden || exit.isHidden));
}

/**
 * Build the exit strip for the current room.
 *
 * @param {object} opts
 * @param {Array}  opts.exits       raw server exits (`name`, `target`, `hidden`)
 * @param {Array}  opts.places      atlas places carrying `{ id, name, discovered }`
 * @param {object} opts.visitedRooms locally visited rooms keyed by id
 * @returns {Array<{ key, direction, glyph, label, explored, targetId, name }>}
 */
export function buildExitLegend({ exits, places, visitedRooms } = {}) {
  const placeById = new Map();
  for (const p of places || []) {
    if (p && p.id != null) placeById.set(String(p.id), p);
  }
  const out = [];
  const seen = new Set();

  for (const exit of exits || []) {
    if (!exit || isHidden(exit)) continue;
    const name = rawName(exit);
    const dir = normalizeDirection(rawName(exit));
    const targetId = String(exit.target || exit.Target || '').trim();
    if (!dir && !name && !targetId) continue;

    const key = dir || rawName(exit).toLowerCase() || targetId;
    if (seen.has(key)) continue;
    seen.add(key);

    const place = targetId ? (placeById.get(targetId) || null) : null;
    const visitedRoom = targetId ? ((visitedRooms || {})[targetId] || null) : null;

    const discovered = !!(place && place.discovered) || !!visitedRoom;
    // Never leak an undiscovered room name through the atlas.
    const label = discovered
      ? (place?.name || visitedRoom?.name || targetId || rawName(exit))
      : '?';

    out.push({
      key: dir || targetId || rawName(exit).toLowerCase(),
      direction: dir,
      glyph: directionGlyph(rawName(exit) || dir),
      name: rawName(exit),
      targetId: targetId || null,
      explored: discovered,
      label,
    });
  }

  out.sort((a, b) => {
    const dr = directionRank(a.direction) - directionRank(b.direction);
    if (dr !== 0) return dr;
    return String(a.key).localeCompare(String(b.key));
  });
  return out;
}
