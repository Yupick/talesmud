/**
 * Mob / NPC factory model (Creator Phase 5.3, initial cut).
 *
 * Pure module: template defaults, normalization, validation, serialization and
 * a best-effort mapping to the backend NPC payload. The component
 * (`ui/NpcFactory.svelte`) owns the reactive state; everything here runs without
 * a DOM so `node --test` can cover it.
 *
 * Persistence mirrors `classEditor.js`: templates live in localStorage while
 * the in-game factory is the authoring surface. The backend serves the Batch 21
 * template CRUD routes (`/api/npc-templates`, see `src/api/npcs.js`) so the
 * component offers a best-effort push/pull on top of the local copy via the
 * `toTemplatePayload`/`fromTemplatePayload` mapping below. The older
 * `toNpcPayload`/`fromNpcPayload` pair (legacy `/api/npcs` Go payloads) is
 * kept for spawn tooling and is not touched by the template contract.
 */

import { normalizeBehavior, normalizeScaling } from './npcBehavior.js';

export const NPCS_STORAGE_KEY = 'mud-client.npc-factory.v1';

export const LLM_RELATION_KINDS = ['ally', 'rival', 'family', 'faction', 'mentor'];

export function emptyLLMPersonality() {
  return {
    openness: 0,
    conscientiousness: 0,
    extraversion: 0,
    agreeableness: 0,
    neuroticism: 0,
  };
}

export function emptyLLMConfig() {
  return {
    enabled: false,
    persona: '',
    voice: '',
    temperature: 0.8,
    memoryTurns: 8,
    fallbackDialogId: '',
    cooldownSeconds: 3,
    personality: emptyLLMPersonality(),
    goals: [],
    secrets: [],
    relations: [],
  };
}

export function emptyLLMRelation() {
  return {
    target: '',
    kind: 'ally',
    sentiment: 0,
  };
}

/** Friendly roles the factory exposes. `enemy`/`boss` become hostile mobs. */
export const NPC_ROLES = [
  { id: 'merchant', label: 'Merchant' },
  { id: 'guard', label: 'Guard' },
  { id: 'quest_giver', label: 'Quest Giver' },
  { id: 'trainer', label: 'Trainer' },
  { id: 'innkeeper', label: 'Innkeeper' },
  { id: 'critter', label: 'Critter' },
  { id: 'enemy', label: 'Enemy (mob)' },
  { id: 'boss', label: 'Boss' },
];
export const NPC_ROLE_IDS = NPC_ROLES.map((role) => role.id);

/** Mirrors the backend `npc.EnemyTrait` enums. */
export const CREATURE_TYPES = [
  'beast',
  'humanoid',
  'undead',
  'elemental',
  'construct',
  'demon',
  'dragon',
  'aberration',
];
export const COMBAT_STYLES = ['melee', 'ranged', 'magic', 'swarm', 'brute', 'agile'];
export const DIFFICULTIES = ['trivial', 'easy', 'normal', 'hard', 'boss'];

/** Inclusive numeric ranges the validator enforces. */
export const RANGES = {
  level: { min: 1, max: 100 },
  maxHp: { min: 1, max: 1000000 },
  damage: { min: 0, max: 100000 },
  aggroRadius: { min: 0, max: 10 },
  xpReward: { min: 0, max: 1000000000 },
  gold: { min: 0, max: 1000000000 },
  spawnCount: { min: 0, max: 1000 },
  respawnSeconds: { min: 0, max: 604800 },
  lootChance: { min: 0, max: 100 },
  lootQuantity: { min: 1, max: 1000 },
};

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

function numOr(value, fallback) {
  const n = typeof value === 'number' ? value : Number.parseFloat(value);
  return Number.isFinite(n) ? n : fallback;
}

function intOr(value, fallback) {
  const n = numOr(value, NaN);
  return Number.isFinite(n) ? Math.trunc(n) : fallback;
}

