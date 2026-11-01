/**
 * NPC behavior "lite" + deterministic level scaling (Creator Phase 5.3, cut 2).
 *
 * Pure module (no DOM, no imports) so `node --test` can cover it and so
 * `npcFactory.js` can import the normalizers without cycles.
 *
 * Three concerns:
 *
 * 1. **Scaling per level** — `scaleTemplate(template, targetLevel)` produces a
 *    new template with `maxHp`, `damageMin/Max`, `xpReward` and `goldMin/Max`
 *    multiplied by a documented, deterministic growth factor (see
 *    `SCALING_RATES` below). The template's current stats are treated as the
 *    baseline at its *current* level, so re-scaling to the same level is a
 *    no-op and moving 5 → 10 applies the same relative growth as 1 → 2.35….
 *
 * 2. **Behavior data model** — a Lua behavior-tree/state-machine equivalent is
 *    out of scope for the client right now (FASE 5.3: "AI behavior trees Lua").
 *    Instead we keep a serializable model — `stateMachine` transitions, aggro
 *    config and abilities — that validates cleanly and can be exported as a
 *    commented Lua skeleton for future backend integration.
 *
 * 3. **Lua export** — `toLuaBehavior()` renders a *preview/export only* string.
 *    Nothing here is executed; the backend will consume (or ignore) it later.
 */

// ---------------------------------------------------------------------------
// Level scaling
// ---------------------------------------------------------------------------

/**
 * Per-level growth rates. Each stat grows multiplicatively:
 *
 *   growth(level) = 1 + rate * (level - 1)
 *   scaled        = round(base * growth(targetLevel) / growth(currentLevel))
 *
 * Examples with a level-1 template (growth(1) === 1):
 *   - maxHp  level 10 → ×(1 + 0.15·9) = ×2.35
 *   - damage level 10 → ×(1 + 0.10·9) = ×1.90
 *   - xp     level 10 → ×(1 + 0.25·9) = ×3.25
 *   - gold   level 10 → ×(1 + 0.20·9) = ×2.80
 *
 * Scaling is deterministic (Math.round) and never mutates the input.
 */
export const SCALING_RATES = {
  hp: 0.15,
  damage: 0.1,
  xp: 0.25,
  gold: 0.2,
};

/** Inclusive level bounds (mirrors `npcFactory.RANGES.level`). */
export const SCALING_LEVEL_RANGE = { min: 1, max: 100 };

function clampLevel(level) {
  const n = Math.trunc(Number(level));
  if (!Number.isFinite(n)) return SCALING_LEVEL_RANGE.min;
  return Math.min(SCALING_LEVEL_RANGE.max, Math.max(SCALING_LEVEL_RANGE.min, n));
}

/** Growth multiplier relative to level 1 for a given rate. */
export function growthFactor(level, rate) {
  return 1 + rate * (clampLevel(level) - 1);
}

function intOr(value, fallback) {
  const n = typeof value === 'number' ? value : Number.parseFloat(value);
  return Number.isFinite(n) ? Math.trunc(n) : fallback;
}

function scaleStat(base, fromLevel, toLevel, rate, min) {
  const factor = growthFactor(toLevel, rate) / growthFactor(fromLevel, rate);
  return Math.max(min, Math.round(intOr(base, 0) * factor));
}

/** Recursive copy of plain objects/arrays (template data is JSON-shaped). */
function deepClonePlain(value) {
  if (Array.isArray(value)) return value.map(deepClonePlain);
  if (value && typeof value === 'object') {
    const out = {};
    for (const key of Object.keys(value)) out[key] = deepClonePlain(value[key]);
    return out;
  }
  return value;
}

/**
 * Return a NEW template with combat/reward stats scaled to `targetLevel`.
 * `level`, `maxHp`, `damageMin`, `damageMax`, `xpReward`, `goldMin` and
 * `goldMax` are rewritten; every other field is deep-copied verbatim.
 * The input template is never mutated.
 *
 * @param {object} template sparse or normalized template
 * @param {number} targetLevel 1..100 (clamped)
 * @returns {object} new template object
 */
