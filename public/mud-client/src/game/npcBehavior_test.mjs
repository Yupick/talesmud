import assert from 'node:assert/strict';
import {
  SCALING_RATES,
  SCALING_LEVEL_RANGE,
  growthFactor,
  scaleTemplate,
  previewScaling,
  defaultBehavior,
  defaultScaling,
  emptyTransition,
  emptyAbility,
  normalizeBehavior,
  normalizeScaling,
  validateBehavior,
  behaviorStates,
  toLuaBehavior,
} from './npcBehavior.js';
import { normalizeTemplate, emptyTemplate } from './npcFactory.js';

const baseTpl = () => ({
  id: 'rat',
  name: 'Giant Rat',
  level: 1,
  maxHp: 10,
  damageMin: 1,
  damageMax: 3,
  xpReward: 4,
  goldMin: 0,
  goldMax: 2,
  loot: [{ itemId: 'rat_tail', chance: 50, quantity: 1 }],
});

// ---------------------------------------------------------------------------
// Scaling
// ---------------------------------------------------------------------------

// Growth factors are documented and deterministic.
{
  assert.equal(growthFactor(1, SCALING_RATES.hp), 1);
  assert.equal(growthFactor(10, SCALING_RATES.hp), 1 + 0.15 * 9);
  assert.equal(growthFactor(10, SCALING_RATES.damage), 1 + 0.1 * 9);
  assert.equal(growthFactor(10, SCALING_RATES.xp), 1 + 0.25 * 9);
  assert.equal(growthFactor(10, SCALING_RATES.gold), 1 + 0.2 * 9);
}

// Level 1 → 1 is a no-op on stats; every field comes back unchanged.
{
  const tpl = baseTpl();
  const scaled = scaleTemplate(tpl, 1);
  assert.equal(scaled.level, 1);
  assert.equal(scaled.maxHp, 10);
  assert.equal(scaled.damageMin, 1);
  assert.equal(scaled.damageMax, 3);
  assert.equal(scaled.xpReward, 4);
  assert.equal(scaled.goldMin, 0);
  assert.equal(scaled.goldMax, 2);
  assert.equal(scaled.id, 'rat');
  assert.deepEqual(scaled.loot, tpl.loot);
}

// Scaling to the template's own level is also a no-op (relative growth = 1).
{
  const tpl = { ...baseTpl(), level: 5, maxHp: 40, damageMin: 3, damageMax: 7 };
  const scaled = scaleTemplate(tpl, 5);
  assert.equal(scaled.maxHp, 40);
  assert.equal(scaled.damageMin, 3);
  assert.equal(scaled.damageMax, 7);
}

// High levels scale up with the documented formulas.
{
  const tpl = baseTpl();
  const scaled = scaleTemplate(tpl, 10);
  // hp:  round(10 * (1 + 0.15*9)) — float gives 23.4999… → 23.
  assert.equal(scaled.level, 10);
  assert.equal(scaled.maxHp, 23);
  assert.equal(scaled.damageMin, 2); // round(1 * 1.9) = 2
  assert.equal(scaled.damageMax, 6); // round(3 * 1.9) = 6 (5.7)
  assert.equal(scaled.xpReward, 13); // round(4 * 3.25)
  assert.equal(scaled.goldMax, 6); // round(2 * 2.8)
}

// Scaling down toward level 1 shrinks stats.
{
  const tpl = { ...baseTpl(), level: 10, maxHp: 24, damageMin: 2, damageMax: 6, xpReward: 13, goldMax: 6 };
  const scaled = scaleTemplate(tpl, 1);
  assert.equal(scaled.level, 1);
  assert.equal(scaled.maxHp, 10); // round(24 / 2.35) = 10
  assert.equal(scaled.damageMin, 1); // round(2 / 1.9)
  assert.equal(scaled.xpReward, 4); // round(13 / 3.25)
}