function boolOr(value, fallback) {
  if (typeof value === 'boolean') return value;
  if (value === 0 || value === 1) return value === 1;
  if (typeof value === 'string') {
    const v = value.trim().toLowerCase();
    if (v === 'true' || v === '1' || v === 'yes') return true;
    if (v === 'false' || v === '0' || v === 'no' || v === '') return false;
  }
  return fallback;
}

/** Blank loot row (a simple drop chance). */
export function emptyLootEntry() {
  return { itemId: '', chance: 100, quantity: 1 };
}

/** Start-of-template defaults. `id` is a safe, editable seed. */
export function emptyTemplate() {
  return {
    id: 'new_npc',
    name: 'New NPC',
    description: '',
    role: 'enemy',
    creatureType: 'humanoid',
    combatStyle: 'melee',
    difficulty: 'normal',
    level: 1,
    maxHp: 10,
    damageMin: 1,
    damageMax: 3,
    aggressive: true,
    aggroRadius: 0,
    xpReward: 0,
    goldMin: 0,
    goldMax: 0,
    lootTableId: '',
    loot: [],
    spawnRoomId: '',
    spawnCount: 1,
    respawnSeconds: 0,
    isTemplate: true,
    behavior: { stateMachine: [], aggro: { radius: 0, conditions: [] }, abilities: [] },
    scaling: { targetLevel: null },
    llm: null,
  };
}

/** Coerce a possibly-sparse loot row into the editor shape. */
export function normalizeLootEntry(raw = {}) {
  const base = emptyLootEntry();
  return {
    itemId: String(raw.itemId ?? raw.item ?? '').trim(),
    chance: intOr(raw.chance, base.chance),
    quantity: intOr(raw.quantity ?? raw.qty, base.quantity),
  };
}

// Keep a non-empty string as-is so `validateTemplate` can flag unknown values;
// fall back to the default only when the value is missing or blank.
function enumOr(value, fallback) {
  const v = String(value ?? '').trim();
  return v || fallback;
}

function normalizeLLMPersonality(raw = {}) {
  const base = emptyLLMPersonality();
  const clamp = (v) => {
    const n = numOr(v, NaN);
    if (!Number.isFinite(n)) return 0;
    if (n < 0) return 0;
    if (n > 1) return 1;
    return n;
  };
  return {
    openness: clamp(raw.openness),
    conscientiousness: clamp(raw.conscientiousness),
    extraversion: clamp(raw.extraversion),
    agreeableness: clamp(raw.agreeableness),
    neuroticism: clamp(raw.neuroticism),
  };
}

function normalizeLLMRelation(raw = {}) {
  const base = emptyLLMRelation();
  const kind = String(raw.kind ?? base.kind).trim();
  const allowed = LLM_RELATION_KINDS.includes(kind) ? kind : base.kind;
  let sentiment = numOr(raw.sentiment, base.sentiment);
  if (!Number.isFinite(sentiment)) sentiment = base.sentiment;
  if (sentiment < -1) sentiment = -1;
  if (sentiment > 1) sentiment = 1;
  return {
    target: String(raw.target ?? '').trim(),
    kind: allowed,
    sentiment,
  };
}

function normalizeLLM(raw) {
  if (!raw || typeof raw !== 'object') return null;
  const base = emptyLLMConfig();
  const temperature = numOr(raw.temperature, base.temperature);
  const clampedTemp = Number.isFinite(temperature)
    ? Math.min(1, Math.max(0, temperature))
    : base.temperature;
  const memoryTurns = intOr(raw.memoryTurns, base.memoryTurns);
  const cooldownSeconds = intOr(raw.cooldownSeconds, base.cooldownSeconds);
  return {
    enabled: boolOr(raw.enabled, base.enabled),
    persona: String(raw.persona ?? ''),
    voice: String(raw.voice ?? ''),
    temperature: clampedTemp,
    memoryTurns: memoryTurns < 0 ? 0 : memoryTurns,
    fallbackDialogId: String(raw.fallbackDialogId ?? raw.fallbackDialogID ?? ''),
    cooldownSeconds: cooldownSeconds < 0 ? 0 : cooldownSeconds,
    personality: normalizeLLMPersonality(raw.personality || {}),
    goals: Array.isArray(raw.goals) ? raw.goals.map((g) => String(g || '')).filter((g) => g.trim() !== '') : [],
    secrets: Array.isArray(raw.secrets) ? raw.secrets.map((s) => String(s || '')).filter((s) => s.trim() !== '') : [],
    relations: Array.isArray(raw.relations) ? raw.relations.map(normalizeLLMRelation).filter((r) => r.target.trim() !== '') : [],
  };
}

