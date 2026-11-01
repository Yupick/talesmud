/**
 * Class / skill editor model (Phase 5.2).
 *
 * Holds the demo fixture catalog, id helpers and the localStorage persistence
 * used while the backend class-CRUD endpoints are still pending. The editor
 * component (`ui/ClassesEditor.svelte`) owns the reactive state; everything in
 * here is pure and can be unit tested without a DOM.
 *
 * Shape mirrors `data/classes/*.yaml` (`pkg/classkit/catalog.go`) so a fixture
 * can round-trip to the pack once `src/api/classes.js` gains real endpoints.
 */
import { SCHOOLS, COMPONENTS } from './balanceRules.js';

export const CLASSES_STORAGE_KEY = 'mud-client.classes-editor.v1';

export const ARMOR_TYPES = ['Cloth', 'Leather', 'Plate'];
export const COMBAT_TYPES = ['Melee', 'Magic'];
export const ATTRIBUTES = ['str', 'dex', 'int', 'wis', 'sta'];
export const PRIMARY_ATTRIBUTES = ['STR', 'DEX', 'INT', 'WIS'];
export const TARGETS = ['enemy', 'all_enemies', 'ally', 'all_allies', 'self'];

export { SCHOOLS, COMPONENTS };

/** Deep clone that works in the browser and in Node tests. */
export function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function slug(value, fallback) {
  const base = String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');
  return base || fallback;
}

/** Start-of-class defaults. `id` is a safe, unique-ish seed the user can edit. */
export function emptyClass() {
  return {
    id: 'new_class',
    name: 'New Class',
    description: '',
    school: 'martial',
    armorType: 'Leather',
    combatType: 'Melee',
    primary: 'STR',
    order: 99,
    caster: false,
    hpMultiplier: 1.0,
    hotbarCap: 4,
    mana: 0,
    attributes: { str: 10, dex: 10, int: 10, wis: 10, sta: 10 },
    balance: { damageDealt: 1.0, damageTaken: 1.0, behindDealt: 1.0, swings: 1 },
    hotbar: [],
    skills: [],
  };
}

/** A blank skill for a class. Keeps ids prefixed `<class>_<effect>` like the pack. */
export function emptySkill(classId = '', effect = 'slam') {
  const prefix = slug(classId, 'class');
  return {
    id: `${prefix}_${slug(effect, 'skill')}`,
    name: 'New Skill',
    effect,
    school: '',
    level: 1,
    manaCost: 0,
    component: '',
    multiplier: 1.0,
    duration: 0,
    cooldown: 0,
    oncePerFight: false,
    keepsSwing: false,
    target: 'enemy',
    tooltip: '',
  };
}

/** Coerce a possibly-sparse class row into the full editor shape. */
export function normalizeClass(raw = {}) {
  const base = emptyClass();
  const attributes = raw.attributes || raw.template || {};
  const skills = Array.isArray(raw.skills) ? raw.skills.map(normalizeSkill) : [];
  return {
    ...base,
    ...raw,
    attributes: {
      str: numOr(attributes.str, base.attributes.str),
      dex: numOr(attributes.dex, base.attributes.dex),
      int: numOr(attributes.int, base.attributes.int),
      wis: numOr(attributes.wis, base.attributes.wis),
      sta: numOr(attributes.sta, base.attributes.sta),
    },
    balance: { ...base.balance, ...(raw.balance || {}) },
    hotbar: Array.isArray(raw.hotbar) ? raw.hotbar.slice() : skills.slice(0, base.hotbarCap).map((s) => s.id),
    skills,
  };
}

export function normalizeSkill(raw = {}) {
  const base = emptySkill(raw.classId || '', raw.effect || 'slam');
  return {
    ...base,
    ...raw,
    level: numOr(raw.level, base.level),
    manaCost: numOr(raw.manaCost, base.manaCost),
    multiplier: numOr(raw.multiplier, base.multiplier),
    duration: numOr(raw.duration, base.duration),
    cooldown: numOr(raw.cooldown, base.cooldown),
    oncePerFight: raw.oncePerFight ?? raw.once_per_fight ?? false,
    keepsSwing: raw.keepsSwing ?? raw.keeps_swing ?? false,
  };
}

