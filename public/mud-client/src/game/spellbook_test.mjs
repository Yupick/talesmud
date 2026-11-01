import assert from 'assert';
import {
  EQUIPMENT_SLOTS,
  EQUIPMENT_SLOT_ORDER,
  equipmentEntries,
  filledEquipmentSlots,
  hotbarSkillSlots,
  hotbarSlotForSkill,
  equippedSlotForSkill,
  buildSpellbookRows,
  spellbookRowCount,
  canonicalSkillId,
  canonicalSkillName,
} from './spellbook.js';
import { makeActionBind, makeSkillBind, makeItemBind } from './hudPrefs.js';

// ── Equipment slots ──
assert.strictEqual(EQUIPMENT_SLOTS.length, EQUIPMENT_SLOT_ORDER.length);
assert.deepStrictEqual(EQUIPMENT_SLOT_ORDER[0], 'main_hand');
assert.deepStrictEqual(EQUIPMENT_SLOT_ORDER[EQUIPMENT_SLOT_ORDER.length - 1], 'ring2');

const empty = equipmentEntries({});
assert.strictEqual(empty.length, EQUIPMENT_SLOTS.length);
assert.ok(empty.every((slot) => slot.item === null), 'no items equipped by default');
assert.strictEqual(empty[0].key, 'main_hand');

const equipped = {
  head: { name: 'Leather Cap', quality: 'normal' },
  main_hand: { name: 'Rusty Sword', quality: 'magic' },
  ring2: { name: 'Copper Ring' },
};
const entries = equipmentEntries(equipped);
assert.strictEqual(entries[0].key, 'main_hand');
assert.strictEqual(entries[0].item.name, 'Rusty Sword');
const filled = filledEquipmentSlots(equipped);
assert.deepStrictEqual(
  filled.map((s) => s.key),
  ['main_hand', 'head', 'ring2'],
  'filled slots keep display order'
);
assert.strictEqual(filledEquipmentSlots({}).length, 0);

// ── Hotbar skill slots ──
const binds = [
  makeItemBind({ name: 'Potion', templateId: 'potion' }),
  makeSkillBind('warrior_brace'),
  null,
  makeSkillBind('warrior_slam'),
  makeActionBind('rest'),
];
const skillSlots = hotbarSkillSlots(binds);
assert.deepStrictEqual(
  skillSlots.map((s) => [s.id, s.slot]),
  [
    ['warrior_brace', 2],
    ['warrior_slam', 4],
  ],
  'only skill binds are listed, 1-based'
);
assert.strictEqual(hotbarSlotForSkill(binds, 'warrior_brace'), 2);
assert.strictEqual(hotbarSlotForSkill(binds, 'Brace'), 2, 'matches by display name');
assert.strictEqual(hotbarSlotForSkill(binds, 'warrior_slam'), 4);
assert.strictEqual(hotbarSlotForSkill(binds, 'warrior_stand'), 0, 'unbound skill is 0');
assert.strictEqual(hotbarSkillSlots([null, null]).length, 0);

// ── Equipped slot order ──
const equippedIds = ['warrior_brace', 'warrior_slam'];
assert.strictEqual(equippedSlotForSkill(equippedIds, 'warrior_brace'), 1);
assert.strictEqual(equippedSlotForSkill(equippedIds, 'Slam'), 2, 'matches by name');
assert.strictEqual(equippedSlotForSkill(equippedIds, 'warrior_stand'), 0);
assert.strictEqual(equippedSlotForSkill([], 'warrior_brace'), 0);

// ── Identity helpers ──
assert.strictEqual(canonicalSkillId('Brace'), 'warrior_brace');
assert.strictEqual(canonicalSkillName('warrior_brace'), 'Brace');

// ── Spellbook rows ──
const rows = buildSpellbookRows({
  classId: 'warrior',
  level: 8,
  equippedIds,
  hotbarBinds: binds,
});
assert.deepStrictEqual(
  rows.equipped.map((s) => s.id),
  ['warrior_brace', 'warrior_slam']
);
const braceRow = rows.equipped[0];
assert.strictEqual(braceRow.equippedSlot, 1);
assert.strictEqual(braceRow.hotbarSlot, 2);

const slamRow = rows.equipped[1];
assert.strictEqual(slamRow.equippedSlot, 2);
assert.strictEqual(slamRow.hotbarSlot, 4);

const standRow = rows.available.find((s) => s.id === 'warrior_stand');
assert.ok(standRow, 'stand is unlocked at level 8');
assert.strictEqual(standRow.equippedSlot, 0);
assert.strictEqual(standRow.hotbarSlot, 0);

const lowRows = buildSpellbookRows({ classId: 'warrior', level: 1, equippedIds: [], hotbarBinds: [] });
assert.ok(lowRows.locked.some((s) => s.id === 'warrior_slam'), 'locked under level');
assert.strictEqual(spellbookRowCount(rows), rows.equipped.length + rows.available.length + rows.locked.length);
assert.strictEqual(spellbookRowCount(null), 0);

console.log('spellbook_test: ok');
