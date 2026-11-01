import assert from 'assert';
import {
  GAME_COMMANDS,
  HISTORY_LIMIT,
  HISTORY_STORAGE_KEY,
  autocompleteSource,
  createHistoryState,
  filterCompletions,
  historyNext,
  historyPrev,
  loadHistory,
  pushHistory,
  resetHistoryCursor,
  saveHistory,
  tabComplete,
} from './commandHistory.js';

// ── pushHistory: cap + consecutive dedup ────────────────────────────────────

let h = pushHistory([], 'look');
assert.deepStrictEqual(h, ['look']);

h = pushHistory(h, 'look');
assert.deepStrictEqual(h, ['look'], 'consecutive duplicate collapses');

h = pushHistory(h, '  look  ');
assert.deepStrictEqual(h, ['look'], 'trimmed duplicate collapses');

h = pushHistory(h, 'north');
h = pushHistory(h, 'look');
assert.deepStrictEqual(h, ['look', 'north', 'look'], 'non-consecutive duplicate is kept');

assert.deepStrictEqual(pushHistory(h, '   '), h, 'blank entry ignored');
assert.deepStrictEqual(pushHistory(h, null), h, 'null entry ignored');

// Newlines are flattened so a persisted entry stays single-line.
assert.deepStrictEqual(pushHistory([], 'say hello\nworld'), ['say hello world']);

// Cap keeps the most recent entries.
let capped = [];
for (let i = 0; i < 60; i++) capped = pushHistory(capped, `cmd${i}`);
assert.strictEqual(capped.length, HISTORY_LIMIT);
assert.strictEqual(capped[0], 'cmd10');
assert.strictEqual(capped[capped.length - 1], 'cmd59');

// Custom limit + input is never mutated.
const base = ['a'];
const out = pushHistory(base, 'b', 1);
assert.deepStrictEqual(base, ['a'], 'input array untouched');
assert.deepStrictEqual(out, ['b']);

assert.ok(GAME_COMMANDS.includes('look'));
assert.ok(GAME_COMMANDS.includes('north'));
assert.ok(GAME_COMMANDS.includes('tell'));

// ── createHistoryState ──────────────────────────────────────────────────────

let st = createHistoryState(['look', 'north', '']);
assert.deepStrictEqual(st.list, ['look', 'north']);
assert.strictEqual(st.cursor, 2);
assert.strictEqual(st.draft, '');

// ── historyPrev / historyNext: limits + draft preservation ───────────────────

const browse = createHistoryState(['look', 'north', 'say hi']);

let a = historyPrev(browse, 'half typed');
assert.strictEqual(a.cursor, 2);
assert.strictEqual(a.text, 'say hi');
assert.strictEqual(a.draft, 'half typed', 'first Up snapshots the draft');

let b = historyPrev(a, 'half typed');
assert.strictEqual(b.text, 'north');
assert.strictEqual(b.draft, 'half typed', 'draft survives further browsing');

let c = historyPrev(b, 'half typed');
assert.strictEqual(c.text, 'look');
assert.strictEqual(c.cursor, 0);

// At the oldest entry, Up stays put.
let d = historyPrev(c, 'half typed');
assert.strictEqual(d.text, 'look');
assert.strictEqual(d.cursor, 0);

// Down walks forward again.
let e = historyNext(d, 'half typed');
assert.strictEqual(e.text, 'north');
let f = historyNext(e, 'half typed');
assert.strictEqual(f.text, 'say hi');
let g = historyNext(f, 'half typed');
assert.strictEqual(g.text, 'half typed', 'past newest restores the draft');
assert.strictEqual(g.cursor, 3, 'cursor returns to "not browsing"');

// Down with no browsing is a no-op.
const noop = historyNext(createHistoryState(['a']), 'typing');
assert.strictEqual(noop.text, 'typing');
assert.strictEqual(noop.cursor, 1);

// Empty history never moves.
const empty = historyPrev(createHistoryState([]), 'typing');
assert.strictEqual(empty.text, 'typing');
assert.strictEqual(historyNext(createHistoryState([]), 'typing').text, 'typing');

// resetHistoryCursor clears browsing but keeps the list.
const reset = resetHistoryCursor(a);
assert.deepStrictEqual(reset.list, ['look', 'north', 'say hi']);
assert.strictEqual(reset.cursor, 3);
assert.strictEqual(reset.draft, '');

// ── filterCompletions ───────────────────────────────────────────────────────

