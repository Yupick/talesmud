import assert from 'node:assert/strict';
import {
  createGrid,
  setCell,
  addRoom,
  addCorridor,
  addEncounter,
  addLootTable,
  exportDefinition,
  DUNGEON_STORAGE_KEY,
} from './dungeonBuilder.js';
import {
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
} from './dungeonLibrary.js';

function sampleGrid() {
  const g = createGrid(10, 8);
  addRoom(g, { x: 1, y: 1, w: 3, h: 3 });
  setCell(g, 1, 1, 'entrance');
  addCorridor(g, [
    [4, 2],
    [5, 2],
    [6, 2],
    [7, 2],
  ]);
  setCell(g, 7, 2, 'exit');
  addEncounter(g, { x: 2, y: 2, npcTemplateId: 'goblin', count: 2, level: 3 });
  addLootTable(g, { name: 'basic', entries: [{ itemId: 'coin', chance: 50, qty: 5 }] });
  return g;
}

function fakeStorage(init = {}) {
  const map = new Map(Object.entries(init));
  return {
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => {
      map.set(k, String(v));
    },
    removeItem: (k) => {
      map.delete(k);
    },
    _map: map,
  };
}

// isDefinition / not-a-definition
{
  const def = exportDefinition(sampleGrid());
  assert.equal(isDefinition(def), true);
  assert.equal(isDefinition(createGrid(4, 4)), true); // plain grid shape also ok
  assert.equal(isDefinition(null), false);
  assert.equal(isDefinition('nope'), false);
  assert.equal(isDefinition({ width: 4 }), false); // no cells
}

// buildDungeonPayload
{
  const g = sampleGrid();
  const payload = buildDungeonPayload({
    name: '  Crypt of Tests  ',
    description: 'demo',
    author: 'dm',
    tags: [' small ', '', 'crypt'],
    grid: g,
  });
  assert.notEqual(payload, null);
  assert.equal(payload.name, 'Crypt of Tests');
  assert.equal(payload.description, 'demo');
  assert.equal(payload.author, 'dm');
  assert.deepEqual(payload.tags, ['small', 'crypt']);
  assert.equal(payload.gridWidth, 10);
  assert.equal(payload.gridHeight, 8);
  assert.equal(payload.definition.version, 1);
  assert.equal(payload.definition.grid.width, 10);
  assert.equal(payload.definition.grid.height, 8);
  assert.equal(payload.definition.cells.length, 8);
  assert.equal(payload.definition.cells[0].length, 10);
  assert.equal(payload.definition.meta.name, 'Crypt of Tests');
  // payload must not alias the live grid state
  payload.definition.meta.name = 'changed';
  assert.equal(g.meta.name, undefined);
}

// buildDungeonPayload: name required, grid required
{
  const g = sampleGrid();
  assert.equal(buildDungeonPayload({ name: '   ', grid: g }), null);
  assert.equal(buildDungeonPayload({ name: 'x' }), null);
  assert.equal(buildDungeonPayload({ name: 'x', grid: null }), null);
}

// definitionToGrid round-trip (exportDefinition -> grid)
{
  const g = sampleGrid();
  const def = exportDefinition(g);
  const restored = definitionToGrid(def);
  assert.notEqual(restored, null);
  assert.equal(restored.width, 10);
  assert.equal(restored.height, 8);
  assert.equal(restored.cells[1][1].type, 'entrance');
  assert.equal(restored.cells[2][7].type, 'exit');
  assert.equal(restored.rooms.length, 1);
  assert.equal(restored.rooms[0].id, 'room_1');
  assert.equal(restored.encounters.length, 1);
  assert.equal(restored.encounters[0].npcTemplateId, 'goblin');
  assert.equal(restored.lootTables.length, 1);
  // entrances/exits recomputed from cells by autoValidate
  assert.deepEqual(restored.entrances, [{ x: 1, y: 1 }]);
  assert.deepEqual(restored.exits, [{ x: 7, y: 2 }]);
}

// definitionToGrid also accepts a plain grid-shaped object
{
  const g = sampleGrid();
  const restored = definitionToGrid({ width: g.width, height: g.height, cells: g.cells, rooms: g.rooms });
  assert.notEqual(restored, null);
  assert.equal(restored.cells[1][1].type, 'entrance');
}

// definitionToGrid rejects garbage
{
  assert.equal(definitionToGrid(null), null);
  assert.equal(definitionToGrid({}), null);
  assert.equal(definitionToGrid({ grid: { width: 0, height: 0 }, cells: [] }), null);
}

