// Player chat tab helpers (Task 3.3).
//
// The server does not send a distinct `tell`/`party`/`system` message type for
// every line: `say` arrives as a room broadcast ("Name says: text" or a
// username-bearing default message) and `tell`/`party` arrive as prose
// ("Name tells you: text", "[Party] Name: text"). Everything else that the
// Global channels (B7) arrive with a structured `channel` field and/or a
// "[General] Name: text" prefix and route to the Ch tab. Everything else the
// client logs (room descriptions, combat, quests, level-ups, position text)
// falls back to the System tab. These helpers are pure so they can be unit
// tested with `node` like the other `*_test.mjs` files.

import { isSelfName, parseRoomChatLine } from './roomChat.js';
import { parsePartyChatLine } from './partyState.js';

/** Tabs that hold data. `clan` is a disabled placeholder handled by the UI. */
export const CHAT_TABS = ['say', 'tell', 'party', 'channel', 'system'];

/** Tab order + labels rendered by ChatPanel (Clan is disabled / "coming soon"). */
export const CHAT_TAB_META = [
  { id: 'say', label: 'Say' },
  { id: 'tell', label: 'Tell' },
  { id: 'party', label: 'Party' },
  { id: 'channel', label: 'Ch' },
  { id: 'system', label: 'System' },
  { id: 'clan', label: 'Clan', disabled: true, title: 'Clans coming soon' },
];

/**
 * Global chat channels (B7). The server tags channel lines with a `channel`
 * field and also prefixes the prose with "[General] Name: text". Both forms
 * are normalized here so the Ch tab can route and filter them.
 */
export const GLOBAL_CHANNELS = [
  { id: 'general', label: 'General' },
  { id: 'trade', label: 'Trade' },
  { id: 'lfg', label: 'LFG' },
  { id: 'help', label: 'Help' },
  { id: 'roleplay', label: 'Roleplay' },
  { id: 'admin', label: 'Admin' },
];

const CHANNEL_IDS = GLOBAL_CHANNELS.map((c) => c.id);

export const DEFAULT_CHAT_TAB = 'say';
export const MAX_CHAT_MESSAGES = 300;

// Message types a future structured client/server could use for direct tells.
const TELL_TYPES = ['tell', 'whisper', 'dm', 'direct'];

const SAY_RE = /^(.+?)\s+says:\s*(.*)$/i;
const TELL_RE = /^(.+?)\s+tells you:\s*(.*)$/i;
const YOU_TELL_RE = /^You tell\s+(.+?):\s*(.*)$/i;
const PLAIN_NAME_RE = /^[A-Za-z0-9_'.-]+$/;
// "[General] Name: text" / "[LFG] anyone?" global-channel prose prefix (B7).
const CHANNEL_PREFIX_RE = /^\[([^\]]+)\]\s*(.*)$/;

function makeLine(tab, name, text, character, opts = {}) {
  const cleanName = String(name || '').trim();
  const self = isSelfName(cleanName, character);
  const line = {
    tab,
    name: cleanName,
    text: String(text == null ? '' : text),
    isYou: opts.isYou != null ? opts.isYou : self,
    system: !!opts.system,
    canReply: opts.canReply != null ? opts.canReply : Boolean(cleanName) && !self,
  };
  if (opts.channel) line.channel = opts.channel;
  return line;
}

/** Normalize a channel id/label ("General", "LFG") to its canonical id. */
export function normalizeChannel(value) {
  const v = String(value == null ? '' : value).trim().toLowerCase();
  return CHANNEL_IDS.includes(v) ? v : '';
}

/** Human label for a channel id ("lfg" → "LFG"). */
export function channelLabel(value) {
  const id = normalizeChannel(value);
  const found = GLOBAL_CHANNELS.find((c) => c.id === id);
  return found ? found.label : '';
}

/** Split "[General] body" into `{ channel, body }`; null when not a channel line. */
export function parseChannelPrefix(text) {
  const m = String(text == null ? '' : text).match(CHANNEL_PREFIX_RE);
  if (!m) return null;
  const channel = normalizeChannel(m[1]);
  if (!channel) return null;
  return { channel, body: m[2] || '' };
}

/**
 * Build a channel chat line from a structured payload or a prefixed prose line.
 * `username` wins as the speaker; otherwise a leading "Name: " in the body is
 * treated as the speaker and stripped.
 */
export function buildChannelLine(channel, username, message, character) {
  const id = normalizeChannel(channel);
  if (!id) return null;

  let body = String(message == null ? '' : message).trim();
  const prefixed = parseChannelPrefix(body);
  if (prefixed && prefixed.channel === id) body = prefixed.body.trim();

  let name = String(username || '').trim();
  let text = body;
  const colon = body.match(/^([^:]+):\s*(.*)$/);
  if (colon) {
    const candidate = colon[1].trim();
    if (!name) name = candidate;
    if (!username || candidate.toLowerCase() === name.toLowerCase()) {
      text = colon[2].trim();
    }
  }

  return makeLine('channel', name, text, character, { channel: id, system: !name });
}

