import assert from 'node:assert/strict';
import {
  validateClass,
  validateSkill,
  validateClasses,
  issueCounts,
  previewSkill,
  previewContextFor,
  estimatedMaxHp,
  schoolById,
  isDamageEffect,
  isHealEffect,
  isHotEffect,
  MAX_LEVEL,
} from './balanceRules.js';

const finding = (issues, field) => issues.find((i) => i.field === field);
const hasField = (issues, field, level) =>
  issues.some((i) => i.field === field && (!level || i.level === level));

// A fully specified martial class should pass with no issues at all.
const warrior = {
  id: 'warrior',
  name: 'Warrior',
  school: 'martial',
  caster: false,
  hpMultiplier: 1.2,
  hotbarCap: 4,
  attributes: { str: 14, dex: 7, int: 4, wis: 5, sta: 20 },
  skills: [
    { id: 'warrior_brace', name: 'Brace', effect: 'brace', level: 1, cooldown: 0, multiplier: 0, target: 'enemy', oncePerFight: true, tooltip: 'Halve the next hit.' },
    { id: 'warrior_slam', name: 'Slam', effect: 'slam', level: 4, cooldown: 4, multiplier: 1.4, target: 'enemy', oncePerFight: false, tooltip: 'Heavy swing.' },
    { id: 'warrior_stand', name: 'Stand', effect: 'stand', level: 8, cooldown: 0, multiplier: 0, target: 'enemy', oncePerFight: true, tooltip: 'Redirect hits.' },
  ],
};

{
  const issues = validateClass(warrior, [warrior]);
  assert.deepEqual(issues, [], `clean warrior should have no issues, got ${JSON.stringify(issues)}`);
}

// Empty school is a hard error on both the class and a skill override.
{
  const cls = { ...warrior, school: '' };
  assert.ok(hasField(validateClass(cls, [cls]), 'school', 'error'));
  // A skill with no school inherits the class; an empty class school is the error.
  const skill = { ...warrior.skills[1], school: '' };
  assert.deepEqual(validateSkill(skill, { classDef: { ...warrior, school: 'martial' } }).filter((i) => i.field === 'skills.school' || i.field === 'school'), []);
  assert.ok(hasField(validateSkill(skill, { classDef: { ...warrior, school: '' } }), 'school', 'error'));
}

// Unknown school is rejected.
{
  const cls = { ...warrior, school: 'laser' };
  assert.ok(hasField(validateClass(cls, [cls]), 'school', 'error'));
  assert.equal(schoolById('laser'), null);
  assert.equal(schoolById('fire').caster, true);
}

// Mana cost versus unlock level.
{
  const caster = { id: 'mage', name: 'Mage', school: 'arcane', caster: true, hpMultiplier: 0.75, hotbarCap: 4, mana: 40, skills: [] };
  const cheap = { id: 'mage_sear', name: 'Sear', effect: 'sear', level: 4, cooldown: 4, multiplier: 1.8, target: 'enemy', oncePerFight: false, tooltip: 'Arcane cast.', component: 'arcane_dust', manaCost: 18 };
  assert.deepEqual(validateSkill(cheap, { classDef: caster }), []);

  const pricey = { ...cheap, manaCost: 40 };
  assert.ok(hasField(validateSkill(pricey, { classDef: caster }), 'manaCost', 'error'));

  const mid = { ...cheap, manaCost: 25 };
  assert.ok(hasField(validateSkill(mid, { classDef: caster }), 'manaCost', 'warn'));

  const physical = { ...cheap, manaCost: 8 };
  assert.ok(hasField(validateSkill(physical, { classDef: warrior }), 'manaCost', 'warn'));
}

// Damage multiplier band.
{
  const base = { id: 'mage_sear', name: 'Sear', effect: 'sear', level: 4, cooldown: 4, target: 'enemy', oncePerFight: false, tooltip: 'x', component: 'arcane_dust', school: 'arcane' };
  assert.ok(hasField(validateSkill({ ...base, multiplier: 3 }, {}), 'multiplier', 'error'));
  assert.ok(hasField(validateSkill({ ...base, multiplier: 2.2 }, {}), 'multiplier', 'warn'));
  assert.equal(isDamageEffect('sear'), true);
}

