// Subtle NPC↔player relationship feedback (Batch 27.4).
//
// The server may attach relationship information to a dialog or default
// message. The exact payload is intentionally tolerated in a few shapes:
//
//   { relation: { delta: 0.03, sentiment: 0.52, target: "player" } }
//   { relation: { text: "Aldric regards you a little more warmly." } }
//   { relationDelta: -0.02 }
//
// Nothing is rendered unless the message actually carries one of these fields.

const DELTA_KEYS = ['delta', 'change', 'relationDelta', 'sentimentDelta', 'adjustment', 'amount'];
const VALUE_KEYS = ['sentiment', 'value', 'score', 'standing', 'affinity'];
const TEXT_KEYS = ['feedback', 'text', 'message', 'label', 'description', 'summary'];
const TARGET_KEYS = ['target', 'targetName', 'npcName', 'npc', 'name'];
const DIRECTION_KEYS = ['direction', 'trend'];

const UP_WORDS = ['up', 'increase', 'increased', 'improve', 'improved', 'positive', 'warmer', 'warm', 'rise', 'rising', 'gain', 'grew'];
const DOWN_WORDS = ['down', 'decrease', 'decreased', 'worsen', 'worsened', 'negative', 'colder', 'cold', 'fall', 'falling', 'loss', 'dropped'];

function finiteNumber(value) {
  if (typeof value === 'number') return Number.isFinite(value) ? value : null;
  if (typeof value === 'string' && value.trim() !== '') {
    const n = Number(value);
    return Number.isFinite(n) ? n : null;
  }
  return null;
}

function firstFinite(obj, keys) {
  for (const key of keys) {
    if (Object.prototype.hasOwnProperty.call(obj, key)) {
      const n = finiteNumber(obj[key]);
      if (n !== null) return n;
    }
  }
  return null;
}

function firstString(obj, keys) {
  for (const key of keys) {
    if (!Object.prototype.hasOwnProperty.call(obj, key)) continue;
    const value = obj[key];
    if (typeof value === 'string' && value.trim() !== '') return value.trim();
  }
  return '';
}

function normalizeDirection(value, delta) {
  const raw = String(value == null ? '' : value).trim().toLowerCase();
  if (UP_WORDS.some((word) => raw === word || raw.startsWith(`${word} `))) return 'up';
  if (DOWN_WORDS.some((word) => raw === word || raw.startsWith(`${word} `))) return 'down';
  if (delta !== null && delta !== 0) return delta > 0 ? 'up' : 'down';
  return 'neutral';
}

/**
 * Normalize relationship info from a message payload.
 * Returns null when the message carries no relationship change.
 */
export function extractRelationFeedback(source, fallbackName = '') {
  if (!source || typeof source !== 'object') return null;

  let raw = source.relation ?? source.relationship ?? source.npcRelation ?? source.relationInfo;
  if (raw == null) {
    const topDelta = firstFinite(source, ['relationDelta', 'sentimentDelta', 'relationshipDelta']);
    const flat = {};
    if (topDelta !== null) flat.delta = topDelta;
    else if (source.relationChange != null) {
      const change = source.relationChange;
      const n = finiteNumber(change);
      if (n === null) flat.text = String(change);
      else flat.delta = n;
    }
    if (source.relationLabel != null) flat.text = String(source.relationLabel);
    if (source.relationFeedback != null) flat.text = String(source.relationFeedback);
    if (source.relationTarget != null) flat.target = String(source.relationTarget);
    if (source.relationDirection != null) flat.direction = String(source.relationDirection);
    if (source.relationSentiment != null) flat.sentiment = source.relationSentiment;
    if (Object.keys(flat).length > 0) raw = flat;
  }
  if (raw == null) return null;

  let obj;
  if (typeof raw === 'number' || typeof raw === 'string') {
    const n = finiteNumber(raw);
    obj = n === null ? { text: String(raw) } : { delta: n };
  } else if (typeof raw === 'object') {
    obj = raw;
  } else {
    return null;
  }

  let delta = firstFinite(obj, DELTA_KEYS);
  const value = firstFinite(obj, VALUE_KEYS);
  if (delta === null) {
    const before = firstFinite(obj, ['before', 'previous', 'old', 'from']);
    const after = firstFinite(obj, ['after', 'current', 'new', 'to']);
    if (before !== null && after !== null) delta = after - before;
  }
  const text = firstString(obj, TEXT_KEYS);
  const explicitDirection = firstString(obj, DIRECTION_KEYS)
    || (typeof obj.change === 'string' ? obj.change.trim() : '');
  const target = firstString(obj, TARGET_KEYS) || String(fallbackName || '').trim();
  const direction = normalizeDirection(explicitDirection, delta);

  // A stored sentiment alone is current state, not a change: do not render it.
  if (delta === null && !text && !explicitDirection) return null;
  if (delta === 0 && !text && !explicitDirection) return null;

  return { target, delta, value, text, direction };
}

/** Short, human label used by the dialog overlay. */
export function relationSummary(relation, fallbackName = '') {
  if (!relation) return '';
  if (relation.text) return relation.text;

  const target = String(relation.target || '').trim();
  const name = String(fallbackName || '').trim()
    || (target && !/^(player|you|character)$/i.test(target) ? target : '')
    || 'The NPC';

  if (relation.direction === 'up') return `${name}'s regard for you warms.`;
  if (relation.direction === 'down') return `${name}'s regard for you cools.`;
  return `${name}'s regard for you shifts.`;
}

/** Terminal/chat line. Bracketed to match the existing prose style. */
export function formatRelationFeedback(relation, fallbackName = '') {
  const summary = relationSummary(relation, fallbackName);
  if (!summary) return '';
  return /^\[.*\]$/.test(summary) ? summary : `[${summary}]`;
}

/**
 * Append feedback to a plain message, unless the server already included the
 * explicit feedback text. Returns the original text when there is no relation.
 */
export function appendRelationFeedback(text, source, fallbackName = '') {
  const base = String(text == null ? '' : text);
  const relation = extractRelationFeedback(source, fallbackName);
  const feedback = formatRelationFeedback(relation, fallbackName);
  if (!feedback) return base;
  if (relation.text && base.includes(relation.text.trim())) return base;
  const trimmed = base.trim();
  return trimmed ? `${trimmed}\n${feedback}` : feedback;
}