/** Coerce a possibly-sparse template into the full editor shape. */
export function normalizeTemplate(raw = {}) {
  const base = emptyTemplate();
  const loot = Array.isArray(raw.loot)
    ? raw.loot.map(normalizeLootEntry)
    : Array.isArray(raw.drops)
      ? raw.drops.map(normalizeLootEntry)
      : [];
  return {
    ...base,
    ...raw,
    id: String(raw.id ?? base.id).trim(),
    name: String(raw.name ?? base.name),
    description: String(raw.description ?? base.description),
    role: enumOr(raw.role, base.role),
    creatureType: enumOr(raw.creatureType, base.creatureType),
    combatStyle: enumOr(raw.combatStyle, base.combatStyle),
    difficulty: enumOr(raw.difficulty, base.difficulty),
    level: intOr(raw.level, base.level),
    maxHp: intOr(raw.maxHp ?? raw.maxHitPoints, base.maxHp),
    damageMin: intOr(raw.damageMin, base.damageMin),
    damageMax: intOr(raw.damageMax ?? raw.attackPower, base.damageMax),
    aggressive: boolOr(raw.aggressive, base.aggressive),
    aggroRadius: intOr(raw.aggroRadius, base.aggroRadius),
    xpReward: intOr(raw.xpReward, base.xpReward),
    goldMin: intOr(raw.goldMin ?? (raw.goldDrop && raw.goldDrop.min), base.goldMin),
    goldMax: intOr(raw.goldMax ?? (raw.goldDrop && raw.goldDrop.max), base.goldMax),
    lootTableId: String(raw.lootTableId ?? base.lootTableId),
    loot,
    spawnRoomId: String(raw.spawnRoomId ?? base.spawnRoomId),
    spawnCount: intOr(raw.spawnCount, base.spawnCount),
    respawnSeconds: intOr(raw.respawnSeconds, base.respawnSeconds),
    isTemplate: boolOr(raw.isTemplate, base.isTemplate),
    // Phase 5.3: behavior/scaling are optional and retrocompatible — templates
    // saved before they existed load with the default idle⇄combat behavior.
    behavior: normalizeBehavior(raw.behavior),
    scaling: normalizeScaling(raw.scaling),
    llm: normalizeLLM(raw.llm),
  };
}

function rangeIssue(field, value, range, label) {
  const n = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(n)) {
    return { field, level: 'error', msg: `${label} must be a number.` };
  }
  if (!Number.isInteger(n)) {
    return { field, level: 'error', msg: `${label} must be a whole number.` };
  }
  if (n < range.min || n > range.max) {
    return {
      field,
      level: 'error',
      msg: `${label} must be between ${range.min} and ${range.max}.`,
    };
  }
  return null;
}

function pushIssue(issues, issue) {
  if (issue) issues.push(issue);
}

/**
 * Validate a template and return a list of `{ field, level, msg }` issues.
 *
 * @param {object} raw template (sparse accepted)
 * @param {{otherIds?: string[]}} [opts] ids to check the template id against
 *   (pass the sibling ids, excluding this template's own id).
 * @returns {Array<{field:string, level:'error'|'warn', msg:string}>}
 */