export function scaleTemplate(template = {}, targetLevel) {
  const src = template && typeof template === 'object' ? template : {};
  const fromLevel = clampLevel(src.level ?? SCALING_LEVEL_RANGE.min);
  const toLevel = clampLevel(targetLevel ?? fromLevel);

  // Deep copy without JSON round-trip edge cases for non-serializable values.
  // The scaled stats are rewritten below; every other field is copied verbatim,
  // so mutating the result never touches the source template.
  const out = deepClonePlain(src);

  out.level = toLevel;
  out.maxHp = scaleStat(src.maxHp, fromLevel, toLevel, SCALING_RATES.hp, 1);
  out.damageMin = scaleStat(src.damageMin, fromLevel, toLevel, SCALING_RATES.damage, 0);
  out.damageMax = scaleStat(src.damageMax, fromLevel, toLevel, SCALING_RATES.damage, 0);
  out.xpReward = scaleStat(src.xpReward, fromLevel, toLevel, SCALING_RATES.xp, 0);
  out.goldMin = scaleStat(src.goldMin, fromLevel, toLevel, SCALING_RATES.gold, 0);
  out.goldMax = scaleStat(src.goldMax, fromLevel, toLevel, SCALING_RATES.gold, 0);

  // Rounding can invert a degenerate min/max pair on tiny values; keep order.
  if (out.damageMax < out.damageMin) out.damageMax = out.damageMin;
  if (out.goldMax < out.goldMin) out.goldMax = out.goldMin;
  return out;
}

/**
 * Compare the template across several levels. Returns one row per requested
 * level (in input order), always including the template's current level
 * untouched. Useful for the editor preview table.
 *
 * @param {object} template
 * @param {number[]} levels
 * @returns {Array<{level:number, maxHp:number, damageMin:number, damageMax:number,
 *   xpReward:number, goldMin:number, goldMax:number}>}
 */
export function previewScaling(template = {}, levels = []) {
  const list = Array.isArray(levels) ? levels : [];
  return list.map((level) => {
    const scaled = scaleTemplate(template, level);
    return {
      level: scaled.level,
      maxHp: scaled.maxHp,
      damageMin: scaled.damageMin,
      damageMax: scaled.damageMax,
      xpReward: scaled.xpReward,
      goldMin: scaled.goldMin,
      goldMax: scaled.goldMax,
    };
  });
}

// ---------------------------------------------------------------------------
// Behavior model (lite; future Lua behavior-tree integration)
// ---------------------------------------------------------------------------

/** Blank transition row for the editor. */
export function emptyTransition() {
  return { from: 'idle', event: 'aggro', to: 'combat' };
}

/** Blank ability row for the editor. */
export function emptyAbility() {
  return { id: '', cooldown: 0, range: 1 };
}

/**
 * Default behavior block: a minimal idle ⇄ combat loop with no abilities.
 * Templates saved before FASE 5.3 get this on load (retrocompatible).
 */
export function defaultBehavior() {
  return {
    stateMachine: [
      { from: 'idle', event: 'aggro', to: 'combat' },
      { from: 'combat', event: 'target_lost', to: 'idle' },
    ],
    aggro: { radius: 0, conditions: [] },
    abilities: [],
  };
}

/** Default scaling bookkeeping (the last level the creator scaled to). */
export function defaultScaling() {
  return { targetLevel: null };
}

function strOr(value) {
  return typeof value === 'string' ? value : value == null ? '' : String(value);
}

/**
 * Coerce a sparse behavior block into the full editor shape. Blank rows are
 * KEPT (not dropped) so `validateBehavior` can flag them, mirroring the loot
 * editor philosophy in `npcFactory.js`.
 */