/** Classify a free-text console line into a chat tab. Returns null for blanks. */
export function classifyChatText(rawText, character) {
  const text = String(rawText == null ? '' : rawText).trim();
  if (!text) return null;

  // Party chat + party system events ("[Party] Name: text" / "[Party] X left").
  const party = parsePartyChatLine(text, character && character.name);
  if (party) {
    if (party.system) {
      return makeLine('party', '', party.text, character, { system: true });
    }
    return makeLine('party', party.name, party.text, character, { isYou: party.isYou });
  }

  // Global channel prose ("[General] Name: text") when no structured field.
  const channel = parseChannelPrefix(text);
  if (channel) {
    const line = buildChannelLine(channel.channel, '', channel.body, character);
    if (line) return line;
  }

  // Direct tell received: "Alice tells you: hi".
  const tell = text.match(TELL_RE);
  if (tell) return makeLine('tell', tell[1], tell[2], character);

  // Direct tell sent: "You tell Alice: hi".
  const youTell = text.match(YOU_TELL_RE);
  if (youTell) {
    return makeLine('tell', youTell[1], youTell[2], character, { isYou: true, canReply: false });
  }

  // Room say / emote. parseRoomChatLine already understands both the
  // username-bearing default message and the "Name says: text" prose form.
  const room = parseRoomChatLine({ type: 'message', message: text }, character);
  if (room) return makeLine('say', room.name, room.text, character, { isYou: room.isYou });

  // Anything else the client prints (room look, combat, quest, level-up...).
  return makeLine('system', '', text, character, { system: true });
}

/**
 * Classify a value passed to the terminal renderer: either a structured
 * `{ username, message }` payload or a plain string line.
 */
export function classifyChatMessage(data, character) {
  if (data == null) return null;

  if (typeof data === 'object') {
    const type = String(data.type || '');
    const username = String(data.username || data.speaker || '').trim();
    const message = String(data.message != null ? data.message : (data.text != null ? data.text : ''));

    // Structured global channel tag (B7) takes precedence over say routing.
    const channel = normalizeChannel(data.channel);
    if (channel) {
      const line = buildChannelLine(channel, username, message, character);
      if (line) return line;
    }

    if (TELL_TYPES.includes(type)) {
      const fromName = username;
      return makeLine('tell', fromName, message, character, {
        canReply: Boolean(fromName) && !isSelfName(fromName, character),
      });
    }

    if (username) {
      if (/^system$/i.test(username)) {
        return makeLine('system', '', message, character, { system: true });
      }
      // Default `message` with a username is the speaker's chat body.
      return makeLine('say', username, message, character);
    }

    return classifyChatText(message, character);
  }

  return classifyChatText(String(data), character);
}

/**
 * Build the exact DM command for click-to-respond: `tell <name> ` (the server
 * parses `tell <player> <message>`). Accepts a chat line object, a raw name,
 * or a full line such as "Alice says: hi".
 */
export function buildTellCommand(source) {
  let name = '';

  if (source && typeof source === 'object') {
    name = source.name || source.speaker || '';
  } else {
    const text = String(source == null ? '' : source).trim();
    if (!text) return null;
    const tell = text.match(TELL_RE);
    const youTell = text.match(YOU_TELL_RE);
    const say = text.match(SAY_RE);
    if (tell) name = tell[1];
    else if (youTell) name = youTell[1];
    else if (say) name = say[1];
    else if (!text.includes(':')) name = text;
    else return null;
  }

  // Strip surrounding punctuation but keep the casing the player typed.
  name = String(name).trim().replace(/[.,!?;:]+$/, '').trim();
  // `tell` splits on whitespace, so a player name is a single token.
  if (!name || !PLAIN_NAME_RE.test(name)) return null;
  return `tell ${name} `;
}

/**
 * Build the quick-reply command for a global channel: `chat <channel> `.
 * Returns null for unknown channels.
 */
export function buildChannelCommand(channel) {
  const id = normalizeChannel(channel);
  if (!id) return null;
  return `chat ${id} `;
}

export function filterChatByTab(messages, tab) {
  const list = Array.isArray(messages) ? messages : [];
  if (!tab) return list;
  return list.filter((line) => line && line.tab === tab);
}

/** Narrow Ch tab lines to a single channel id (empty channel → every Ch line). */
export function filterChatByChannel(messages, channel) {
  const list = Array.isArray(messages) ? messages : [];
  const id = normalizeChannel(channel);
  return list.filter((line) => line && line.tab === 'channel' && (!id || line.channel === id));
}

export function createChatState() {
  return { messages: [], activeTab: DEFAULT_CHAT_TAB, unread: {} };
}

export function appendChatLine(state, line) {
  const base = state && Array.isArray(state.messages) ? state : createChatState();
  if (!line || !CHAT_TABS.includes(line.tab)) return base;

  const messages = [...base.messages, line];
  if (messages.length > MAX_CHAT_MESSAGES) {
    messages.splice(0, messages.length - MAX_CHAT_MESSAGES);
  }

  const unread = { ...(base.unread || {}) };
  if (line.tab !== base.activeTab) {
    unread[line.tab] = (unread[line.tab] || 0) + 1;
  }

  return { ...base, messages, unread };
}

export function setActiveTabState(state, tab) {
  const base = state && Array.isArray(state.messages) ? state : createChatState();
  return {
    ...base,
    activeTab: tab,
    unread: { ...(base.unread || {}), [tab]: 0 },
  };
}

export function unreadForTab(unread, tab) {
  return Number((unread && unread[tab]) || 0);
}
