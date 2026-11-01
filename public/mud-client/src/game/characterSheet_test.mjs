import assert from 'assert';
import {
  QUALITY_COLORS,
  characterEquipmentRows,
  countEquippedSlots,
  equippedSkillRows,
  equipmentQualityColor,
  equipmentQualityLabel,
  equipmentSlotIcon,
  equipmentSlotLabel,
  hotbarSkillRows,
  isSkillOnHotbar,
} from './characterSheet.js';
import { EQUIPMENT_SLOTS } from './spellbook.js';
import { makeActionBind, makeItemBind, makeSkillBind } from './hudPrefs.js';

// ── Slot metadata ──
assert.strictEqual(equipmentSlotIcon('head'), 'face');
assert.strictEqual(equipmentSlotIcon('boots'), 'do_not_step', 'engine slot is boots');
assert.strictEqual(equipmentSlotIcon('feet'), 'help_outline', 'feet is not an engine slot');
assert.strictEqual(equipmentSlotIcon(undefined), 'help_outline');
assert.strictEqual(equipmentSlotLabel('main_hand'), 'Main Hand');
assert.strictEqual(equipmentSlotLabel('strange'), 'strange');

// ── Quality colours ──
assert.strictEqual(equipmentQualityColor('magic'), QUALITY_COLORS.magic);
assert.strictEqual(equipmentQualityColor('RARE'), QUALITY_COLORS.rare, 'case-insensitive');
assert.strictEqual(equipmentQualityColor('mythic'), QUALITY_COLORS.mythic);
assert.strictEqual(equipmentQualityColor(''), QUALITY_COLORS.default);
assert.strictEqual(equipmentQualityColor(undefined), QUALITY_COLORS.default);
assert.strictEqual(equipmentQualityLabel('legendary'), 'LEGENDARY');
assert.strictEqual(equipmentQualityLabel('normal'), '');
assert.strictEqual(equipmentQualityLabel(''), '');

// ── Gear rows ──
const emptyRows = characterEquipmentRows({});
assert.strictEqual(emptyRows.length, EQUIPMENT_SLOTS.length, 'every slot is listed');
assert.ok(emptyRows.every((row) => row.empty && row.item === null), 'nothing equipped');
assert.strictEqual(emptyRows[0].key, 'main_hand');
assert.strictEqual(emptyRows[0].name, 'Main Hand', 'empty rows fall back to the slot label');
assert.strictEqual(emptyRows[0].color, null);
assert.strictEqual(emptyRows[0].icon, 'bolt');
assert.strictEqual(countEquippedSlots(emptyRows), 0);
assert.strictEqual(countEquippedSlots(null), 0);

const worn = {
  head: { name: 'Leather Cap', quality: 'normal' },
  main_hand: { name: 'Rusty Sword', quality: 'magic' },
  ring2: { name: 'Copper Ring' },
  // Not an engine slot: must be ignored, not rendered as a phantom row.
  feet: { name: 'Ghost Boots', quality: 'legendary' },
};
const rows = characterEquipmentRows(worn);
assert.strictEqual(rows.length, EQUIPMENT_SLOTS.length);
assert.strictEqual(countEquippedSlots(rows), 3);
assert.strictEqual(rows[0].key, 'main_hand');
assert.strictEqual(rows[0].item.name, 'Rusty Sword');
assert.strictEqual(rows[0].color, QUALITY_COLORS.magic);
assert.strictEqual(rows[0].empty, false);

const head = rows.find((row) => row.key === 'head');
assert.strictEqual(head.qualityLabel, '', 'normal quality has no badge');

const ring = rows.find((row) => row.key === 'ring2');
assert.strictEqual(ring.name, 'Copper Ring');
assert.strictEqual(ring.qualityLabel, '', 'missing quality has no badge');
assert.ok(!rows.some((row) => row.key === 'feet'), 'unknown slot keys are dropped');

// ── Hotbar rows ──
const binds = [
  makeItemBind({ name: 'Potion', templateId: 'potion' }),
  makeSkillBind('warrior_brace'),
  null,
  makeSkillBind('warrior_slam'),
  makeActionBind('rest'),
];
assert.deepStrictEqual(
  hotbarSkillRows(binds).map((row) => row.slot),
  [2, 4]
);
assert.strictEqual(hotbarSkillRows([]).length, 0);

// ── Equipped skill rows ──
const equipped = equippedSkillRows(['warrior_brace', '', 'warrior_slam'], binds);
assert.deepStrictEqual(
  equipped.map((row) => [row.id, row.castSlot, row.hotbarSlot]),
  [
    ['warrior_brace', 1, 2],
    ['warrior_slam', 3, 4],
  ],
  'castSlot follows server order, hotbarSlot follows binds'
);
assert.strictEqual(equipped[0].name, 'Brace');
assert.deepStrictEqual(equippedSkillRows(null, binds), []);
const unknown = equippedSkillRows(['not_a_real_skill'], []);
assert.strictEqual(unknown.length, 1);
assert.strictEqual(unknown[0].id, 'not_a_real_skill');
assert.strictEqual(unknown[0].castSlot, 1);
assert.strictEqual(unknown[0].hotbarSlot, 0);
assert.ok(unknown[0].name, 'unknown ids still get a display name');

// ── Hotbar pin check ──
assert.strictEqual(isSkillOnHotbar(binds, 'warrior_brace'), true);
assert.strictEqual(isSkillOnHotbar(binds, 'Brace'), true, 'matches by display name');
assert.strictEqual(isSkillOnHotbar(binds, 'warrior_stand'), false);
assert.strictEqual(isSkillOnHotbar([], 'warrior_brace'), false);

console.log('characterSheet_test: ok');
