/**
 * Class / skill balance rules (Phase 5.2) — pure, dependency-free helpers.
 *
 * This module is the single source of truth for two things:
 *   1. `validateClass` / `validateSkill` / `validateClasses` → a list of
 *      `{ level: 'error' | 'warn', field, msg }` issues the editor renders.
 *   2. `previewSkill` → the damage / heal / HoT numbers shown in the editor's
 *      preview block, derived from the same school + level model.
 *
 * The rules mirror the world-pack contract in `data/classes/README.md`
 * (schools, effects, mana vs cooldown, `mend` = direct heal, `renew` = HoT).
 * Nothing here touches the DOM, stores or the network, so it is unit tested
 * directly under `balanceRules_test.mjs`.
 */

/** Highest unlock level an editor should accept. */
export const MAX_LEVEL = 20;

// ---------------------------------------------------------------------------
// Schools (id is the YAML value; `pkg/classkit/schools.go` is the backend source)
// ---------------------------------------------------------------------------

/**
 * @typedef {Object} School
 * @property {string} id
 * @property {string} name
 * @property {number} damage multiplier applied to outgoing damage previews
 * @property {number} heal   multiplier applied to heal/HoT previews
 * @property {boolean} caster whether the school spends mana by default
 */
export const SCHOOLS = [
  { id: 'martial', name: 'Martial', damage: 1.0, heal: 1.0, caster: false },
  { id: 'fire', name: 'Fire', damage: 1.15, heal: 0.95, caster: true },
  { id: 'ice', name: 'Ice', damage: 1.05, heal: 1.0, caster: true },
  { id: 'arcane', name: 'Arcane', damage: 1.1, heal: 1.05, caster: true },
  { id: 'divine', name: 'Divine', damage: 1.0, heal: 1.2, caster: true },
  { id: 'nature', name: 'Nature', damage: 1.05, heal: 1.15, caster: true },
  { id: 'shadow', name: 'Shadow', damage: 1.1, heal: 0.95, caster: true },
];

export const SCHOOL_IDS = SCHOOLS.map((s) => s.id);

/** The school row for an id, or null when the id is unknown. */
export function schoolById(id) {
  const key = String(id || '').trim().toLowerCase();
  if (!key) return null;
  return SCHOOLS.find((school) => school.id === key) || null;
}

// ---------------------------------------------------------------------------
// Effects
// ---------------------------------------------------------------------------

/** Primitive effects from `classkit.KnownEffects` plus the generic skill effects. */
export const EFFECTS = [
  'brace',
  'slam',
  'stand',
  'slip',
  'nick',
  'smoke',
  'inscribe',
  'sear',
  'glyph',
  'guard',
  'bolt',
  'rig',
  'overload',
  'mend',
  'renew',
  'damage',
  'heal',
  'hot',
  'dot',
  'buff',
  'debuff',
];

const DAMAGE_EFFECTS = new Set(['slam', 'nick', 'sear', 'bolt', 'rig', 'overload', 'damage']);
const HEAL_EFFECTS = new Set(['mend', 'heal']);
const HOT_EFFECTS = new Set(['renew', 'hot']);
const DOT_EFFECTS = new Set(['inscribe', 'dot', 'burn']);

/** True when the effect deals direct damage. */
export function isDamageEffect(effect) {
  return DAMAGE_EFFECTS.has(String(effect || '').trim().toLowerCase());
}
/** True when the effect is a direct heal (`mend`). */
export function isHealEffect(effect) {
  return HEAL_EFFECTS.has(String(effect || '').trim().toLowerCase());
}
/** True when the effect is a heal-over-time (`renew`). */
export function isHotEffect(effect) {
  return HOT_EFFECTS.has(String(effect || '').trim().toLowerCase());
}
/** True when the effect is a damage-over-time (`inscribe`). */
export function isDotEffect(effect) {
  return DOT_EFFECTS.has(String(effect || '').trim().toLowerCase());
}

/** Component suggestions, matching the world-pack ingredients. */
export const COMPONENTS = [
  'arcane_dust',
  'fire_salt',
  'holy_water',
  'beast_fur',
  'venom_sac',
  'shadow_ash',
];

// ---------------------------------------------------------------------------
// Rule helpers
// ---------------------------------------------------------------------------

const ID_RE = /^[a-z][a-z0-9_]*$/;

function issue(level, field, msg) {
  return { level, field, msg };
}

function toNumber(value) {
  const n = typeof value === 'number' ? value : Number.parseFloat(value);
  return Number.isFinite(n) ? n : NaN;
}

