/**
 * Pure helpers for party overlay store payloads (mirrors Friends structured msgs).
 */

export const DEFAULT_MAX_PARTY = 5;

/** Loot rules offered to the party leader (B8). */
export const DEFAULT_LOOT_RULE = 'free';
export const LOOT_RULES = [
  { id: 'free', label: 'Free-for-all' },
  { id: 'leader', label: 'Leader' },
  { id: 'roundrobin', label: 'Round Robin' },
  { id: 'needgreed', label: 'Need/Greed' },
];

const LOOT_RULE_IDS = LOOT_RULES.map((r) => r.id);

/** Coerce any server value to a known loot rule id (defaults to "free"). */
export function normalizeLootRule(value) {
  const raw = String(value == null ? '' : value).trim().toLowerCase().replace(/[\s_-]/g, '');
  if (raw === 'need/greed' || raw === 'needgreed') return 'needgreed';
  return LOOT_RULE_IDS.includes(raw) ? raw : DEFAULT_LOOT_RULE;
}

/** Human label for a loot rule id ("roundrobin" → "Round Robin"). */
export function lootRuleLabel(value) {
  const id = normalizeLootRule(value);
  const found = LOOT_RULES.find((r) => r.id === id);
  return found ? found.label : id;
}

export function normalizePartyMember(raw) {
  const m = raw && typeof raw === 'object' ? raw : {};
  return {
    id: String(m.id || ''),
    name: String(m.name || ''),
    online: !!m.online,
    level: Number(m.level) || 0,
    class: String(m.class || m.className || ''),
    portrait: String(m.portrait || m.picture || ''),
    isLeader: !!(m.isLeader || m.leader),
  };
}

export function normalizePartyState(raw) {
  const next = raw && typeof raw === 'object' ? raw : {};
  const members = Array.isArray(next.members)
    ? next.members.map(normalizePartyMember)
    : [];
  const maxMembers = Number(next.maxMembers) > 0 ? Number(next.maxMembers) : DEFAULT_MAX_PARTY;
  return {
    inParty: !!next.inParty,
    partyId: String(next.partyId || next.partyID || ''),
    partyName: String(next.partyName || ''),
    leaderId: String(next.leaderId || next.leaderCharacterId || next.LeaderCharacterID || ''),
    maxMembers,
    members,
    following: !!next.following,
    lootRule: normalizeLootRule(next.lootRule || next.LootRule),
  };
}

/**
 * Pure leadership check used to gate the loot-rule selector + ready check.
 * Matches by character id first, then by member `isLeader` flags.
 */
export function isPartyLeader(party, character) {
  const p = party && typeof party === 'object' ? party : {};
  const me = character && typeof character === 'object' ? character : {};
  const myId = me.id ? String(me.id) : '';
  const myName = me.name ? String(me.name).toLowerCase() : '';
  const leaderId = String(p.leaderId || p.leaderCharacterId || p.LeaderCharacterID || '');
  if (myId && leaderId && myId === leaderId) return true;
  const members = Array.isArray(p.members) ? p.members : [];
  for (const m of members) {
    if (!m) continue;
    const isMe =
      (myId && m.id && String(m.id) === myId) ||
      (myName && m.name && String(m.name).toLowerCase() === myName);
    if (isMe && (m.isLeader || m.leader)) return true;
  }
  return false;
}

/**
 * Derive the ready-check phase from the party chat feed (no dedicated signal
 * exists yet). Scans in order so the newest marker wins:
 *   "[Party] Ready check! ..."        → active
 *   "[Party] Ready: X/Y · Not ready"  → active
 *   "[Party] Ready check over: ..."   → done
 * Returns 'idle' | 'active' | 'done'.
 */
export function classifyReadyCheck(lines) {
  const list = Array.isArray(lines) ? lines : [];
  let state = 'idle';
  for (const line of list) {
    if (!line) continue;
    const text = String(line.text != null ? line.text : (line.message != null ? line.message : ''));
    const lower = text.toLowerCase();
    if (lower.includes('ready check over')) state = 'done';
    else if (lower.includes('ready check')) state = 'active';
    else if (lower.includes('not ready')) state = 'active';
  }
  return state;
}

export function normalizePartyInvite(raw) {
  if (!raw || !raw.pending) return null;
  const expiresAt = Number(raw.expiresAt) || 0;
  return {
    pending: true,
    inviterName: String(raw.inviterName || ''),
    partyId: String(raw.partyId || raw.partyID || ''),
    expiresAt,
  };
}

/** Parse "[Party] Name: text" or "[Party] Name joined/left..." into a strip line. */
export function parsePartyChatLine(text, selfName) {
  const raw = String(text || '').trim();
  if (!raw.startsWith('[Party]')) return null;
  const body = raw.slice('[Party]'.length).trim();
  const say = body.match(/^([^:]+):\s*(.+)$/);
  if (say) {
    const name = say[1].trim();
    return {
      id: `psay-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
      name,
      text: say[2].trim(),
      isYou: !!(selfName && name.toLowerCase() === String(selfName).toLowerCase()),
      system: false,
    };
  }
  return {
    id: `psys-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
    name: '',
    text: body,
    isYou: false,
    system: true,
  };
}