export function validateTemplate(raw, opts = {}) {
  const tpl = normalizeTemplate(raw);
  const otherIds = new Set((opts.otherIds || []).map((id) => String(id)));
  const issues = [];

  // Identity
  if (!tpl.id) {
    issues.push({ field: 'id', level: 'error', msg: 'Id is required.' });
  } else if (!/^[a-z0-9_]+$/.test(tpl.id)) {
    issues.push({
      field: 'id',
      level: 'error',
      msg: 'Id must use lowercase letters, digits and underscores only.',
    });
  } else if (otherIds.has(tpl.id)) {
    issues.push({ field: 'id', level: 'error', msg: 'Id is already used by another template.' });
  }
  if (!tpl.name.trim()) {
    issues.push({ field: 'name', level: 'error', msg: 'Name is required.' });
  } else if (tpl.name.length > 60) {
    issues.push({ field: 'name', level: 'warn', msg: 'Name is longer than 60 characters.' });
  }
  if (!NPC_ROLE_IDS.includes(tpl.role)) {
    issues.push({ field: 'role', level: 'error', msg: `Unknown role "${tpl.role}".` });
  }
  if (!CREATURE_TYPES.includes(tpl.creatureType)) {
    issues.push({ field: 'creatureType', level: 'error', msg: `Unknown creature type "${tpl.creatureType}".` });
  }
  if (!COMBAT_STYLES.includes(tpl.combatStyle)) {
    issues.push({ field: 'combatStyle', level: 'error', msg: `Unknown combat style "${tpl.combatStyle}".` });
  }
  if (!DIFFICULTIES.includes(tpl.difficulty)) {
    issues.push({ field: 'difficulty', level: 'error', msg: `Unknown difficulty "${tpl.difficulty}".` });
  }

  // Numbers
  pushIssue(issues, rangeIssue('level', tpl.level, RANGES.level, 'Level'));
  pushIssue(issues, rangeIssue('maxHp', tpl.maxHp, RANGES.maxHp, 'Max HP'));
  pushIssue(issues, rangeIssue('damageMin', tpl.damageMin, RANGES.damage, 'Min damage'));
  pushIssue(issues, rangeIssue('damageMax', tpl.damageMax, RANGES.damage, 'Max damage'));
  pushIssue(issues, rangeIssue('aggroRadius', tpl.aggroRadius, RANGES.aggroRadius, 'Aggro radius'));
  pushIssue(issues, rangeIssue('xpReward', tpl.xpReward, RANGES.xpReward, 'XP reward'));
  pushIssue(issues, rangeIssue('goldMin', tpl.goldMin, RANGES.gold, 'Min gold'));
  pushIssue(issues, rangeIssue('goldMax', tpl.goldMax, RANGES.gold, 'Max gold'));
  pushIssue(issues, rangeIssue('spawnCount', tpl.spawnCount, RANGES.spawnCount, 'Spawn count'));
  pushIssue(issues, rangeIssue('respawnSeconds', tpl.respawnSeconds, RANGES.respawnSeconds, 'Respawn seconds'));

  if (tpl.damageMax < tpl.damageMin) {
    issues.push({ field: 'damageMax', level: 'error', msg: 'Max damage cannot be below min damage.' });
  }
  if (tpl.goldMax < tpl.goldMin) {
    issues.push({ field: 'goldMax', level: 'error', msg: 'Max gold cannot be below min gold.' });
  }

  // Cross-field warnings
  const hostile = tpl.role === 'enemy' || tpl.role === 'boss';
  if (hostile && !tpl.aggressive) {
    issues.push({ field: 'aggressive', level: 'warn', msg: 'Enemy/boss templates are usually aggressive.' });
  }
  if (tpl.aggressive && tpl.aggroRadius === 0) {
    issues.push({
      field: 'aggroRadius',
      level: 'warn',
      msg: 'Aggressive, but aggro radius 0: players must attack first.',
    });
  }
  if (hostile && tpl.xpReward === 0) {
    issues.push({ field: 'xpReward', level: 'warn', msg: 'Enemy/boss with no XP reward.' });
  }
  if (!tpl.spawnRoomId.trim()) {
    issues.push({ field: 'spawnRoomId', level: 'warn', msg: 'No spawn area set.' });
  }

  // Loot
  if (!Array.isArray(tpl.loot)) {
    issues.push({ field: 'loot', level: 'error', msg: 'Loot must be a list.' });
  } else {
    const seen = new Set();
    tpl.loot.forEach((entry, index) => {
      const prefix = `loot[${index}].`;
      if (!entry.itemId) {
        issues.push({ field: `${prefix}itemId`, level: 'error', msg: 'Loot item id is required.' });
      } else if (seen.has(entry.itemId)) {
        issues.push({ field: `${prefix}itemId`, level: 'warn', msg: `Duplicate loot item "${entry.itemId}".` });
      }
      seen.add(entry.itemId);
      pushIssue(issues, rangeIssue(`${prefix}chance`, entry.chance, RANGES.lootChance, 'Drop chance'));
      pushIssue(issues, rangeIssue(`${prefix}quantity`, entry.quantity, RANGES.lootQuantity, 'Drop quantity'));
    });
  }

  return issues;
}

