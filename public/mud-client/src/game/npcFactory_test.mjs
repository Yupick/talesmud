import assert from 'node:assert/strict';
import {
  NPCS_STORAGE_KEY,
  sampleTemplates,
  emptyTemplate,
  emptyLootEntry,
  normalizeTemplate,
  normalizeLootEntry,
  validateTemplate,
  issueCounts,
  serializeTemplate,
  deserializeTemplate,
  uniqueId,
  duplicateTemplate,
  toNpcPayload,
  fromNpcPayload,
  toTemplatePayload,
  fromTemplatePayload,
  loadTemplates,
  saveTemplates,
  resetTemplates,
} from './npcFactory.js';

function memoryStorage() {
  const map = new Map();
  return {
    getItem: (key) => (map.has(key) ? map.get(key) : null),
    setItem: (key, value) => map.set(key, String(value)),
    removeItem: (key) => map.delete(key),
    _map: map,
  };
}

// Fixtures are deep-copied and independent.
{
  const a = sampleTemplates();
  const b = sampleTemplates();
  assert.equal(a.length, 3);
  a[0].name = 'Mutated';
  assert.notEqual(b[0].name, 'Mutated');
  assert.equal(a[0].id, 'goblin_scout');
}

// Every fixture passes validation with no errors.
{
  for (const tpl of sampleTemplates()) {
    const issues = validateTemplate(tpl, { otherIds: [] });
    const errors = issues.filter((i) => i.level === 'error');
    assert.deepEqual(errors, [], `fixture ${tpl.id} should have no errors: ${JSON.stringify(errors)}`);
  }
}

// Sparse rows normalize to the full editor shape and carry defaults.
{
  const tpl = normalizeTemplate({ id: 'x', name: 'X' });
  const base = emptyTemplate();
  assert.equal(tpl.role, base.role);
  assert.equal(tpl.level, base.level);
  assert.equal(tpl.maxHp, base.maxHp);
  assert.equal(tpl.aggressive, true);
  assert.deepEqual(tpl.loot, []);
  assert.equal(tpl.isTemplate, true);

  // Backend-ish keys are accepted.
  const fromApi = normalizeTemplate({ id: 'y', maxHitPoints: 33, attackPower: 9, goldDrop: { min: 1, max: 5 } });
  assert.equal(fromApi.maxHp, 33);
  assert.equal(fromApi.damageMax, 9);
  assert.equal(fromApi.goldMin, 1);
  assert.equal(fromApi.goldMax, 5);

  const loot = normalizeLootEntry({ item: 'pelt', qty: 3 });
  assert.deepEqual(loot, { itemId: 'pelt', chance: 100, quantity: 3 });
  assert.deepEqual(emptyLootEntry(), { itemId: '', chance: 100, quantity: 1 });
}

// Validation catches identity, range, cross-field and loot problems.
{
  const bad = normalizeTemplate({
    id: 'Bad Id!',
    name: '',
    role: 'dragon',
    creatureType: 'nope',
    combatStyle: 'nope',
    difficulty: 'nope',
    level: 0,
    maxHp: 0,
    damageMin: 10,
    damageMax: 5,
    aggroRadius: 99,
    goldMin: 10,
    goldMax: 1,
    spawnCount: -1,
    respawnSeconds: -5,
    loot: [
      { itemId: '', chance: 200, quantity: 0 },
      { itemId: 'x', chance: 50, quantity: 1 },
      { itemId: 'x', chance: 50, quantity: 1 },
    ],
  });
  const issues = validateTemplate(bad, { otherIds: [] });
  const fields = issues.map((i) => i.field);
  assert.ok(fields.includes('id'));
  assert.ok(fields.includes('name'));
  assert.ok(fields.includes('role'));
  assert.ok(fields.includes('creatureType'));
  assert.ok(fields.includes('combatStyle'));
  assert.ok(fields.includes('difficulty'));
  assert.ok(fields.includes('level'));
  assert.ok(fields.includes('maxHp'));
  assert.ok(fields.includes('damageMax'));
  assert.ok(fields.includes('aggroRadius'));
  assert.ok(fields.includes('goldMax'));
  assert.ok(fields.includes('spawnCount'));
  assert.ok(fields.includes('respawnSeconds'));
  assert.ok(fields.includes('loot[0].itemId'));
  assert.ok(fields.includes('loot[0].chance'));
  assert.ok(fields.includes('loot[0].quantity'));
  assert.equal(issues.find((i) => i.field === 'loot[2].itemId').level, 'warn');
  assert.equal(issueCounts(issues).errors > 0, true);
}

