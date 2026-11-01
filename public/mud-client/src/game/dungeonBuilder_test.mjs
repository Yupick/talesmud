import assert from 'node:assert/strict';
import {
  createGrid,
  setCell,
  paintCell,
  addRoom,
  addCorridor,
  autoValidate,
  serialize,
  deserialize,
  addEncounter,
  addLootTable,
  exportDefinition,
} from './dungeonBuilder.js';

// Creación
{
  const g = createGrid(16, 16);
  assert.equal(g.width, 16);
  assert.equal(g.height, 16);
  assert.equal(g.cells[0][0].type, 'empty');
}

// Pintado y setCell
{
  const g = createGrid(5, 5);
  assert.equal(setCell(g, 0, 0, 'room'), true);
  assert.equal(g.cells[0][0].type, 'room');
  // paint cell fills connected empty? set a small area
  setCell(g, 1, 0, 'empty');
  setCell(g, 2, 0, 'empty');
  const painted = paintCell(g, 1, 0, 'corridor');
  assert.ok(painted >= 1);
}

// Validación de conectividad (caso válido e inválido)
{
  // caso inválido: sin entrada/salida
  const g = createGrid(10, 10);
  assert.equal(g.cells[0][0].type, 'empty');
  const issues = autoValidate(g);
  const errors = issues.filter((i) => i.level === 'error');
  assert.ok(errors.length >= 1); // falta entrada/salida
}

// caso válido
{
  const g = createGrid(10, 10);
  setCell(g, 1, 1, 'entrance');
  setCell(g, 8, 8, 'exit');
  // add corridor connecting
  addCorridor(g, [
    [1, 1],
    [2, 1],
    [3, 1],
    [4, 1],
    [5, 1],
    [6, 1],
    [7, 1],
    [8, 1],
    [8, 2],
    [8, 3],
    [8, 4],
    [8, 5],
    [8, 6],
    [8, 7],
    [8, 8],
  ]);
  const issues = autoValidate(g);
  const errors = issues.filter((i) => i.level === 'error');
  assert.deepEqual(errors, []);
}

// rooms solapadas
{
  const g = createGrid(10, 10);
  addRoom(g, { x: 0, y: 0, w: 3, h: 3 });
  addRoom(g, { x: 2, y: 2, w: 3, h: 3 });
  const issues = autoValidate(g);
  const errors = issues.filter((i) => i.level === 'error');
  assert.ok(errors.some((e) => e.msg.includes('overlap')));
}

// serialize round-trip
{
  const g = createGrid(8, 8);
  setCell(g, 0, 0, 'entrance');
  setCell(g, 7, 7, 'exit');
  addCorridor(g, [[0, 0], [1, 0], [7, 0], [7, 7]]);
  const ser = serialize(g);
  const g2 = deserialize(ser);
  assert.notEqual(g2, null);
  assert.equal(g2.width, 8);
  assert.equal(g2.cells[0][0].type, 'entrance');
  assert.equal(g2.cells[7][7].type, 'exit');
}

// encounters/loot
{
  const g = createGrid(5, 5);
  const enc = addEncounter(g, { x: 1, y: 1, npcTemplateId: 'goblin', count: 2, level: 3 });
  assert.notEqual(enc, null);
  assert.equal(g.cells[1][1].encounter.count, 2);
  const lt = addLootTable(g, { name: 'basic', entries: [{ itemId: 'coin', chance: 50, qty: 5 }] });
  assert.equal(lt.name, 'basic');
  const def = exportDefinition(g);
  assert.equal(def.version, 1);
  assert.ok(Array.isArray(def.encounters));
  assert.ok(Array.isArray(def.lootTables));
}

console.log('dungeonBuilder_test ok');