function isBlank(value) {
  return value === undefined || value === null || String(value).trim() === '';
}

/**
 * Validate a single skill.
 *
 * @param {object} skill
 * @param {object} [ctx]
 * @param {object} [ctx.classDef] owning class (for school / caster defaults)
 * @param {number} [ctx.index] index in the skills array (prefixes `field`)
 * @returns {Array<{level:'error'|'warn', field:string, msg:string}>}
 */
export function validateSkill(skill, ctx = {}) {
  const issues = [];
  const prefix = Number.isInteger(ctx.index) ? `skills[${ctx.index}].` : '';
  const field = (name) => `${prefix}${name}`;
  const s = skill && typeof skill === 'object' ? skill : {};
  const classDef = ctx.classDef && typeof ctx.classDef === 'object' ? ctx.classDef : {};
  const isCaster = !!(classDef.caster || schoolById(classDef.school)?.caster);

  if (isBlank(s.name)) {
    issues.push(issue('error', field('name'), 'Skill needs a name.'));
  }
  if (isBlank(s.id)) {
    issues.push(issue('error', field('id'), 'Skill needs an id (used by the hotbar).'));
  }

  // School: the skill may override the class school. Empty/unknown is fatal.
  const school = s.school || classDef.school;
  if (isBlank(school)) {
    issues.push(issue('error', field('school'), 'School is empty — pick martial/ice/fire/arcane/divine/nature/shadow.'));
  } else if (!schoolById(school)) {
    issues.push(issue('error', field('school'), `Unknown school "${school}".`));
  }

  const effect = String(s.effect || '').trim().toLowerCase();
  if (!effect) {
    issues.push(issue('error', field('effect'), 'Pick an effect.'));
  } else if (!DAMAGE_EFFECTS.has(effect) && !HEAL_EFFECTS.has(effect) && !HOT_EFFECTS.has(effect) && !DOT_EFFECTS.has(effect) && !EFFECTS.includes(effect)) {
    issues.push(issue('error', field('effect'), `Unknown effect "${effect}".`));
  }

  // Unlock level.
  const level = toNumber(s.level);
  if (!Number.isInteger(level) || level < 1) {
    issues.push(issue('error', field('level'), 'Unlock level must be a whole number ≥ 1.'));
  } else if (level > MAX_LEVEL) {
    issues.push(issue('error', field('level'), `Unlock level ${level} is above the level cap (${MAX_LEVEL}).`));
  }

  // Cooldown.
  const cooldown = toNumber(s.cooldown);
  if (Number.isNaN(cooldown) || cooldown < 0) {
    issues.push(issue('error', field('cooldown'), 'Cooldown cannot be negative.'));
  } else if (cooldown > 12) {
    issues.push(issue('warn', field('cooldown'), `Cooldown ${cooldown} is very long for a combat skill.`));
  }
  if (s.oncePerFight && cooldown > 0) {
    issues.push(issue('warn', field('cooldown'), 'Once-per-fight skills usually have no cooldown.'));
  }

  // Mana cost, scaled against the unlock level.
  const manaCost = toNumber(s.manaCost);
  if (!Number.isNaN(manaCost) && manaCost < 0) {
    issues.push(issue('error', field('manaCost'), 'Mana cost cannot be negative.'));
  } else if (manaCost > 0) {
    if (!isCaster) {
      issues.push(issue('warn', field('manaCost'), 'A physical class spends cooldown, not mana.'));
    }
    if (Number.isInteger(level) && level >= 1) {
      const warnAt = 8 + level * 3;
      const errorAt = 12 + level * 6;
      if (manaCost > errorAt) {
        issues.push(issue('error', field('manaCost'), `Mana cost ${manaCost} is far above the level ${level} budget (${errorAt}).`));
      } else if (manaCost > warnAt) {
        issues.push(issue('warn', field('manaCost'), `Mana cost ${manaCost} is above the level ${level} budget (${warnAt}).`));
      }
    }
  }

  // Power multipliers per effect family.
  const multiplier = toNumber(s.multiplier);
  const duration = toNumber(s.duration);
  if (isDamageEffect(effect)) {
    if (multiplier <= 0) {
      issues.push(issue('warn', field('multiplier'), 'Damage skill has no swing multiplier.'));
    } else if (multiplier > 2.5) {
      issues.push(issue('error', field('multiplier'), `Multiplier ${multiplier}× is above the damage cap (2.5×).`));
    } else if (multiplier > 2.0) {
      issues.push(issue('warn', field('multiplier'), `Multiplier ${multiplier}× is at the high end of the damage band.`));
    }
  } else if (isHealEffect(effect)) {
    if (multiplier <= 0) {
      issues.push(issue('error', field('multiplier'), 'Heal skill needs a positive multiplier.'));
    } else if (multiplier > 0.5) {
      issues.push(issue('error', field('multiplier'), `Heal ${Math.round(multiplier * 100)}% of max HP is above the heal cap (50%).`));
    } else if (multiplier > 0.35) {
      issues.push(issue('warn', field('multiplier'), `Heal ${Math.round(multiplier * 100)}% of max HP is very strong.`));
    }
  } else if (isHotEffect(effect) || isDotEffect(effect)) {
    if (!(duration > 0)) {
      issues.push(issue('error', field('duration'), `${isHotEffect(effect) ? 'HoT' : 'DoT'} skills need a duration in rounds.`));
    }
    if (isHotEffect(effect)) {
      if (multiplier <= 0) {
        issues.push(issue('error', field('multiplier'), 'HoT needs a positive per-round multiplier.'));
      } else if (multiplier > 0.5) {
        issues.push(issue('error', field('multiplier'), `HoT tick ${Math.round(multiplier * 100)}% of max HP is above the cap (50%).`));
      } else if (multiplier > 0.25) {
        issues.push(issue('warn', field('multiplier'), `HoT tick ${Math.round(multiplier * 100)}% of max HP is strong.`));
      }
    } else if (!(multiplier >= 0)) {
      issues.push(issue('error', field('multiplier'), 'DoT needs a non-negative multiplier.'));
    }
  } else if (duration > 0) {
    issues.push(issue('warn', field('duration'), 'Duration is ignored for this effect.'));
  }

  // Tagging.
  if (s.oncePerFight === undefined && s.once_per_fight === undefined) {
    issues.push(issue('warn', field('oncePerFight'), 'Set once-per-fight (true/false).'));
  }

  // Targeting must match the effect family.
  const target = String(s.target || '').trim().toLowerCase();
  if (!target) {
    issues.push(issue('error', field('target'), 'Pick a target.'));
  } else if (isHealEffect(effect) || isHotEffect(effect)) {
    if (target !== 'ally' && target !== 'self' && target !== 'all_allies') {
      issues.push(issue('error', field('target'), 'Heal/HoT skills must target an ally or self.'));
    }
  } else if (isDamageEffect(effect) && target !== 'enemy' && target !== 'all_enemies') {
    issues.push(issue('warn', field('target'), 'Damage skills normally target an enemy.'));
  }

  // Components (Phase 9) and tooltips.
  if (isCaster && (isDamageEffect(effect) || isHealEffect(effect) || isHotEffect(effect)) && isBlank(s.component)) {
    issues.push(issue('warn', field('component'), 'Caster skills should declare a component.'));
  }
  if (isBlank(s.tooltip)) {
    issues.push(issue('warn', field('tooltip'), 'Add a short tooltip for the hotbar.'));
  }

  return issues;
}