// gridFromEntry reads {definition} (server GET) and {grid} entries
{
  const g = sampleGrid();
  const viaDefinition = gridFromEntry({ id: 'd1', definition: exportDefinition(g) });
  assert.equal(viaDefinition.cells[1][1].type, 'entrance');
  assert.equal(gridFromEntry(null), null);
  assert.equal(gridFromEntry({ id: 'd1' }), null);
}

// normalizeListRows: defaults, drops rows without id
{
  const rows = normalizeListRows([
    { id: 'd1', name: 'Crypt', gridWidth: '12', gridHeight: 9, tags: ['a'] },
    { name: 'no id' },
    null,
    'string',
    { id: '  ', name: 'blank id' },
    { id: 'd2' },
  ]);
  assert.equal(rows.length, 2);
  assert.equal(rows[0].id, 'd1');
  assert.equal(rows[0].name, 'Crypt');
  assert.equal(rows[0].gridWidth, 12);
  assert.equal(rows[0].gridHeight, 9);
  assert.deepEqual(rows[0].tags, ['a']);
  assert.equal(rows[1].id, 'd2');
  assert.equal(rows[1].name, 'd2'); // name falls back to id
  assert.deepEqual(rows[1].tags, []);
  assert.equal(normalizeListRows('nope').length, 0);
  assert.equal(normalizeListRows(undefined).length, 0);
}

// local library: save/load round trip, corrupt data degrades to []
{
  const store = fakeStorage();
  const g = sampleGrid();
  let entries = [];
  entries = upsertLocalEntry(entries, {
    name: 'Local One',
    description: 'd',
    definition: exportDefinition(g),
  });
  assert.equal(entries.length, 1);
  assert.ok(entries[0].id.startsWith('local_'));
  assert.equal(saveLocalEntries(entries, store), true);
  assert.ok(store._map.has(DUNGEON_STORAGE_KEY));

  const loaded = loadLocalEntries(store);
  assert.equal(loaded.length, 1);
  assert.equal(loaded[0].name, 'Local One');
  assert.equal(isDefinition(loaded[0].definition), true);
  assert.notEqual(loaded[0].definition, entries[0].definition); // deep copy

  const broken = fakeStorage({ [DUNGEON_STORAGE_KEY]: '{oops' });
  assert.deepEqual(loadLocalEntries(broken), []);
  const wrongShape = fakeStorage({ [DUNGEON_STORAGE_KEY]: '{"not":"array"}' });
  assert.deepEqual(loadLocalEntries(wrongShape), []);
  assert.deepEqual(loadLocalEntries(fakeStorage()), []);
}

// upsert by id replaces, keeps order; id-less entries get a fresh local id
{
  let entries = [];
  entries = upsertLocalEntry(entries, { id: 'd1', name: 'A' });
  entries = upsertLocalEntry(entries, { id: 'd2', name: 'B' });
  entries = upsertLocalEntry(entries, { id: 'd1', name: 'A2' });
  assert.equal(entries.length, 2);
  assert.equal(entries[0].id, 'd1');
  assert.equal(entries[0].name, 'A2');
  assert.equal(entries[1].name, 'B');

  const noId = upsertLocalEntry(entries, { name: 'C' });
  assert.equal(noId.length, 3);
  assert.ok(noId[2].id.startsWith('local_'));
  assert.equal(noId[2].name, 'C');
  // original array untouched (immutability)
  assert.equal(entries.length, 2);
}

// removeLocalEntry + clearLocalEntries
{
  const store = fakeStorage();
  let entries = upsertLocalEntry([], { id: 'd1', name: 'A' });
  entries = upsertLocalEntry(entries, { id: 'd2', name: 'B' });
  saveLocalEntries(entries, store);
  const without = removeLocalEntry(entries, 'd1');
  assert.equal(without.length, 1);
  assert.equal(without[0].id, 'd2');
  assert.equal(removeLocalEntry(entries, 'missing').length, 2);
  assert.equal(clearLocalEntries(store), true);
  assert.deepEqual(loadLocalEntries(store), []);
}

// nextLocalId is unique-ish and prefixed
{
  const a = nextLocalId([]);
  const b = nextLocalId([{ id: a }, { id: 'x' }]);
  assert.ok(a.startsWith('local_'));
  assert.ok(b.startsWith('local_'));
  assert.notEqual(a, b);
}

console.log('dungeonLibrary_test ok');