/** Split an issue list into error/warning counts for the panel badges. */
export function issueCounts(issues = []) {
  let errors = 0;
  let warnings = 0;
  for (const issue of issues) {
    if (issue.level === 'error') errors += 1;
    else if (issue.level === 'warn') warnings += 1;
  }
  return { errors, warnings };
}

/** Serialize a template to a stable JSON string. */
export function serializeTemplate(raw) {
  return JSON.stringify(normalizeTemplate(raw));
}

/**
 * Parse a serialized template. Returns a normalized template on success and
 * `null` when the payload is missing or corrupt.
 */
export function deserializeTemplate(text) {
  if (typeof text !== 'string' || !text.trim()) return null;
  try {
    const parsed = JSON.parse(text);
    if (!parsed || typeof parsed !== 'object') return null;
    return normalizeTemplate(parsed);
  } catch (err) {
    return null;
  }
}

/** First free `<base>`, `<base>_2`, `<base>_3` … against a list of ids. */
export function uniqueId(base, existingIds = []) {
  const used = new Set((existingIds || []).map((id) => String(id)));
  const clean = slug(base, 'npc');
  if (!used.has(clean)) return clean;
  let n = 2;
  while (used.has(`${clean}_${n}`)) n += 1;
  return `${clean}_${n}`;
}

/** Deep-copy a template with a unique id and a "(copy)" name suffix. */
export function duplicateTemplate(source, existingIds = []) {
  const copy = clone(source || emptyTemplate());
  copy.id = uniqueId(`${copy.id}_copy`, existingIds);
  copy.name = `${copy.name || 'NPC'} (copy)`;
  return normalizeTemplate(copy);
}

/**
 * Best-effort mapping to the backend NPC JSON payload
 * (`pkg/entities/npcs/npc.go`). `respawnTime` is a Go `time.Duration`
 * (nanoseconds). Loot rows at 100% become `guaranteedLoot`.
 */
export function toNpcPayload(raw) {
  const tpl = normalizeTemplate(raw);
  const hostile = tpl.role === 'enemy' || tpl.role === 'boss';
  const payload = {
    id: tpl.id,
    name: tpl.name,
    description: tpl.description,
    level: tpl.level,
    maxHitPoints: tpl.maxHp,
    currentHitPoints: tpl.maxHp,
    isTemplate: tpl.isTemplate !== false,
    spawnRoomId: tpl.spawnRoomId,
    respawnTime: tpl.respawnSeconds * 1e9,
  };
  if (hostile || tpl.aggressive) {
    payload.enemyTrait = {
      creatureType: tpl.creatureType,
      combatStyle: tpl.combatStyle,
      difficulty: tpl.difficulty,
      attackPower: tpl.damageMax,
      defense: 0,
      attackSpeed: 0,
      aggroRadius: tpl.aggroRadius,
      aggroOnSight: tpl.aggressive,
      callForHelp: false,
      fleeThreshold: 0,
      xpReward: tpl.xpReward,
      goldDrop: { min: tpl.goldMin, max: tpl.goldMax },
      lootTableId: tpl.lootTableId,
      guaranteedLoot: tpl.loot.filter((entry) => entry.chance >= 100 && entry.itemId).map((entry) => entry.itemId),
      maxDrops: 0,
    };
  }
  return payload;
}