/**
 * Validate a class definition and its embedded skills.
 *
 * @param {object} classDef
 * @param {Array<object>} [allClasses] used for duplicate-id checks at collection level
 * @returns {Array<{level:'error'|'warn', field:string, msg:string, skillIndex?:number}>}
 */
export function validateClass(classDef, allClasses = []) {
  const issues = [];
  const cls = classDef && typeof classDef === 'object' ? classDef : {};

  if (isBlank(cls.name)) {
    issues.push(issue('error', 'name', 'Class needs a name.'));
  }
  if (isBlank(cls.id)) {
    issues.push(issue('error', 'id', 'Class needs an id.'));
  } else if (!ID_RE.test(String(cls.id))) {
    issues.push(issue('error', 'id', 'Class id must be lowercase letters/digits/underscore (e.g. battle_mage).'));
  }

  if (isBlank(cls.school)) {
    issues.push(issue('error', 'school', 'School is empty — pick martial/ice/fire/arcane/divine/nature/shadow.'));
  } else if (!schoolById(cls.school)) {
    issues.push(issue('error', 'school', `Unknown school "${cls.school}".`));
  }

  const hp = toNumber(cls.hpMultiplier);
  if (Number.isNaN(hp) || hp <= 0) {
    issues.push(issue('error', 'hpMultiplier', 'HP multiplier must be a positive number.'));
  } else if (hp < 0.5 || hp > 2) {
    issues.push(issue('warn', 'hpMultiplier', `HP multiplier ${hp} is outside the usual 0.5–2.0 band.`));
  }

  const cap = toNumber(cls.hotbarCap);
  if (!Number.isInteger(cap) || cap < 1 || cap > 8) {
    issues.push(issue('error', 'hotbarCap', 'Hotbar cap must be a whole number between 1 and 8.'));
  }

  if (cls.caster && isBlank(cls.mana)) {
    issues.push(issue('warn', 'mana', 'Caster class has no starting mana.'));
  }

  const skills = Array.isArray(cls.skills) ? cls.skills : [];
  if (skills.length === 0) {
    issues.push(issue('warn', 'skills', 'Class has no skills yet.'));
  }

  const seen = new Set();
  let previous = null;
  skills.forEach((skill, index) => {
    const skillIssues = validateSkill(skill, { classDef: cls, index });
    for (const item of skillIssues) issues.push({ ...item, skillIndex: index });

    const id = skill && skill.id ? String(skill.id) : '';
    if (id) {
      if (seen.has(id)) {
        issues.push(issue('error', `skills[${index}].id`, `Duplicate skill id "${id}".`));
        issues[issues.length - 1].skillIndex = index;
      }
      seen.add(id);
    }

    const level = toNumber(skill && skill.level);
    if (Number.isInteger(level) && level >= 1) {
      if (index === 0 && level !== 1) {
        issues.push(issue('warn', `skills[${index}].level`, 'The first skill should unlock at level 1.'));
        issues[issues.length - 1].skillIndex = index;
      }
      if (previous !== null && level < previous) {
        issues.push(issue('warn', `skills[${index}].level`, 'Unlock level is lower than the previous skill — check the order.'));
        issues[issues.length - 1].skillIndex = index;
      }
      previous = level;
    }
  });

  // Duplicate class ids across the whole catalog.
  if (allClasses && Array.isArray(allClasses) && cls.id) {
    const dupes = allClasses.filter((c) => c && c.id === cls.id).length;
    if (dupes > 1) {
      issues.push(issue('error', 'id', `Class id "${cls.id}" is used by more than one class.`));
    }
  }

  return issues;
}

