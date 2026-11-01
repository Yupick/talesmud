import assert from 'assert';
import {
  appendRelationFeedback,
  extractRelationFeedback,
  formatRelationFeedback,
  relationSummary,
} from './relationFeedback.js';

// No relation info → nothing changes.
assert.strictEqual(extractRelationFeedback({ message: 'Hello' }), null);
assert.strictEqual(appendRelationFeedback('Hello', { message: 'Hello' }), 'Hello');
assert.strictEqual(formatRelationFeedback(null), '');

// Top-level delta (the shape the dialogue result already uses).
const topLevel = extractRelationFeedback({ relationDelta: 0.03 }, 'Aldric');
assert.strictEqual(topLevel.direction, 'up');
assert.strictEqual(topLevel.delta, 0.03);
assert.strictEqual(formatRelationFeedback(topLevel, 'Aldric'), "[Aldric's regard for you warms.]");
assert.strictEqual(
  appendRelationFeedback('[Aldric] Well met.', { relationDelta: 0.03 }, 'Aldric'),
  "[Aldric] Well met.\n[Aldric's regard for you warms.]"
);

// Nested relation object, negative delta, target "player" is not shown as the NPC.
const nested = extractRelationFeedback(
  { relation: { target: 'player', delta: -0.04, sentiment: 0.41 } },
  'Aldric'
);
assert.strictEqual(nested.direction, 'down');
assert.strictEqual(nested.value, 0.41);
assert.strictEqual(relationSummary(nested, 'Aldric'), "Aldric's regard for you cools.");

// Explicit server prose wins.
const explicit = extractRelationFeedback(
  { relation: { text: 'She studies you with new respect.', direction: 'up' } },
  'Mira'
);
assert.strictEqual(relationSummary(explicit, 'Mira'), 'She studies you with new respect.');
assert.strictEqual(formatRelationFeedback(explicit, 'Mira'), '[She studies you with new respect.]');

// Do not duplicate explicit prose already present in the message.
assert.strictEqual(
  appendRelationFeedback(
    'Mira nods. She studies you with new respect.',
    { relation: { text: 'She studies you with new respect.' } },
    'Mira'
  ),
  'Mira nods. She studies you with new respect.'
);

// Current sentiment without a delta is not a change.
assert.strictEqual(extractRelationFeedback({ relation: { sentiment: 0.8 } }), null);
assert.strictEqual(extractRelationFeedback({ relationDelta: 0 }), null);

// String deltas and string relation payloads are tolerated.
assert.strictEqual(extractRelationFeedback({ relationDelta: '-0.02' }).direction, 'down');
assert.strictEqual(extractRelationFeedback({ relation: 'The merchant softens.' }).text, 'The merchant softens.');

// Flat fields are tolerated too.
assert.strictEqual(extractRelationFeedback({ relationLabel: 'A new bond forms.' }).text, 'A new bond forms.');
assert.strictEqual(extractRelationFeedback({ relationDirection: 'down' }).direction, 'down');

// Before/after and textual change still produce a direction.
assert.strictEqual(extractRelationFeedback({ relation: { before: 0.2, after: 0.24 } }).direction, 'up');
assert.strictEqual(extractRelationFeedback({ relation: { change: 'improved' } }).direction, 'up');

// Explicit direction without a numeric delta still renders.
assert.strictEqual(extractRelationFeedback({ relation: { direction: 'up' } }).direction, 'up');

// Already-bracketed server prose is not double-wrapped.
assert.strictEqual(
  formatRelationFeedback({ text: '[Mira warms to you.]', direction: 'up' }, 'Mira'),
  '[Mira warms to you.]'
);

console.log('relationFeedback_test.mjs ok');
