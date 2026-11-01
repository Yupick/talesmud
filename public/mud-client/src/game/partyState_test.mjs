import assert from 'assert';
import {
  LOOT_RULES,
  classifyReadyCheck,
  isPartyLeader,
  lootRuleLabel,
  normalizeLootRule,
  normalizePartyInvite,
  normalizePartyState,
  parsePartyChatLine,
  DEFAULT_MAX_PARTY,
} from './partyState.js';

assert.deepStrictEqual(
  normalizePartyState(null),
  { inParty: false, partyId: '', partyName: '', leaderId: '', maxMembers: DEFAULT_MAX_PARTY, members: [], following: false, lootRule: 'free' },
  'null party → empty'
);

assert.deepStrictEqual(
  normalizePartyState({
    inParty: true,
    partyID: 'p1',
    partyName: "Aster's Party",
    leaderId: 'c1',
    maxMembers: 5,
    members: [{ id: 'c1', name: 'Aster', online: true, level: 3, class: 'Warrior', isLeader: true }],
  }),
  {
    inParty: true,
    partyId: 'p1',
    partyName: "Aster's Party",
    leaderId: 'c1',
    maxMembers: 5,
    members: [{ id: 'c1', name: 'Aster', online: true, level: 3, class: 'Warrior', portrait: '', isLeader: true }],
    following: false,
    lootRule: 'free',
  },
  'partyID alias + rich members preserved'
);

// Loot rule normalization + labels (B8).
assert.strictEqual(normalizeLootRule('roundrobin'), 'roundrobin');
assert.strictEqual(normalizeLootRule('Need/Greed'), 'needgreed');
assert.strictEqual(normalizeLootRule('NEED_GREED'), 'needgreed');
assert.strictEqual(normalizeLootRule('bogus'), 'free');
assert.strictEqual(normalizeLootRule(undefined), 'free');
assert.strictEqual(lootRuleLabel('roundrobin'), 'Round Robin');
assert.strictEqual(lootRuleLabel('needgreed'), 'Need/Greed');
assert.strictEqual(LOOT_RULES.length, 4);
assert.deepStrictEqual(
  normalizePartyState({ inParty: true, lootRule: 'roundrobin', members: [] }).lootRule,
  'roundrobin'
);

// Leadership gating used to show/hide the loot selector + ready check.
const leaderChar = { id: 'c1', name: 'Aster' };
const memberChar = { id: 'c2', name: 'Bryn' };
const leaderParty = {
  leaderId: 'c1',
  members: [
    { id: 'c1', name: 'Aster', isLeader: true },
    { id: 'c2', name: 'Bryn', isLeader: false },
  ],
};
assert.strictEqual(isPartyLeader(leaderParty, leaderChar), true);
assert.strictEqual(isPartyLeader(leaderParty, memberChar), false);
assert.strictEqual(isPartyLeader(null, leaderChar), false);
assert.strictEqual(
  isPartyLeader({ leaderId: '', members: [{ id: 'c2', name: 'Bryn', isLeader: true }] }, memberChar),
  true,
  'matches by member isLeader flag'
);

// Ready-check phase derived from the party chat feed.
assert.strictEqual(classifyReadyCheck([]), 'idle');
assert.strictEqual(
  classifyReadyCheck([{ text: "Ready check! Type 'party ready' or 'party notready'." }]),
  'active'
);
assert.strictEqual(
  classifyReadyCheck([{ text: 'Ready check!' }, { text: 'Ready: 1/3 · Not ready: 2/3' }]),
  'active'
);
assert.strictEqual(
  classifyReadyCheck([{ text: 'Ready check!' }, { text: 'Ready check over: 3/3 ready.' }]),
  'done'
);
assert.strictEqual(
  classifyReadyCheck([{ text: 'Ready check over: 3/3 ready.' }, { text: 'Ready check!' }]),
  'active',
  'newest marker wins'
);

assert.strictEqual(normalizePartyInvite(null), null);
assert.strictEqual(normalizePartyInvite({ pending: false }), null);
assert.deepStrictEqual(
  normalizePartyInvite({ pending: true, inviterName: 'Aster', partyId: 'p1', expiresAt: 1700000045 }),
  { pending: true, inviterName: 'Aster', partyId: 'p1', expiresAt: 1700000045 },
  'pending invite normalized'
);

const say = parsePartyChatLine('[Party] Aster: regroup', 'Aster');
assert.ok(say && say.name === 'Aster' && say.text === 'regroup' && say.isYou);
const sys = parsePartyChatLine('[Party] Bryn joined the party.', 'Aster');
assert.ok(sys && sys.system && sys.text.includes('joined'));

console.log('partyState: normalize helpers OK');