/**
 * Validate an entire catalog: per-class issues plus cross-class duplicate ids.
 * @param {Array<object>} classes
 */
export function validateClasses(classes) {
  const list = Array.isArray(classes) ? classes : [];
  const issues = [];
  const byId = new Map();
  list.forEach((cls, index) => {
    const id = cls && cls.id ? String(cls.id) : '';
    if (id) {
      if (!byId.has(id)) byId.set(id, []);
      byId.get(id).push(index);
    }
  });
  for (const [, indexes] of byId) {
    if (indexes.length > 1) {
      for (const index of indexes) {
        issues.push(issue('error', `classes[${index}].id`, 'Duplicate class id across the catalog.'));
      }
    }
  }
  for (const cls of list) {
    for (const item of validateClass(cls, list)) issues.push(item);
  }
  return issues;
}

/** Convenience counters for the editor header. */
export function issueCounts(issues) {
  const list = Array.isArray(issues) ? issues : [];
  return {
    errors: list.filter((i) => i.level === 'error').length,
    warnings: list.filter((i) => i.level === 'warn').length,
  };
}

// ---------------------------------------------------------------------------
// Preview
// ---------------------------------------------------------------------------

/** A naked level-1 swing before weapon/attribute bonuses. */
export const PREVIEW_BASE_HIT = 10;
/** HP added per point of stamina in the preview model. */
export const PREVIEW_HP_PER_STA = 5;
export const PREVIEW_BASE_HP = 20;
/** Unlock-level growth applied to damage previews. */
export const PREVIEW_LEVEL_GROWTH = 0.03;

/** Estimate a character's max HP from the class attribute spread. */
export function estimatedMaxHp(attributes = {}) {
  const sta = toNumber(attributes.sta ?? attributes.STA);
  return Math.max(1, Math.round(PREVIEW_BASE_HP + (Number.isNaN(sta) ? 0 : sta) * PREVIEW_HP_PER_STA));
}

/**
 * Build the preview context from a class definition (attributes + school).
 * @param {object} classDef
 * @param {{level?:number, primaryValue?:number, maxHp?:number}} [overrides]
 */
export function previewContextFor(classDef, overrides = {}) {
  const cls = classDef && typeof classDef === 'object' ? classDef : {};
  const attributes = cls.attributes || cls.template || {};
  const school = schoolById(cls.school) || SCHOOLS[0];
  const level = toNumber(overrides.level);
  const maxHp = toNumber(overrides.maxHp);
  const primaryValue = toNumber(overrides.primaryValue);
  return {
    school: school.id,
    schoolFactor: school.damage,
    healFactor: school.heal,
    level: Number.isInteger(level) && level >= 1 ? level : 1,
    primaryValue: Number.isNaN(primaryValue) ? 0 : primaryValue,
    maxHp: Number.isNaN(maxHp) ? estimatedMaxHp(attributes) : maxHp,
  };
}