// Input is never mutated; the result is a fresh object (immutability).
{
  const tpl = baseTpl();
  const snapshot = JSON.stringify(tpl);
  const scaled = scaleTemplate(tpl, 20);
  assert.equal(JSON.stringify(tpl), snapshot);
  assert.notEqual(scaled, tpl);
  assert.notEqual(scaled.loot, tpl.loot);
  scaled.loot[0].itemId = 'mutated';
  assert.equal(tpl.loot[0].itemId, 'rat_tail');
  assert.equal(scaled.maxHp, 39); // round(10 * (1 + 0.15*19)) = round(38.5) = 39

  // Nested behavior/scaling blocks are copied too, never shared by reference.
  const withBehavior = {
    ...baseTpl(),
    behavior: {
      stateMachine: [{ from: 'idle', event: 'aggro', to: 'combat' }],
      aggro: { radius: 1, conditions: ['hp_below_50'] },
      abilities: [],
    },
    scaling: { targetLevel: 3 },
  };
  const scaledNested = scaleTemplate(withBehavior, 12);
  assert.notEqual(scaledNested.behavior, withBehavior.behavior);
  assert.notEqual(scaledNested.behavior.aggro, withBehavior.behavior.aggro);
  assert.notEqual(scaledNested.scaling, withBehavior.scaling);
  scaledNested.behavior.aggro.radius = 9;
  scaledNested.scaling.targetLevel = 99;
  assert.equal(withBehavior.behavior.aggro.radius, 1);
  assert.deepEqual(withBehavior.scaling, { targetLevel: 3 });
}

// Degenerate inputs: clamped levels, floors on stats, min/max order kept.
{
  const clampedHigh = scaleTemplate(baseTpl(), 999);
  assert.equal(clampedHigh.level, SCALING_LEVEL_RANGE.max);
  const clampedLow = scaleTemplate({ ...baseTpl(), level: 10 }, 0);
  assert.equal(clampedLow.level, 1);

  const zero = scaleTemplate({ level: 1, maxHp: 0, damageMin: 0, damageMax: 0, xpReward: 0 }, 50);
  assert.equal(zero.maxHp, 1); // HP floor
  assert.equal(zero.damageMin, 0);
  assert.equal(zero.xpReward, 0);

  const degenerate = scaleTemplate({ level: 2, damageMin: 5, damageMax: 5 }, 3);
  assert.ok(degenerate.damageMax >= degenerate.damageMin);
}

// previewScaling compares levels in input order without mutating.
{
  const tpl = baseTpl();
  const rows = previewScaling(tpl, [1, 5, 10]);
  assert.equal(rows.length, 3);
  assert.deepEqual(rows.map((r) => r.level), [1, 5, 10]);
  assert.equal(rows[0].maxHp, 10);
  assert.equal(rows[1].maxHp, 16); // round(10 * (1 + 0.15*4))
  assert.equal(rows[2].maxHp, 23);
  // Monotonically non-decreasing HP across levels.
  assert.ok(rows[1].maxHp >= rows[0].maxHp && rows[2].maxHp >= rows[1].maxHp);
  assert.equal(tpl.maxHp, 10);
  assert.deepEqual(previewScaling(tpl, []), []);
}

// ---------------------------------------------------------------------------
// Behavior normalization + validation
// ---------------------------------------------------------------------------

