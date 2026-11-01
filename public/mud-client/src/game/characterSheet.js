/**
 * Character sheet view helpers (pure, testable).
 *
 * The character widget renders a Gear tab and a Hotbar strip from data the
 * server already sends:
 * - `inventoryUpdate` → `$store.equippedItems` (`{ [slot]: item }`)
 * - `characterStats.equippedSkills` → numeric cast order
 * - interface `hotbarBinds` → player 1–9 hotbar
 *
 * Nothing here reads stores or sends commands; the Svelte component does that.
 * Equipment slot ordering / icons / quality colours mirror EquipmentWidget so
 * both views agree on every engine slot (`boots`, never `feet`).
 */
import {
  EQUIPMENT_SLOTS,
  canonicalSkillId,
  canonicalSkillName,
  equipmentEntries,
  hotbarSkillSlots,
  hotbarSlotForSkill,
} from './spellbook.js';

/**
 * Quality → border/name colour, matching EquipmentWidget.getQualityColor.
 * Unknown or missing quality falls back to the muted "empty" slate.
 */
export const QUALITY_COLORS = {
  magic: '#22c55e',
  rare: '#3b82f6',
  legendary: '#a855f7',
  mythic: '#f59e0b',
  default: 'rgba(148, 163, 184, 0.45)',
};

export const EMPTY_SLOT_COLOR = 'rgba(148, 163, 184, 0.45)';

export function equipmentQualityColor(quality) {
  const key = String(quality || '').toLowerCase();
  return QUALITY_COLORS[key] || QUALITY_COLORS.default;
}

/** Uppercase quality tag for badges; empty string for normal/unknown quality. */
export function equipmentQualityLabel(quality) {
  const key = String(quality || '').toLowerCase();
  if (!key || key === 'normal' || key === 'common') return '';
  return key.toUpperCase();
}

const SLOT_BY_KEY = new Map(EQUIPMENT_SLOTS.map((slot) => [slot.key, slot]));

/** Material glyph for an engine slot key (`help_outline` for unknown keys). */
export function equipmentSlotIcon(key) {
  return SLOT_BY_KEY.get(String(key || ''))?.icon || 'help_outline';
}

/** Human label for an engine slot key; falls back to the raw key. */
export function equipmentSlotLabel(key) {
  return SLOT_BY_KEY.get(String(key || ''))?.label || String(key || '');
}

/**
 * All gear slots in display order. Every row is always present so the sheet
 * shows empty slots too:
 *   { key, label, icon, item, empty, name, quality, qualityLabel, color }
 */
export function characterEquipmentRows(equippedItems) {
  return equipmentEntries(equippedItems).map((entry) => {
    const item = entry.item;
    const quality = item?.quality || '';
    return {
      key: entry.key,
      label: entry.label,
      icon: entry.icon,
      item,
      empty: !item,
      name: item?.name || entry.label,
      quality,
      qualityLabel: item ? equipmentQualityLabel(quality) : '',
      color: item ? equipmentQualityColor(quality) : null,
    };
  });
}

/** How many gear slots hold an item (for the tab badge). */
export function countEquippedSlots(rows) {
  if (!Array.isArray(rows)) return 0;
  return rows.filter((row) => row && !row.empty).length;
}

/** Hotbar rows whose bind is a skill (1-based `slot`). */
export function hotbarSkillRows(binds) {
  return hotbarSkillSlots(binds);
}

/**
 * Equipped skills in server slot order, decorated for the sheet:
 *   { id, name, castSlot (1-based), hotbarSlot (1-based, 0 = unbound) }
 * Blank entries are skipped.
 */
export function equippedSkillRows(equippedIds, binds) {
  const list = Array.isArray(equippedIds) ? equippedIds : [];
  return list
    .map((raw, index) => {
      const id = canonicalSkillId(raw);
      if (!id) return null;
      return {
        id,
        name: canonicalSkillName(id) || String(raw),
        castSlot: index + 1,
        hotbarSlot: hotbarSlotForSkill(binds, id),
      };
    })
    .filter(Boolean);
}

/** True when this equipped skill is also pinned to the 1–9 hotbar. */
export function isSkillOnHotbar(binds, idOrName) {
  return hotbarSlotForSkill(binds, idOrName) > 0;
}