export function normalizeBehavior(raw = {}) {
  const src = raw && typeof raw === 'object' ? raw : {};
  const base = defaultBehavior();
  const transitions = Array.isArray(src.stateMachine)
    ? src.stateMachine.map((row = {}) => ({
        from: strOr(row.from).trim(),
        event: strOr(row.event).trim(),
        to: strOr(row.to).trim(),
      }))
    : base.stateMachine.map((row) => ({ ...row }));
  const aggroSrc = src.aggro && typeof src.aggro === 'object' ? src.aggro : {};
  const conditions = Array.isArray(aggroSrc.conditions)
    ? aggroSrc.conditions.map((c) => strOr(c).trim())
    : [];
  const abilities = Array.isArray(src.abilities)
    ? src.abilities.map((row = {}) => ({
        id: strOr(row.id).trim(),
        cooldown: intOr(row.cooldown, 0),
        range: intOr(row.range, 1),
      }))
    : [];
  return {
    stateMachine: transitions,
    aggro: {
      radius: intOr(aggroSrc.radius, base.aggro.radius),
      conditions,
    },
    abilities,
  };
}

/** Coerce sparse scaling bookkeeping. */
export function normalizeScaling(raw = {}) {
  const src = raw && typeof raw === 'object' ? raw : {};
  const level = src.targetLevel;
  if (level == null || level === '') return defaultScaling();
  const n = Math.trunc(Number(level));
  return { targetLevel: Number.isFinite(n) ? clampLevel(n) : null };
}

/**
 * Validate a behavior block. Returns `{ field, level, msg }` issues using
 * `behavior.`-prefixed field names so the factory panel can merge them with
 * template issues.
 *
 * Checks: non-empty states/events, coherent cooldowns (≥0) and ranges (≥0),
 * non-negative aggro radius, no duplicate `from+event` transitions and no
 * duplicate ability ids.
 *
 * @param {object} raw sparse or normalized behavior
 * @returns {Array<{field:string, level:'error'|'warn', msg:string}>}
 */
export function validateBehavior(raw) {
  const behavior = normalizeBehavior(raw);
  const issues = [];

  // State machine
  if (!Array.isArray(behavior.stateMachine)) {
    issues.push({ field: 'behavior.stateMachine', level: 'error', msg: 'State machine must be a list of transitions.' });
  } else {
    if (behavior.stateMachine.length === 0) {
      issues.push({
        field: 'behavior.stateMachine',
        level: 'warn',
        msg: 'No transitions: the NPC never changes state.',
      });
    }
    const seen = new Set();
    behavior.stateMachine.forEach((tr, index) => {
      const prefix = `behavior.stateMachine[${index}]`;
      for (const key of ['from', 'event', 'to']) {
        if (!tr[key]) {
          issues.push({ field: `${prefix}.${key}`, level: 'error', msg: `Transition ${key} must not be empty.` });
        }
      }
      const key = `${tr.from}|${tr.event}`;
      if (tr.from && tr.event) {
        if (seen.has(key)) {
          issues.push({
            field: `${prefix}.event`,
            level: 'warn',
            msg: `Duplicate transition for "${tr.from}" + "${tr.event}".`,
          });
        }
        seen.add(key);
      }
    });
  }

  // Aggro
  const { radius, conditions } = behavior.aggro;
  if (!Number.isInteger(radius) || radius < 0) {
    issues.push({ field: 'behavior.aggro.radius', level: 'error', msg: 'Aggro radius must be a whole number ≥ 0.' });
  } else if (radius > 10) {
    issues.push({ field: 'behavior.aggro.radius', level: 'warn', msg: 'Aggro radius above 10 is unusual (rooms).' });
  }
  if (!Array.isArray(conditions)) {
    issues.push({ field: 'behavior.aggro.conditions', level: 'error', msg: 'Aggro conditions must be a list.' });
  } else {
    conditions.forEach((cond, index) => {
      if (!cond) {
        issues.push({
          field: `behavior.aggro.conditions[${index}]`,
          level: 'error',
          msg: 'Aggro condition must not be empty.',
        });
      }
    });
  }

  // Abilities
  if (!Array.isArray(behavior.abilities)) {
    issues.push({ field: 'behavior.abilities', level: 'error', msg: 'Abilities must be a list.' });
  } else {
    const seenIds = new Set();
    behavior.abilities.forEach((ability, index) => {
      const prefix = `behavior.abilities[${index}]`;
      if (!ability.id) {
        issues.push({ field: `${prefix}.id`, level: 'error', msg: 'Ability id is required.' });
      } else if (seenIds.has(ability.id)) {
        issues.push({ field: `${prefix}.id`, level: 'warn', msg: `Duplicate ability id "${ability.id}".` });
      }
      seenIds.add(ability.id);
      if (!Number.isFinite(ability.cooldown) || ability.cooldown < 0) {
        issues.push({ field: `${prefix}.cooldown`, level: 'error', msg: 'Ability cooldown must be ≥ 0 seconds.' });
      }
      if (!Number.isFinite(ability.range) || ability.range < 0) {
        issues.push({ field: `${prefix}.range`, level: 'error', msg: 'Ability range must be ≥ 0.' });
      }
    });
  }

  return issues;
}