assert.deepStrictEqual(
  filterCompletions(['look', 'north', 'northeast', 'northwest', 'new'], 'no'),
  ['north', 'northeast', 'northwest'],
  'prefix filter is case-insensitive and sorted'
);
assert.deepStrictEqual(filterCompletions(['Look', 'look', 'LOOK'], 'lo'), ['Look'], 'dedup keeps first spelling');
assert.deepStrictEqual(filterCompletions(['a', 'b'], ''), ['a', 'b'], 'empty prefix returns all');
assert.deepStrictEqual(filterCompletions([{ command: 'kill' }, { name: 'Ranger' }], 'k'), ['kill']);
assert.deepStrictEqual(filterCompletions(null, 'x'), []);

// ── autocompleteSource ──────────────────────────────────────────────────────

const commands = ['look', 'north', 'northeast', 'kill'];
const players = ['Alice', 'Aldric', 'Bob'];

let src = autocompleteSource('nor', { commands, players });
assert.strictEqual(src.kind, 'command');
assert.strictEqual(src.prefix, '');
assert.deepStrictEqual(src.values, ['north', 'northeast']);

src = autocompleteSource('tell Al', { commands, players });
assert.strictEqual(src.kind, 'player');
assert.strictEqual(src.prefix, 'tell ');
assert.deepStrictEqual(src.values, ['Aldric', 'Alice']);

src = autocompleteSource('tell ', { commands, players });
assert.deepStrictEqual(src.values, ['Aldric', 'Alice', 'Bob'], 'trailing space offers every player (sorted)');

src = autocompleteSource('tell Z', { commands, players });
assert.deepStrictEqual(src.values, []);

// ── tabComplete: first press + cycling ──────────────────────────────────────

let tc = tabComplete('nor', { commands, players });
assert.strictEqual(tc.matched, true);
assert.strictEqual(tc.input, 'north');
assert.deepStrictEqual(tc.cycle, ['north', 'northeast']);
assert.strictEqual(tc.cursor, 1);

tc = tabComplete(tc.input, { cycle: tc.cycle, cursor: tc.cursor });
assert.strictEqual(tc.input, 'northeast');
tc = tabComplete(tc.input, { cycle: tc.cycle, cursor: tc.cursor });
assert.strictEqual(tc.input, 'north', 'cycle wraps back to the first candidate');

tc = tabComplete('tell Al', { commands, players });
assert.strictEqual(tc.input, 'tell Aldric');
tc = tabComplete(tc.input, { cycle: tc.cycle, cursor: tc.cursor });
assert.strictEqual(tc.input, 'tell Alice');

const miss = tabComplete('zzz', { commands, players });
assert.strictEqual(miss.matched, false);
assert.strictEqual(miss.input, 'zzz');
assert.strictEqual(miss.cycle, null);

// ── persistence: load/save round-trip ───────────────────────────────────────

function fakeStorage(seed) {
  const map = new Map(seed ? Object.entries(seed) : []);
  return {
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => map.set(k, String(v)),
    removeItem: (k) => map.delete(k),
    _map: map,
  };
}

const storage = fakeStorage();
assert.strictEqual(saveHistory(['look', 'north', 'look'], storage), true);
assert.deepStrictEqual(loadHistory(storage), ['look', 'north', 'look']);
assert.ok(storage.getItem(HISTORY_STORAGE_KEY).includes('north'));

// Corrupt payload returns [].
assert.deepStrictEqual(loadHistory(fakeStorage({ [HISTORY_STORAGE_KEY]: '{not json' })), []);
assert.deepStrictEqual(loadHistory(fakeStorage({ [HISTORY_STORAGE_KEY]: '{"a":1}' })), []);
assert.deepStrictEqual(loadHistory(fakeStorage()), []);

// Load caps and drops blanks.
const many = Array.from({ length: 80 }, (_, i) => `c${i}`);
assert.strictEqual(loadHistory(fakeStorage({ [HISTORY_STORAGE_KEY]: JSON.stringify([...many, '']) })).length, HISTORY_LIMIT);
assert.deepStrictEqual(loadHistory(null), [], 'missing storage is safe');
assert.strictEqual(saveHistory(['x'], null), false, 'missing storage is safe');

// Throwing storage is swallowed.
const boom = {
  getItem: () => { throw new Error('nope'); },
  setItem: () => { throw new Error('nope'); },
};
assert.deepStrictEqual(loadHistory(boom), []);
assert.strictEqual(saveHistory(['x'], boom), false);

console.log('commandHistory_test.mjs ok');