// Heal and HoT bands + duration.
{
  const healer = { ...warrior, id: 'cleric', name: 'Cleric', school: 'divine', caster: true };
  const mend = { id: 'mage_mend', name: 'Mend', effect: 'mend', level: 6, cooldown: 5, multiplier: 0.2, target: 'self', oncePerFight: false, tooltip: 'self heal', component: 'arcane_dust', school: 'arcane', manaCost: 12 };
  assert.deepEqual(validateSkill(mend, { classDef: healer }), []);
  assert.ok(hasField(validateSkill({ ...mend, multiplier: 0.8 }, { classDef: healer }), 'multiplier', 'error'));
  assert.ok(hasField(validateSkill({ ...mend, multiplier: 0.4 }, { classDef: healer }), 'multiplier', 'warn'));
  assert.ok(hasField(validateSkill({ ...mend, target: 'enemy' }, { classDef: healer }), 'target', 'error'));
  assert.equal(isHealEffect('mend'), true);

  const renew = { id: 'mage_renew', name: 'Renew', effect: 'renew', level: 8, cooldown: 0, multiplier: 0.05, duration: 3, target: 'ally', oncePerFight: false, tooltip: 'hot', component: 'nature_leaf', school: 'nature', manaCost: 8 };
  assert.deepEqual(validateSkill(renew, { classDef: healer }), []);
  const noDuration = { ...renew, duration: 0 };
  assert.ok(hasField(validateSkill(noDuration, { classDef: healer }), 'duration', 'error'));
  assert.equal(isHotEffect('renew'), true);
}

// Unlock level consistency across a class.
{
  const cls = {
    ...warrior,
    skills: [
      { ...warrior.skills[0], level: 3 },
      { ...warrior.skills[1], level: 6 },
      { ...warrior.skills[2], level: 4 },
    ],
  };
  const issues = validateClass(cls, [cls]);
  assert.ok(hasField(issues, 'skills[0].level', 'warn'), 'first skill not level 1');
  assert.ok(hasField(issues, 'skills[2].level', 'warn'), 'unlock level goes backwards');
}

// Bounds and level cap.
{
  const cls = { ...warrior, level: undefined, skills: [{ ...warrior.skills[0], level: MAX_LEVEL + 1 }] };
  const issues = validateClass(cls, [cls]);
  assert.ok(hasField(issues, 'skills[0].level', 'error'));

  const badCap = { ...warrior, hotbarCap: 0 };
  assert.ok(hasField(validateClass(badCap, [badCap]), 'hotbarCap', 'error'));

  const badHp = { ...warrior, hpMultiplier: 0 };
  assert.ok(hasField(validateClass(badHp, [badHp]), 'hpMultiplier', 'error'));
}

// Duplicate skill ids and duplicate class ids.
{
  const cls = { ...warrior, skills: [warrior.skills[0], { ...warrior.skills[1], id: 'warrior_brace' }] };
  const issues = validateClass(cls, [cls]);
  assert.ok(hasField(issues, 'skills[1].id', 'error'));

  const dupA = { ...warrior, skills: [] };
  const dupB = { ...warrior, name: 'Warrior Two' };
  const catalogIssues = validateClasses([dupA, dupB]);
  assert.ok(catalogIssues.some((i) => i.field.startsWith('classes[') && i.level === 'error'));
  assert.ok(catalogIssues.every((i) => i.level === 'error' || i.level === 'warn'));
}

// issueCounts.
{
  const counts = issueCounts([
    { level: 'error', field: 'a', msg: 'x' },
    { level: 'warn', field: 'b', msg: 'y' },
    { level: 'warn', field: 'c', msg: 'z' },
  ]);
  assert.deepEqual(counts, { errors: 1, warnings: 2 });
}

// estimatedMaxHp.
assert.equal(estimatedMaxHp({ sta: 20 }), 120);
assert.equal(estimatedMaxHp({}), 20);

// Preview: damage is deterministic and grows with level / primary attribute.
{
  const ctx = previewContextFor(warrior, { level: 4, primaryValue: 14 });
  const preview = previewSkill(warrior.skills[1], ctx);
  assert.equal(preview.kind, 'damage');
  assert.equal(preview.perTick, 36.62);
  assert.equal(preview.min, 31.13);
  assert.equal(preview.max, 42.12);

  const low = previewSkill(warrior.skills[1], previewContextFor(warrior, { level: 1, primaryValue: 14 }));
  assert.ok(preview.perTick > low.perTick, 'higher unlock level deals more damage');
  assert.ok(preview.notes.some((n) => n.includes('Martial')));
}

// Preview: heal is a fraction of max HP.
{
  const mend = { effect: 'mend', multiplier: 0.2, duration: 0 };
  const preview = previewSkill(mend, { school: 'divine', level: 6, maxHp: 100 });
  assert.equal(preview.kind, 'heal');
  assert.equal(preview.perTick, 24);
  assert.equal(preview.min, 21.6);
  assert.equal(preview.max, 26.4);
}

// Preview: HoT multiplies per tick by duration.
{
  const renew = { effect: 'renew', multiplier: 0.05, duration: 3 };
  const preview = previewSkill(renew, { school: 'nature', level: 8, maxHp: 100 });
  assert.equal(preview.kind, 'hot');
  assert.equal(preview.perTick, 5.75);
  assert.equal(preview.total, 17.25);
  assert.equal(preview.ticks, 3);
}

// Preview: utility keeps zeros.
{
  const preview = previewSkill({ effect: 'brace', multiplier: 0, duration: 0 }, previewContextFor(warrior));
  assert.equal(preview.kind, 'utility');
  assert.equal(preview.total, 0);
}

console.log('balanceRules_test ok');