// ---------------------------------------------------------------------------
// Lua export (preview only — nothing is executed client- or server-side)
// ---------------------------------------------------------------------------

function luaString(value) {
  return `"${String(value).replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

/** States in order of first appearance across transitions (deterministic). */
export function behaviorStates(behavior = {}) {
  const normalized = normalizeBehavior(behavior);
  const states = [];
  const seen = new Set();
  for (const tr of normalized.stateMachine) {
    for (const state of [tr.from, tr.to]) {
      if (state && !seen.has(state)) {
        seen.add(state);
        states.push(state);
      }
    }
  }
  return states;
}

/**
 * Render a readable Lua skeleton of the behavior block.
 *
 * This is a PREVIEW / EXPORT artifact for future backend integration — the
 * client never runs it and the server is not wired to consume it yet (FASE
 * 5.3 cut). Output is deterministic for a given behavior + options.
 *
 * @param {object} behavior sparse or normalized behavior
 * @param {{npcId?: string, npcName?: string}} [opts] header annotations
 * @returns {string} Lua source
 */
export function toLuaBehavior(behavior = {}, opts = {}) {
  const b = normalizeBehavior(behavior);
  const npcId = strOr(opts.npcId).trim() || 'npc';
  const npcName = strOr(opts.npcName).trim();

  const lines = [];
  lines.push('-- ============================================================ ');
  lines.push('-- NPC behavior script — PREVIEW / EXPORT ONLY (NOT EXECUTED).');
  lines.push('-- Generated by mud-client NpcFactory (Phase 5.3 behavior lite).');
  lines.push('-- The backend does NOT run this file yet; it is a readable');
  lines.push('-- skeleton for the future Lua behavior-tree integration.');
  lines.push('-- ============================================================ ');
  if (npcName) lines.push(`-- NPC: ${npcId} (${npcName})`);
  else lines.push(`-- NPC: ${npcId}`);
  lines.push('');
  lines.push('local behavior = {}');
  lines.push('');

  const states = behaviorStates(b);
  lines.push('behavior.states = {');
  for (const state of states) lines.push(`  ${luaString(state)},`);
  lines.push('}');
  lines.push('');

  lines.push('-- { from, event, to } transitions (first match wins).');
  lines.push('behavior.transitions = {');
  for (const tr of b.stateMachine) {
    lines.push(
      `  { from = ${luaString(tr.from)}, event = ${luaString(tr.event)}, to = ${luaString(tr.to)} },`
    );
  }
  lines.push('}');
  lines.push('');

  lines.push('behavior.aggro = {');
  lines.push(`  radius = ${b.aggro.radius},`);
  lines.push('  conditions = {');
  for (const cond of b.aggro.conditions) lines.push(`    ${luaString(cond)},`);
  lines.push('  },');
  lines.push('}');
  lines.push('');

  lines.push('behavior.abilities = {');
  for (const ability of b.abilities) {
    lines.push(
      `  { id = ${luaString(ability.id)}, cooldown = ${ability.cooldown}, range = ${ability.range} },`
    );
  }
  lines.push('}');
  lines.push('');

  lines.push('-- Event dispatcher skeleton (future backend hook).');
  lines.push('function behavior.on_event(from_state, event)');
  lines.push('  for _, t in ipairs(behavior.transitions) do');
  lines.push('    if t.from == from_state and t.event == event then');
  lines.push('      return t.to');
  lines.push('    end');
  lines.push('  end');
  lines.push('  return from_state -- unknown event: stay put');
  lines.push('end');
  lines.push('');
  lines.push('return behavior');

  return `${lines.join('\n')}\n`;
}
