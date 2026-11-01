/**
 * Spellbook / loadout view helpers (pure, testable).
 *
 * Joins the three sources the client already has:
 * - class skill catalog (via hudPrefs.classifySkills)
 * - character stats `equippedSkills` (server slot order → numeric cast shortcut)
 * - interface `hotbarBinds` (player 1–9 hotbar)
 *
 * Nothing here sends commands or reads stores; UI components do that.
 */
import {
  classifySkills,
  normalizeHotbarBinds,
  skillById,
  skillDisplayName,
} from './hudPrefs.js';

/**
 * Equipment slots in display order. Keys match the server payload /
 * EquipmentWidget (`$store.equippedItems`). Icons are Material glyphs.
 */
export const EQUIPMENT_SLOT_ORDER = [
  'main_hand',
  'off_hand',
  'head',
  'neck',
  'chest',
  'waist',
  'hands',
  'legs',
  'boots',
  'ring1',
  'ring2',
];

export const EQUIPMENT_SLOTS = [
  { key: 'main_hand', label: 'Main Hand', icon: 'bolt' },
  { key: 'off_hand', label: 'Off Hand', icon: 'shield' },
  { key: 'head', label: 'Head', icon: 'face' },
  { key: 'neck', label: 'Neck', icon: 'more_horiz' },
  { key: 'chest', label: 'Chest', icon: 'checkroom' },
  { key: 'waist', label: 'Waist', icon: 'horizontal_rule' },
  { key: 'hands', label: 'Hands', icon: 'front_hand' },
  { key: 'legs', label: 'Legs', icon: 'straighten' },
  { key: 'boots', label: 'Boots', icon: 'do_not_step' },
  { key: 'ring1', label: 'Ring 1', icon: 'circle' },
  { key: 'ring2', label: 'Ring 2', icon: 'circle' },
];

/** Ordered slot rows with whatever item is currently equipped (or null). */
export function equipmentEntries(equippedItems) {
  const items = equippedItems && typeof equippedItems === 'object' ? equippedItems : {};
  return EQUIPMENT_SLOTS.map((slot) => ({
    key: slot.key,
    label: slot.label,
    icon: slot.icon,
    item: items[slot.key] || null,
  }));
}

/** Only slots that hold an item, in display order. */
export function filledEquipmentSlots(equippedItems) {
  return equipmentEntries(equippedItems).filter((slot) => !!slot.item);
}

/** Canonical skill id for an id or display name (falls back to the raw string). */
export function canonicalSkillId(idOrName) {
  const raw = String(idOrName || '').trim();
  if (!raw) return '';
  const skill = skillById(raw);
  return skill?.id || raw;
}

/** Canonical display name for an id or name. */
export function canonicalSkillName(idOrName) {
  const id = canonicalSkillId(idOrName);
  if (!id) return '';
  const skill = skillById(id);
  return skill?.name || skillDisplayName(id);
}

/**
 * Hotbar skill rows: one per filled slot whose bind is a skill.
 * slot is 1-based (what the player sees on the bar).
 */
export function hotbarSkillSlots(binds) {
  const slots = normalizeHotbarBinds(binds);
  return slots
    .map((bind, index) => {
      if (!bind || bind.kind !== 'skill') return null;
      const id = canonicalSkillId(bind.id || bind.name);
      if (!id) return null;
      const skill = skillById(id);
      return {
        id,
        name: skill?.name || bind.name || skillDisplayName(id),
        slot: index + 1,
        index,
      };
    })
    .filter(Boolean);
}

/** 1-based hotbar slot holding this skill, or 0 when it is not bound. */
export function hotbarSlotForSkill(binds, idOrName) {
  const id = canonicalSkillId(idOrName);
  if (!id) return 0;
  const name = canonicalSkillName(id).toLowerCase();
  for (const entry of hotbarSkillSlots(binds)) {
    if (entry.id === id) return entry.slot;
    if (String(entry.name || '').toLowerCase() === name) return entry.slot;
  }
  return 0;
}

/** 1-based equipped-skill slot (server numeric cast shortcut), or 0. */
export function equippedSlotForSkill(equippedIds, idOrName) {
  const list = Array.isArray(equippedIds) ? equippedIds : [];
  const id = canonicalSkillId(idOrName);
  if (!id) return 0;
  const name = canonicalSkillName(id).toLowerCase();
  for (let i = 0; i < list.length; i++) {
    const entryId = canonicalSkillId(list[i]);
    if (entryId && entryId === id) return i + 1;
    if (canonicalSkillName(entryId).toLowerCase() === name) return i + 1;
  }
  return 0;
}

/**
 * Spellbook sections with loadout decoration.
 * Each row keeps the skill fields and gains:
 * - equippedSlot: 1-based equipped slot, 0 when not equipped
 * - hotbarSlot: 1-based hotbar slot, 0 when not bound
 */
export function buildSpellbookRows({ classId, level, equippedIds, hotbarBinds } = {}) {
  const classified = classifySkills(classId, level, equippedIds);
  const decorate = (skill) => {
    const key = skill?.id || skill?.name;
    return {
      ...skill,
      equippedSlot: equippedSlotForSkill(equippedIds, key),
      hotbarSlot: hotbarSlotForSkill(hotbarBinds, key),
    };
  };
  return {
    equipped: classified.equipped.map(decorate),
    available: classified.available.map(decorate),
    locked: classified.locked.map(decorate),
  };
}

/** Total rows the spellbook shows (equipped + available + locked). */
export function spellbookRowCount(rows) {
  if (!rows) return 0;
  return (rows.equipped?.length || 0) + (rows.available?.length || 0) + (rows.locked?.length || 0);
}