function numOr(value, fallback) {
  const n = typeof value === 'number' ? value : Number.parseFloat(value);
  return Number.isFinite(n) ? n : fallback;
}

/** First free `<base>`, `<base>_2`, `<base>_3` … against a list of ids. */
export function uniqueId(base, existingIds = []) {
  const used = new Set((existingIds || []).map((id) => String(id)));
  const clean = slug(base, 'class');
  if (!used.has(clean)) return clean;
  let n = 2;
  while (used.has(`${clean}_${n}`)) n += 1;
  return `${clean}_${n}`;
}

/** Deep-copy a class with a unique id, name suffix and fresh skill ownership. */
export function duplicateClass(source, existingIds = []) {
  const copy = clone(source || emptyClass());
  copy.id = uniqueId(`${copy.id}_copy`, existingIds);
  copy.name = `${copy.name || 'Class'} (copy)`;
  copy.skills = (copy.skills || []).map((skill) => {
    const next = clone(skill);
    const suffix = copy.id;
    if (next.id && !next.id.startsWith(`${suffix}_`)) {
      next.id = `${suffix}_${next.id.replace(/^[a-z0-9]+_/, '')}`;
    }
    return next;
  });
  copy.hotbar = copy.skills.slice(0, Math.max(1, copy.hotbarCap || 4)).map((s) => s.id);
  return copy;
}

// ---------------------------------------------------------------------------
// Demo fixture (used until the backend class-CRUD endpoints exist)
// ---------------------------------------------------------------------------

const FIXTURE = [
  {
    id: 'warrior',
    name: 'Warrior',
    description: 'Plate front line. Brace once, then a heavy slam.',
    school: 'martial',
    armorType: 'Plate',
    combatType: 'Melee',
    primary: 'STR',
    order: 1,
    caster: false,
    hpMultiplier: 1.2,
    hotbarCap: 4,
    mana: 0,
    attributes: { str: 14, dex: 7, int: 4, wis: 5, sta: 20 },
    balance: { damageDealt: 1.0, damageTaken: 0.9, behindDealt: 1.15, swings: 1 },
    hotbar: ['warrior_brace'],
    skills: [
      { id: 'warrior_brace', name: 'Brace', effect: 'brace', school: 'martial', level: 1, manaCost: 0, component: '', multiplier: 0, duration: 0, cooldown: 0, oncePerFight: true, keepsSwing: true, target: 'enemy', tooltip: 'Once a fight. The next hit on you is halved. You still swing.' },
      { id: 'warrior_slam', name: 'Slam', effect: 'slam', school: 'martial', level: 4, manaCost: 0, component: '', multiplier: 1.4, duration: 0, cooldown: 4, oncePerFight: false, keepsSwing: false, target: 'enemy', tooltip: 'A heavy swing, 1.40×. Replaces your swing this round.' },
      { id: 'warrior_stand', name: 'Stand', effect: 'stand', school: 'martial', level: 8, manaCost: 0, component: '', multiplier: 0, duration: 0, cooldown: 0, oncePerFight: true, keepsSwing: false, target: 'enemy', tooltip: 'Once a fight, for two rounds hits aimed at others hit you.' },
    ],
  },
  {
    id: 'mage',
    name: 'Mage',
    description: 'Arcane tower. Inscribe a burning mark, then sear.',
    school: 'arcane',
    armorType: 'Cloth',
    combatType: 'Magic',
    primary: 'INT',
    order: 3,
    caster: true,
    hpMultiplier: 0.75,
    hotbarCap: 4,
    mana: 41,
    attributes: { str: 4, dex: 6, int: 18, wis: 14, sta: 8 },
    balance: { damageDealt: 1.4, damageTaken: 1.25, behindDealt: 1.15, swings: 1 },
    hotbar: ['mage_inscribe'],
    skills: [
      { id: 'mage_inscribe', name: 'Inscribe', effect: 'inscribe', school: 'arcane', level: 1, manaCost: 0, component: 'arcane_dust', multiplier: 0, duration: 3, cooldown: 4, oncePerFight: false, keepsSwing: false, target: 'enemy', tooltip: 'Arcane. Mark them. 4 a round for 3 rounds. Refresh, no stack.' },
      { id: 'mage_sear', name: 'Sear', effect: 'sear', school: 'arcane', level: 4, manaCost: 16, component: 'arcane_dust', multiplier: 1.8, duration: 0, cooldown: 4, oncePerFight: false, keepsSwing: false, target: 'enemy', tooltip: 'An arcane cast at 1.80×. Replaces your swing this round.' },
      { id: 'mage_mend', name: 'Arcane Mend', effect: 'mend', school: 'arcane', level: 6, manaCost: 16, component: 'arcane_dust', multiplier: 0.2, duration: 0, cooldown: 5, oncePerFight: false, keepsSwing: false, target: 'self', tooltip: 'Mend yourself for 20% of your max HP.' },
      { id: 'mage_glyph', name: 'Glyph', effect: 'glyph', school: 'arcane', level: 8, manaCost: 12, component: 'arcane_dust', multiplier: 0, duration: 0, cooldown: 0, oncePerFight: true, keepsSwing: false, target: 'enemy', tooltip: 'Once a fight. The next hit on you is reduced by 4.' },
    ],
  },
  {
    id: 'cleric',
    name: 'Cleric',
    description: 'Melee bulwark with mends and a renew over time.',
    school: 'divine',
    armorType: 'Leather',
    combatType: 'Melee',
    primary: 'WIS',
    order: 4,
    caster: true,
    hpMultiplier: 1.05,
    hotbarCap: 4,
    mana: 35,
    attributes: { str: 11, dex: 7, int: 10, wis: 16, sta: 16 },
    balance: { damageDealt: 1.05, damageTaken: 1.0, behindDealt: 1.0, swings: 1 },
    hotbar: ['cleric_mend'],
    skills: [
      { id: 'cleric_mend', name: 'Mend', effect: 'mend', school: 'divine', level: 1, manaCost: 12, component: 'holy_water', multiplier: 0.2, duration: 0, cooldown: 5, oncePerFight: false, keepsSwing: false, target: 'ally', tooltip: 'Direct heal for 20% of max HP.' },
      { id: 'cleric_renew', name: 'Renew', effect: 'renew', school: 'divine', level: 4, manaCost: 10, component: 'holy_water', multiplier: 0.05, duration: 3, cooldown: 0, oncePerFight: false, keepsSwing: false, target: 'ally', tooltip: 'Heal over time, 5% a round for 3 rounds.' },
    ],
  },
];

