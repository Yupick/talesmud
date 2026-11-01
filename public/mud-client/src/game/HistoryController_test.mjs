import assert from 'assert';
import { HistoryController } from './echo/HistoryController.js';
import { HISTORY_STORAGE_KEY } from './commandHistory.js';

function fakeStorage(seed) {
  const map = new Map(seed ? Object.entries(seed) : []);
  return {
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => map.set(k, String(v)),
    _map: map,
  };
}

// ── restore + persist ───────────────────────────────────────────────────────

const storage = fakeStorage({
  [HISTORY_STORAGE_KEY]: JSON.stringify(['look', 'north']),
});
const hc = new HistoryController({ storage });

assert.deepStrictEqual(hc.entries, ['look', 'north'], 'loads persisted history');
assert.strictEqual(hc.cursor, 2);

hc.push('say hi');
assert.deepStrictEqual(hc.entries, ['look', 'north', 'say hi']);
assert.deepStrictEqual(
  JSON.parse(storage.getItem(HISTORY_STORAGE_KEY)),
  ['look', 'north', 'say hi'],
  'push persists to storage'
);

hc.push('say hi');
assert.deepStrictEqual(hc.entries, ['look', 'north', 'say hi'], 'consecutive dedup');

// ── draft-preserving navigation ─────────────────────────────────────────────

assert.strictEqual(hc.getPrevious('half typed'), 'say hi');
assert.strictEqual(hc.getPrevious('say hi'), 'north');
assert.strictEqual(hc.getPrevious('north'), 'look');
assert.strictEqual(hc.getPrevious('look'), 'look', 'oldest entry stays put');
assert.strictEqual(hc.getNext('look'), 'north');
assert.strictEqual(hc.getNext('north'), 'say hi');
assert.strictEqual(hc.getNext('say hi'), 'half typed', 'past newest restores the draft');

// ── rewind ──────────────────────────────────────────────────────────────────

hc.getPrevious('draft');
hc.rewind();
assert.strictEqual(hc.cursor, hc.entries.length, 'rewind leaves "not browsing"');

// ── back-compat numeric constructor ─────────────────────────────────────────

const legacy = new HistoryController(3);
legacy.push('a');
legacy.push('b');
legacy.push('c');
legacy.push('d');
assert.deepStrictEqual(legacy.entries, ['b', 'c', 'd'], 'numeric size still caps');

console.log('HistoryController_test.mjs ok');