/** Reverse of `toNpcPayload`. Best-effort; unknown roles fall back to `critter`. */
export function fromNpcPayload(npc = {}) {
  const base = emptyTemplate();
  const enemy = npc.enemyTrait || {};
  const template = {
    ...base,
    id: String(npc.id || base.id).trim(),
    name: String(npc.name ?? base.name),
    description: String(npc.description ?? base.description),
    role: npc.enemyTrait ? 'enemy' : 'critter',
    creatureType: CREATURE_TYPES.includes(enemy.creatureType) ? enemy.creatureType : base.creatureType,
    combatStyle: COMBAT_STYLES.includes(enemy.combatStyle) ? enemy.combatStyle : base.combatStyle,
    difficulty: DIFFICULTIES.includes(enemy.difficulty) ? enemy.difficulty : base.difficulty,
    level: intOr(npc.level, base.level),
    maxHp: intOr(npc.maxHitPoints ?? npc.maxHp, base.maxHp),
    damageMax: intOr(enemy.attackPower, base.damageMax),
    aggressive: boolOr(enemy.aggroOnSight, false),
    aggroRadius: intOr(enemy.aggroRadius, 0),
    xpReward: intOr(enemy.xpReward, 0),
    goldMin: intOr(enemy.goldDrop && enemy.goldDrop.min, 0),
    goldMax: intOr(enemy.goldDrop && enemy.goldDrop.max, 0),
    lootTableId: String(enemy.lootTableId ?? ''),
    spawnRoomId: String(npc.spawnRoomId ?? ''),
    respawnSeconds: Math.trunc(numOr(npc.respawnTime, 0) / 1e9),
    isTemplate: boolOr(npc.isTemplate, true),
  };
  return normalizeTemplate(template);
}

/**
 * Batch 21 template contract mapping (editor shape → `/api/npc-templates`
 * DTO). Unlike `toNpcPayload` (which targets the legacy Go `npc.NPC` JSON with
 * `enemyTrait`/nanosecond durations), the template DTO keeps the flat editor
 * field names, so this is a faithful, explicit pick — no trait wrapping, no
 * unit conversions. `behavior`/`scaling`/`loot` are normalized so the backend
 * always receives the full shape. `updatedAt` is server-owned and omitted.
 */
export function toTemplatePayload(raw) {
  const tpl = normalizeTemplate(raw);
  const payload = {
    id: tpl.id,
    name: tpl.name,
    description: tpl.description,
    role: tpl.role,
    creatureType: tpl.creatureType,
    combatStyle: tpl.combatStyle,
    difficulty: tpl.difficulty,
    level: tpl.level,
    maxHp: tpl.maxHp,
    damageMin: tpl.damageMin,
    damageMax: tpl.damageMax,
    aggressive: tpl.aggressive,
    aggroRadius: tpl.aggroRadius,
    xpReward: tpl.xpReward,
    goldMin: tpl.goldMin,
    goldMax: tpl.goldMax,
    lootTableId: tpl.lootTableId,
    loot: clone(tpl.loot),
    spawnRoomId: tpl.spawnRoomId,
    spawnCount: tpl.spawnCount,
    respawnSeconds: tpl.respawnSeconds,
    isTemplate: tpl.isTemplate !== false,
    behavior: clone(tpl.behavior),
    scaling: clone(tpl.scaling),
  };
  if (tpl.llm) {
    payload.llm = clone(tpl.llm);
  }
  return payload;
}