// Id collisions are reported against sibling ids.
{
  const issues = validateTemplate(
    { id: 'dup', name: 'Dup' },
    { otherIds: ['dup', 'other'] }
  );
  assert.equal(issues.find((i) => i.field === 'id').level, 'error');

  const own = validateTemplate({ id: 'dup', name: 'Dup' }, { otherIds: [] });
  assert.equal(own.some((i) => i.field === 'id'), false);
}

// Cross-field warnings: hostile + passive, aggressive + zero radius, no spawn.
{
  const issues = validateTemplate(
    { id: 'mob', name: 'Mob', role: 'enemy', aggressive: false, aggroRadius: 0, xpReward: 0, spawnRoomId: '' },
    { otherIds: [] }
  );
  const byField = Object.fromEntries(issues.map((i) => [i.field, i]));
  assert.equal(byField.aggressive.level, 'warn');
  assert.equal(byField.xpReward.level, 'warn');
  assert.equal(byField.spawnRoomId.level, 'warn');

  const aggressive = validateTemplate(
    { id: 'mob', name: 'Mob', role: 'guard', aggressive: true, aggroRadius: 0, spawnRoomId: 'r1' },
    { otherIds: [] }
  );
  assert.equal(aggressive.find((i) => i.field === 'aggroRadius').level, 'warn');
}

// issueCounts splits errors and warnings.
{
  const counts = issueCounts([
    { level: 'error' },
    { level: 'warn' },
    { level: 'warn' },
    { level: 'info' },
  ]);
  assert.deepEqual(counts, { errors: 1, warnings: 2 });
  assert.deepEqual(issueCounts(), { errors: 0, warnings: 0 });
}

// uniqueId / duplicateTemplate.
{
  assert.equal(uniqueId('goblin', []), 'goblin');
  assert.equal(uniqueId('goblin', ['goblin']), 'goblin_2');
  assert.equal(uniqueId('goblin', ['goblin', 'goblin_2']), 'goblin_3');
  assert.equal(uniqueId('Forest Wolf!', []), 'forest_wolf');

  const copy = duplicateTemplate(sampleTemplates()[0], ['goblin_scout']);
  assert.equal(copy.id, 'goblin_scout_copy');
  assert.equal(copy.name, 'Goblin Scout (copy)');
}

// Serialization round trip and corrupt input.
{
  const tpl = sampleTemplates()[0];
  const text = serializeTemplate(tpl);
  assert.deepEqual(deserializeTemplate(text), normalizeTemplate(tpl));
  assert.equal(deserializeTemplate('{not json'), null);
  assert.equal(deserializeTemplate(''), null);
  assert.equal(deserializeTemplate(null), null);
  assert.equal(deserializeTemplate('42'), null);
}

// Backend payload mapping round trips the core stats.
{
  const tpl = sampleTemplates()[0];
  const payload = toNpcPayload(tpl);
  assert.equal(payload.id, 'goblin_scout');
  assert.equal(payload.maxHitPoints, 24);
  assert.equal(payload.currentHitPoints, 24);
  assert.equal(payload.isTemplate, true);
  assert.equal(payload.enemyTrait.attackPower, 5);
  assert.equal(payload.enemyTrait.aggroOnSight, true);
  assert.equal(payload.enemyTrait.goldDrop.max, 4);
  assert.deepEqual(payload.enemyTrait.guaranteedLoot, []);

  const guaranteed = toNpcPayload({ id: 'a', name: 'A', role: 'enemy', loot: [{ itemId: 'relic', chance: 100, quantity: 1 }] });
  assert.deepEqual(guaranteed.enemyTrait.guaranteedLoot, ['relic']);

  // Non-hostile, non-aggressive templates omit the enemy trait.
  const guard = toNpcPayload({ id: 'g', name: 'G', role: 'guard', aggressive: false });
  assert.equal(guard.enemyTrait, undefined);

  const back = fromNpcPayload(payload);
  assert.equal(back.id, 'goblin_scout');
  assert.equal(back.maxHp, 24);
  assert.equal(back.damageMax, 5);
  assert.equal(back.level, 3);
  assert.equal(back.respawnSeconds, 120);
  assert.equal(back.spawnRoomId, 'forest_edge');
}