// Defaults fill in for sparse/legacy data.
{
  const behavior = normalizeBehavior(undefined);
  assert.equal(behavior.stateMachine.length, 2);
  assert.equal(behavior.stateMachine[0].from, 'idle');
  assert.equal(behavior.stateMachine[0].to, 'combat');
  assert.deepEqual(behavior.aggro, { radius: 0, conditions: [] });
  assert.deepEqual(behavior.abilities, []);

  const blank = normalizeBehavior({ stateMachine: [{ from: '  ', event: '', to: null }], abilities: [{}] });
  // Blank rows are kept so validation can flag them.
  assert.deepEqual(blank.stateMachine, [{ from: '', event: '', to: '' }]);
  assert.deepEqual(blank.abilities, [{ id: '', cooldown: 0, range: 1 }]);

  assert.deepEqual(defaultScaling(), { targetLevel: null });
  assert.deepEqual(normalizeScaling({ targetLevel: 'x' }), { targetLevel: null });
  assert.deepEqual(normalizeScaling({ targetLevel: 12 }), { targetLevel: 12 });
  assert.deepEqual(normalizeScaling({ targetLevel: 250 }), { targetLevel: SCALING_LEVEL_RANGE.max });
  assert.deepEqual(emptyTransition(), { from: 'idle', event: 'aggro', to: 'combat' });
  assert.deepEqual(emptyAbility(), { id: '', cooldown: 0, range: 1 });
  assert.deepEqual(normalizeBehavior(defaultBehavior()).stateMachine.length, 2);
}

// A well-formed behavior validates cleanly.
{
  const issues = validateBehavior({
    stateMachine: [
      { from: 'idle', event: 'aggro', to: 'combat' },
      { from: 'combat', event: 'target_lost', to: 'idle' },
    ],
    aggro: { radius: 2, conditions: ['hp_below_50'] },
    abilities: [{ id: 'bite', cooldown: 5, range: 1 }],
  });
  assert.deepEqual(issues, []);
  assert.deepEqual(validateBehavior(defaultBehavior()), []);
}

// Invalid behaviors: empty states/events, negative cooldown/range, bad radius.
{
  const issues = validateBehavior({
    stateMachine: [{ from: '', event: '', to: '' }],
    aggro: { radius: -1, conditions: [''] },
    abilities: [{ id: '', cooldown: -3, range: -1 }],
  });
  const fields = issues.map((i) => i.field);
  assert.ok(fields.includes('behavior.stateMachine[0].from'));
  assert.ok(fields.includes('behavior.stateMachine[0].event'));
  assert.ok(fields.includes('behavior.stateMachine[0].to'));
  assert.ok(fields.includes('behavior.aggro.radius'));
  assert.ok(fields.includes('behavior.aggro.conditions[0]'));
  assert.ok(fields.includes('behavior.abilities[0].id'));
  assert.ok(fields.includes('behavior.abilities[0].cooldown'));
  assert.ok(fields.includes('behavior.abilities[0].range'));
  assert.equal(issues.every((i) => i.level === 'error'), true);
}

// Warnings: empty machine, duplicate transitions, duplicate ability ids, big radius.
{
  const issues = validateBehavior({
    stateMachine: [
      { from: 'idle', event: 'aggro', to: 'combat' },
      { from: 'idle', event: 'aggro', to: 'combat' },
    ],
    aggro: { radius: 20, conditions: [] },
    abilities: [
      { id: 'bite', cooldown: 0, range: 1 },
      { id: 'bite', cooldown: 1, range: 1 },
    ],
  });
  const warns = issues.filter((i) => i.level === 'warn');
  assert.ok(warns.some((i) => i.field === 'behavior.stateMachine[1].event'));
  assert.ok(warns.some((i) => i.field === 'behavior.aggro.radius'));
  assert.ok(warns.some((i) => i.field === 'behavior.abilities[1].id'));

  const empty = validateBehavior({ stateMachine: [], aggro: { radius: 0, conditions: [] }, abilities: [] });
  assert.equal(empty[0].field, 'behavior.stateMachine');
  assert.equal(empty[0].level, 'warn');
}

// ---------------------------------------------------------------------------
// behaviorStates + Lua export
// ---------------------------------------------------------------------------

// States are collected in first-appearance order.
{
  assert.deepEqual(behaviorStates(defaultBehavior()), ['idle', 'combat']);
  assert.deepEqual(
    behaviorStates({
      stateMachine: [
        { from: 'combat', event: 'enrage', to: 'enraged' },
        { from: 'idle', event: 'aggro', to: 'combat' },
      ],
    }),
    ['combat', 'enraged', 'idle']
  );
  assert.deepEqual(behaviorStates({ stateMachine: [] }), []);
}