/**
 * Reverse of `toTemplatePayload`: `/api/npc-templates` DTO (summary row or
 * full DTO) → normalized editor template. Faithful because the DTO uses the
 * same field names; sparse summary rows pick up the editor defaults via
 * `normalizeTemplate`, which also normalizes `behavior`/`scaling`/`loot`.
 */
export function fromTemplatePayload(dto = {}) {
  return normalizeTemplate(dto || {});
}

// ---------------------------------------------------------------------------
// Demo fixture (used until the user authors their own catalog)
// ---------------------------------------------------------------------------

const FIXTURE = [
  {
    id: 'goblin_scout',
    name: 'Goblin Scout',
    description: 'A wiry skirmisher that screens the warren.',
    role: 'enemy',
    creatureType: 'humanoid',
    combatStyle: 'melee',
    difficulty: 'easy',
    level: 3,
    maxHp: 24,
    damageMin: 2,
    damageMax: 5,
    aggressive: true,
    aggroRadius: 1,
    xpReward: 12,
    goldMin: 0,
    goldMax: 4,
    lootTableId: '',
    loot: [{ itemId: 'goblin_ear', chance: 60, quantity: 1 }],
    spawnRoomId: 'forest_edge',
    spawnCount: 1,
    respawnSeconds: 120,
    isTemplate: true,
  },
  {
    id: 'town_guard',
    name: 'Town Guard',
    description: 'Stands the gate and takes no nonsense.',
    role: 'guard',
    creatureType: 'humanoid',
    combatStyle: 'melee',
    difficulty: 'normal',
    level: 10,
    maxHp: 120,
    damageMin: 8,
    damageMax: 14,
    aggressive: false,
    aggroRadius: 0,
    xpReward: 0,
    goldMin: 0,
    goldMax: 0,
    lootTableId: '',
    loot: [],
    spawnRoomId: 'town_gate',
    spawnCount: 2,
    respawnSeconds: 300,
    isTemplate: false,
  },
  {
    id: 'forest_wolf',
    name: 'Forest Wolf',
    description: 'Hunts in a small pack at the treeline.',
    role: 'enemy',
    creatureType: 'beast',
    combatStyle: 'agile',
    difficulty: 'normal',
    level: 5,
    maxHp: 40,
    damageMin: 3,
    damageMax: 7,
    aggressive: true,
    aggroRadius: 2,
    xpReward: 18,
    goldMin: 0,
    goldMax: 2,
    lootTableId: '',
    loot: [{ itemId: 'wolf_pelt', chance: 80, quantity: 1 }],
    spawnRoomId: 'deep_woods',
    spawnCount: 3,
    respawnSeconds: 180,
    isTemplate: true,
  },
];

/** Deep copy of the demo catalog (safe to mutate). */
export function sampleTemplates() {
  return clone(FIXTURE);
}

// ---------------------------------------------------------------------------
// Persistence (localStorage; the in-game factory is the authoring surface)
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
export function loadTemplates(storage) {
  const store = resolveStorage(storage);
  if (!store) return sampleTemplates();
  try {
    const raw = store.getItem(NPCS_STORAGE_KEY);
    if (!raw) return sampleTemplates();
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed) || parsed.length === 0) return sampleTemplates();
    return parsed.map(normalizeTemplate);
  } catch (err) {
    return sampleTemplates();
  }
}

/**
 * Persist the catalog. Returns true on success.
 * @param {Array<object>} templates
 * @param {Storage} [storage]
 */
export function saveTemplates(templates, storage) {
  const store = resolveStorage(storage);
  if (!store) return false;
  try {
    store.setItem(NPCS_STORAGE_KEY, JSON.stringify(templates || []));
    return true;
  } catch (err) {
    return false;
  }
}

/** Drop the saved catalog so the fixture returns. */
export function resetTemplates(storage) {
  const store = resolveStorage(storage);
  if (!store) return false;
  try {
    store.removeItem(NPCS_STORAGE_KEY);
    return true;
  } catch (err) {
    return false;
  }
}