// Batch 21 template contract mapping: toTemplatePayload picks every flat DTO
// field faithfully (no enemyTrait wrapping, seconds stay seconds) and omits
// server-owned `updatedAt`.
{
  const tpl = normalizeTemplate({
    ...sampleTemplates()[0],
    behavior: {
      stateMachine: [{ from: 'idle', event: 'aggro', to: 'combat' }],
      aggro: { radius: 2, conditions: ['hp_below_50'] },
      abilities: [{ id: 'bite', cooldown: 3, range: 1 }],
    },
    scaling: { targetLevel: 9 },
    updatedAt: '2026-01-01T00:00:00Z',
  });
  const payload = toTemplatePayload(tpl);

  assert.equal(payload.id, 'goblin_scout');
  assert.equal(payload.name, 'Goblin Scout');
  assert.equal(payload.description, tpl.description);
  assert.equal(payload.role, 'enemy');
  assert.equal(payload.creatureType, 'humanoid');
  assert.equal(payload.combatStyle, 'melee');
  assert.equal(payload.difficulty, 'easy');
  assert.equal(payload.level, 3);
  assert.equal(payload.maxHp, 24);
  assert.equal(payload.damageMin, 2);
  assert.equal(payload.damageMax, 5);
  assert.equal(payload.aggressive, true);
  assert.equal(payload.aggroRadius, 1);
  assert.equal(payload.xpReward, 12);
  assert.equal(payload.goldMin, 0);
  assert.equal(payload.goldMax, 4);
  assert.equal(payload.lootTableId, '');
  assert.deepEqual(payload.loot, [{ itemId: 'goblin_ear', chance: 60, quantity: 1 }]);
  assert.equal(payload.spawnRoomId, 'forest_edge');
  assert.equal(payload.spawnCount, 1);
  assert.equal(payload.respawnSeconds, 120); // seconds, not nanoseconds
  assert.equal(payload.isTemplate, true);
  assert.deepEqual(payload.behavior.aggro, { radius: 2, conditions: ['hp_below_50'] });
  assert.deepEqual(payload.behavior.abilities, [{ id: 'bite', cooldown: 3, range: 1 }]);
  assert.deepEqual(payload.scaling, { targetLevel: 9 });
  assert.equal(payload.updatedAt, undefined);
  assert.equal(payload.enemyTrait, undefined);

  // Round trip: fromTemplatePayload(toTemplatePayload(t)) === normalizeTemplate(t)
  // (minus server-owned metadata such as `updatedAt`).
  const { updatedAt: _ignored, ...withoutMeta } = tpl;
  assert.deepEqual(fromTemplatePayload(payload), normalizeTemplate(withoutMeta));
  for (const fixture of sampleTemplates()) {
    assert.deepEqual(
      fromTemplatePayload(toTemplatePayload(fixture)),
      normalizeTemplate(fixture),
      `round trip failed for ${fixture.id}`
    );
  }

  // Mutating the payload must not alias the source template's loot/behavior.
  payload.loot[0].itemId = 'mutated';
  payload.behavior.abilities[0].id = 'mutated';
  assert.equal(tpl.loot[0].itemId, 'goblin_ear');
  assert.equal(tpl.behavior.abilities[0].id, 'bite');
}

// fromTemplatePayload accepts sparse summary rows (list endpoint) and blank
// input, filling the editor defaults including behavior/scaling/loot.
{
  const summary = fromTemplatePayload({
    id: 'wolf',
    name: 'Wolf',
    role: 'enemy',
    creatureType: 'beast',
    level: 5,
    maxHp: 40,
    damageMin: 3,
    damageMax: 7,
    aggressive: true,
    xpReward: 18,
    goldMin: 0,
    goldMax: 2,
    spawnRoomId: 'deep_woods',
    updatedAt: '2026-01-01T00:00:00Z',
  });
  assert.equal(summary.combatStyle, 'melee');
  assert.equal(summary.spawnCount, 1);
  assert.equal(summary.respawnSeconds, 0);
  assert.deepEqual(summary.loot, []);
  assert.deepEqual(summary.scaling, { targetLevel: null });
  assert.equal(summary.behavior.stateMachine.length, 2);

  const blank = fromTemplatePayload(null);
  assert.equal(blank.id, 'new_npc');
  assert.equal(blank.name, 'New NPC');
}

// Persistence round trip, corruption fallback and reset.
{
  const storage = memoryStorage();
  assert.equal(loadTemplates(storage).length, 3);

  const templates = sampleTemplates();
  templates[0].name = 'Renamed Scout';
  assert.equal(saveTemplates(templates, storage), true);
  assert.equal(storage.getItem(NPCS_STORAGE_KEY).includes('Renamed Scout'), true);
  assert.equal(loadTemplates(storage)[0].name, 'Renamed Scout');

  storage.setItem(NPCS_STORAGE_KEY, '{not json');
  assert.equal(loadTemplates(storage).length, 3);

  storage.setItem(NPCS_STORAGE_KEY, JSON.stringify([]));
  assert.equal(loadTemplates(storage).length, 3);

  assert.equal(resetTemplates(storage), true);
  assert.equal(loadTemplates(storage)[0].name, 'Goblin Scout');
}

// No storage available → fixture, and saves report failure instead of throwing.
{
  assert.equal(loadTemplates(null).length, 3);
  assert.equal(saveTemplates([], null), false);
  assert.equal(resetTemplates(null), false);
}

console.log('npcFactory_test ok');
