import assert from 'node:assert/strict';
import {
  CLASSES_STORAGE_KEY,
  sampleClasses,
  emptyClass,
  emptySkill,
  normalizeClass,
  normalizeSkill,
  uniqueId,
  duplicateClass,
  loadClasses,
  saveClasses,
  resetClasses,
} from './classEditor.js';
import { validateClass } from './balanceRules.js';

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
  const a = sampleClasses();
  const b = sampleClasses();
  assert.equal(a.length, 3);
  a[0].name = 'Mutated';
  assert.notEqual(b[0].name, 'Mutated');
  assert.equal(a[0].id, 'warrior');
}

// Every fixture class passes balance validation with no errors.
{
  for (const cls of sampleClasses()) {
    const errors = validateClass(cls, sampleClasses()).filter((i) => i.level === 'error');
    assert.deepEqual(errors, [], `fixture ${cls.id} should have no errors: ${JSON.stringify(errors)}`);
  }
}

// Sparse rows normalize to the full editor shape.
{
  const cls = normalizeClass({ id: 'x', name: 'X' });
  assert.deepEqual(cls.attributes, { str: 10, dex: 10, int: 10, wis: 10, sta: 10 });
  assert.equal(cls.hpMultiplier, 1.0);
  assert.deepEqual(cls.skills, []);
  assert.deepEqual(cls.balance, { damageDealt: 1.0, damageTaken: 1.0, behindDealt: 1.0, swings: 1 });

  const skill = normalizeSkill({ id: 'x_slam', name: 'Slam', once_per_fight: true, keeps_swing: true });
  assert.equal(skill.oncePerFight, true);
  assert.equal(skill.keepsSwing, true);
  assert.equal(skill.multiplier, 1.0);
}

// emptyClass / emptySkill defaults.
{
  const cls = emptyClass();
  assert.equal(cls.school, 'martial');
  assert.equal(cls.hotbarCap, 4);
  const skill = emptySkill('Battle Mage', 'sear');
  assert.equal(skill.id, 'battle_mage_sear');
  assert.equal(skill.effect, 'sear');
}

// uniqueId picks the first free suffix.
{
  assert.equal(uniqueId('mage', []), 'mage');
  assert.equal(uniqueId('mage', ['mage']), 'mage_2');
  assert.equal(uniqueId('mage', ['mage', 'mage_2']), 'mage_3');
  assert.equal(uniqueId('My Class!', []), 'my_class');
}

// duplicateClass rewrites the id and re-parents skill ids.
{
  const copy = duplicateClass(sampleClasses()[0], ['warrior']);
  assert.equal(copy.id, 'warrior_copy');
  assert.equal(copy.name, 'Warrior (copy)');
  assert.ok(copy.skills.every((s) => s.id.startsWith('warrior_copy_')));
  assert.deepEqual(copy.hotbar, copy.skills.slice(0, copy.hotbarCap).map((s) => s.id));
}

// Persistence round trip, corruption fallback and reset.
{
  const storage = memoryStorage();
  assert.equal(loadClasses(storage).length, 3);

  const classes = sampleClasses();
  classes[0].name = 'Renamed Warrior';
  assert.equal(saveClasses(classes, storage), true);
  assert.equal(storage.getItem(CLASSES_STORAGE_KEY).includes('Renamed Warrior'), true);
  assert.equal(loadClasses(storage)[0].name, 'Renamed Warrior');

  storage.setItem(CLASSES_STORAGE_KEY, '{not json');
  assert.equal(loadClasses(storage).length, 3);

  storage.setItem(CLASSES_STORAGE_KEY, JSON.stringify([]));
  assert.equal(loadClasses(storage).length, 3);

  assert.equal(resetClasses(storage), true);
  assert.equal(loadClasses(storage)[0].name, 'Warrior');
}

// No storage available → fixture, and saves report failure instead of throwing.
{
  assert.equal(loadClasses(null).length, 3);
  assert.equal(saveClasses([], null), false);
  assert.equal(resetClasses(null), false);
}

console.log('classEditor_test ok');