/** Deep copy of the demo catalog (safe to mutate). */
export function sampleClasses() {
  return clone(FIXTURE);
}

// ---------------------------------------------------------------------------
// Persistence (localStorage until the backend CRUD endpoints land)
// ---------------------------------------------------------------------------

function resolveStorage(storage) {
  if (storage) return storage;
  try {
    return typeof localStorage !== 'undefined' ? localStorage : null;
  } catch (err) {
    return null;
  }
}

/**
 * Read the catalog from storage, falling back to the fixture.
 * @param {Storage} [storage]
 * @returns {Array<object>}
 */
export function loadClasses(storage) {
  const store = resolveStorage(storage);
  if (!store) return sampleClasses();
  try {
    const raw = store.getItem(CLASSES_STORAGE_KEY);
    if (!raw) return sampleClasses();
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed) || parsed.length === 0) return sampleClasses();
    return parsed.map(normalizeClass);
  } catch (err) {
    return sampleClasses();
  }
}

/**
 * Persist the catalog. Returns true on success.
 * @param {Array<object>} classes
 * @param {Storage} [storage]
 */
export function saveClasses(classes, storage) {
  const store = resolveStorage(storage);
  if (!store) return false;
  try {
    store.setItem(CLASSES_STORAGE_KEY, JSON.stringify(classes || []));
    return true;
  } catch (err) {
    return false;
  }
}

/** Drop the saved catalog so the fixture returns. */
export function resetClasses(storage) {
  const store = resolveStorage(storage);
  if (!store) return false;
  try {
    store.removeItem(CLASSES_STORAGE_KEY);
    return true;
  } catch (err) {
    return false;
  }
}