// Lua export is a deterministic, commented preview string.
{
  const behavior = {
    stateMachine: [
      { from: 'idle', event: 'aggro', to: 'combat' },
      { from: 'combat', event: 'target_lost', to: 'idle' },
    ],
    aggro: { radius: 2, conditions: ['hp_below_50'] },
    abilities: [{ id: 'bite', cooldown: 5, range: 1 }],
  };
  const lua = toLuaBehavior(behavior, { npcId: 'wolf', npcName: 'Forest Wolf' });
  assert.equal(typeof lua, 'string');
  assert.ok(lua.includes('PREVIEW / EXPORT ONLY (NOT EXECUTED)'));
  assert.ok(lua.includes('-- NPC: wolf (Forest Wolf)'));
  assert.ok(lua.includes('"idle"'));
  assert.ok(lua.includes('"combat"'));
  assert.ok(lua.includes('{ from = "idle", event = "aggro", to = "combat" },'));
  assert.ok(lua.includes('radius = 2'));
  assert.ok(lua.includes('"hp_below_50"'));
  assert.ok(lua.includes('{ id = "bite", cooldown = 5, range = 1 },'));
  assert.ok(lua.includes('function behavior.on_event'));
  assert.ok(lua.trimEnd().endsWith('return behavior'));

  // Deterministic: same input → same output; order preserved.
  assert.equal(toLuaBehavior(behavior, { npcId: 'wolf', npcName: 'Forest Wolf' }), lua);
  const defaultLua = toLuaBehavior(defaultBehavior());
  assert.ok(defaultLua.includes('{ from = "combat", event = "target_lost", to = "idle" },'));
  assert.ok(defaultLua.indexOf('"idle"') < defaultLua.indexOf('"combat"'));

  // Quotes/backslashes in ids are escaped, never break the string literal.
  const weird = toLuaBehavior({ stateMachine: [{ from: 'a"b', event: 'c\\d', to: 'e' }], aggro: { radius: 0, conditions: [] }, abilities: [] });
  assert.ok(weird.includes('"a\\"b"'));
  assert.ok(weird.includes('"c\\\\d"'));
}

// ---------------------------------------------------------------------------
// Integration with npcFactory.normalizeTemplate (retrocompatibility)
// ---------------------------------------------------------------------------

// Templates saved before Phase 5.3 load with behavior/scaling defaults.
{
  const legacy = normalizeTemplate({ id: 'old', name: 'Old', level: 3, maxHp: 24 });
  assert.equal(legacy.behavior.stateMachine.length, 2);
  assert.deepEqual(legacy.aggro, undefined);
  assert.deepEqual(legacy.behavior.aggro, { radius: 0, conditions: [] });
  assert.deepEqual(legacy.scaling, { targetLevel: null });

  const withBehavior = normalizeTemplate({
    id: 'new',
    name: 'New',
    behavior: { stateMachine: [{ from: 'idle', event: 'spot', to: 'hunt' }], aggro: { radius: 3, conditions: ['day'] }, abilities: [{ id: 'howl', cooldown: 30, range: 5 }] },
    scaling: { targetLevel: 7 },
  });
  assert.equal(withBehavior.behavior.stateMachine[0].to, 'hunt');
  assert.equal(withBehavior.behavior.aggro.radius, 3);
  assert.deepEqual(withBehavior.behavior.abilities, [{ id: 'howl', cooldown: 30, range: 5 }]);
  assert.deepEqual(withBehavior.scaling, { targetLevel: 7 });

  // emptyTemplate also carries the blocks so new templates round-trip cleanly.
  const base = emptyTemplate();
  assert.ok(base.behavior);
  assert.ok(base.scaling);
  const roundTrip = normalizeTemplate(normalizeTemplate(base));
  assert.deepEqual(roundTrip.behavior, base.behavior);
  assert.deepEqual(roundTrip.scaling, base.scaling);
}

console.log('npcBehavior_test ok');
