import assert from 'assert';
import {
  DIRECTION_GLYPHS,
  DIRECTION_ORDER,
  buildExitLegend,
  directionGlyph,
  normalizeDirection,
} from './minimapDirections.js';

// ── normalizeDirection ──────────────────────────────────────────────────────

assert.strictEqual(normalizeDirection('North'), 'north');
assert.strictEqual(normalizeDirection(' N '), 'north');
assert.strictEqual(normalizeDirection('ne'), 'northeast');
assert.strictEqual(normalizeDirection('northeast'), 'northeast');
assert.strictEqual(normalizeDirection('UP'), 'up');
assert.strictEqual(normalizeDirection('upstairs'), 'up');
assert.strictEqual(normalizeDirection('out'), 'out');
assert.strictEqual(normalizeDirection('exit'), 'out');
assert.strictEqual(normalizeDirection('temple'), null);
assert.strictEqual(normalizeDirection(''), null);
assert.strictEqual(normalizeDirection(null), null);

// ── directionGlyph ──────────────────────────────────────────────────────────

assert.strictEqual(directionGlyph('north'), 'N');
assert.strictEqual(directionGlyph('NE'), 'NE');
assert.strictEqual(directionGlyph('up'), DIRECTION_GLYPHS.up);
assert.strictEqual(directionGlyph('down'), DIRECTION_GLYPHS.down);
assert.strictEqual(directionGlyph('temple'), 'T', 'special exits fall back to initial');
assert.strictEqual(directionGlyph(''), '?');

// ── buildExitLegend: discovery from atlas places ────────────────────────────

const places = [
  { id: 'room-market', name: 'Market Square', discovered: true },
  { id: 'room-cave', name: 'Hidden Cave', discovered: false },
];
const exits = [
  { name: 'north', target: 'room-market' },
  { name: 'east', target: 'room-cave' },
  { name: 'down', target: 'room-unknown' },
  { name: 'north', target: 'room-market' }, // duplicate direction collapses
  { name: 'temple', target: 'room-shrine' },
  { name: 'secret', target: 'room-vault', hidden: true },
];

let legend = buildExitLegend({ exits, places, visitedRooms: {} });
assert.deepStrictEqual(
  legend.map((e) => e.direction || e.name),
  ['north', 'east', 'down', 'temple'],
  'ordered by compass order, hidden exits dropped, duplicate dirs collapsed'
);

const north = legend.find((e) => e.direction === 'north');
assert.strictEqual(north.explored, true);
assert.strictEqual(north.label, 'Market Square');
assert.strictEqual(north.glyph, 'N');

const east = legend.find((e) => e.direction === 'east');
assert.strictEqual(east.explored, false, 'undiscovered atlas place is not explored');
assert.strictEqual(east.label, '?', 'undiscovered room name is never leaked');
assert.strictEqual(east.glyph, 'E');

const down = legend.find((e) => e.direction === 'down');
assert.strictEqual(down.explored, false);
assert.strictEqual(down.label, '?');
assert.strictEqual(down.targetId, 'room-unknown');

const temple = legend.find((e) => e.name === 'temple');
assert.strictEqual(temple.direction, null);
assert.strictEqual(temple.label, '?', 'unknown special exit is a question mark');
assert.strictEqual(temple.glyph, 'T');

// ── visitedRooms fallback + missing target ──────────────────────────────────

legend = buildExitLegend({
  exits: [
    { name: 'south', target: 'room-visited' },
    { name: 'west', target: '' },
  ],
  places,
  visitedRooms: { 'room-visited': { id: 'room-visited', name: 'Old Well' } },
});
const south = legend.find((e) => e.direction === 'south');
assert.strictEqual(south.explored, true, 'locally visited rooms count as explored');
assert.strictEqual(south.label, 'Old Well');

const west = legend.find((e) => e.direction === 'west');
assert.strictEqual(west.explored, false);
assert.strictEqual(west.label, '?');
assert.strictEqual(west.targetId, null);

// ── empty / malformed input ─────────────────────────────────────────────────

assert.deepStrictEqual(buildExitLegend({}), []);
assert.deepStrictEqual(buildExitLegend(), []);
assert.deepStrictEqual(buildExitLegend({ exits: [null, undefined], places: [] }), []);

// A direction with no target at all still surfaces (server said it exists).
const bare = buildExitLegend({ exits: [{ name: 'east' }], places: [], visitedRooms: {} });
assert.strictEqual(bare.length, 1);
assert.strictEqual(bare[0].direction, 'east');
assert.strictEqual(bare[0].explored, false);
assert.ok(DIRECTION_ORDER.includes('northeast'));
assert.strictEqual(new Set(DIRECTION_ORDER).size, DIRECTION_ORDER.length, 'direction order has no dupes');

console.log('minimapDirections_test.mjs ok');