function round(value) {
  return Math.round(value * 100) / 100;
}

/**
 * Compute the expected damage / heal / HoT numbers for one skill.
 * Pure and deterministic; the editor calls this to render its preview block.
 *
 * @param {object} skill
 * @param {object} [context] result of `previewContextFor` (or compatible overrides)
 * @returns {{kind:'damage'|'heal'|'hot'|'dot'|'utility', tokens:string, min:number, max:number, perTick:number, total:number, ticks:number, notes:string[]}}
 */
export function previewSkill(skill, context = {}) {
  const s = skill && typeof skill === 'object' ? skill : {};
  const school = schoolById(context.school) || SCHOOLS[0];
  const level = Number.isInteger(context.level) && context.level >= 1 ? context.level : 1;
  const primaryValue = Number.isFinite(context.primaryValue) ? context.primaryValue : 0;
  const maxHp = Number.isFinite(context.maxHp) && context.maxHp > 0 ? context.maxHp : estimatedMaxHp({});
  const multiplier = toNumber(s.multiplier);
  const duration = toNumber(s.duration);
  const effect = String(s.effect || '').trim().toLowerCase();
  const notes = [`School ${school.name} (${school.damage}× dmg / ${school.heal}× heal).`, `Level ${level}.`];

  if (isDamageEffect(effect)) {
    const mult = Number.isNaN(multiplier) ? 0 : multiplier;
    const growth = 1 + (level - 1) * PREVIEW_LEVEL_GROWTH;
    const swing = (PREVIEW_BASE_HIT + primaryValue) * mult * school.damage * growth;
    const min = Math.max(0, round(swing * 0.85));
    const max = Math.max(0, round(swing * 1.15));
    if (s.oncePerFight) notes.push('Once per fight.');
    return {
      kind: 'damage',
      tokens: `${min}–${max}`,
      min,
      max,
      perTick: round(swing),
      total: round(swing),
      ticks: 1,
      notes,
    };
  }

  if (isHealEffect(effect)) {
    const mult = Number.isNaN(multiplier) ? 0 : multiplier;
    const heal = maxHp * mult * school.heal;
    const min = Math.max(0, round(heal * 0.9));
    const max = Math.max(0, round(heal * 1.1));
    notes.push(`≈${Math.round(mult * 100)}% of max HP (${Math.round(maxHp)}).`);
    return {
      kind: 'heal',
      tokens: `${min}–${max} HP`,
      min,
      max,
      perTick: round(heal),
      total: round(heal),
      ticks: 1,
      notes,
    };
  }

  if (isHotEffect(effect)) {
    const mult = Number.isNaN(multiplier) ? 0 : multiplier;
    const ticks = Number.isInteger(duration) && duration > 0 ? duration : 0;
    const perTick = maxHp * mult * school.heal;
    const total = perTick * ticks;
    notes.push(`≈${Math.round(mult * 100)}% of max HP per round (${Math.round(maxHp)}).`);
    notes.push(`${ticks || '—'} round${ticks === 1 ? '' : 's'}.`);
    return {
      kind: 'hot',
      tokens: `${round(perTick)}/round × ${ticks || '?'} = ${round(total)} HP`,
      min: 0,
      max: round(total),
      perTick: round(perTick),
      total: round(total),
      ticks,
      notes,
    };
  }

  if (isDotEffect(effect)) {
    const mult = Number.isNaN(multiplier) ? 0 : multiplier;
    const ticks = Number.isInteger(duration) && duration > 0 ? duration : 0;
    const perTick = mult > 0 ? (PREVIEW_BASE_HIT + primaryValue) * mult * school.damage : 4;
    const total = perTick * (ticks || 3);
    notes.push(`Burns over ${ticks || 3} rounds.`);
    return {
      kind: 'dot',
      tokens: `${round(perTick)}/round × ${ticks || 3} = ${round(total)}`,
      min: 0,
      max: round(total),
      perTick: round(perTick),
      total: round(total),
      ticks: ticks || 3,
      notes,
    };
  }

  notes.push('Utility effect — no direct damage or healing.');
  return { kind: 'utility', tokens: 'utility', min: 0, max: 0, perTick: 0, total: 0, ticks: 0, notes };
}
