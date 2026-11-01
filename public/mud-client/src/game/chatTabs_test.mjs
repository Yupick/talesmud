import assert from 'assert';
import {
  CHAT_TAB_META,
  GLOBAL_CHANNELS,
  appendChatLine,
  buildChannelCommand,
  buildTellCommand,
  channelLabel,
  classifyChatMessage,
  classifyChatText,
  createChatState,
  filterChatByChannel,
  filterChatByTab,
  normalizeChannel,
  parseChannelPrefix,
  setActiveTabState,
  unreadForTab,
} from './chatTabs.js';

const you = { id: 'char-gimli', name: 'Gimli' };

// ── Classification ──────────────────────────────────────────────────────────

const sayStructured = classifyChatMessage(
  { type: 'message', username: 'Ranger', message: 'Careful, boars ahead.' },
  you
);
assert.deepStrictEqual(sayStructured, {
  tab: 'say',
  name: 'Ranger',
  text: 'Careful, boars ahead.',
  isYou: false,
  system: false,
  canReply: true,
});

const sayProse = classifyChatMessage('Gimli says: Hold the line.', you);
assert.strictEqual(sayProse.tab, 'say');
assert.strictEqual(sayProse.name, 'Gimli');
assert.strictEqual(sayProse.text, 'Hold the line.');
assert.strictEqual(sayProse.isYou, true);
assert.strictEqual(sayProse.canReply, false, 'self lines are not reply targets');

const tellReceived = classifyChatText('Ranger tells you: Meet me at the gate.', you);
assert.strictEqual(tellReceived.tab, 'tell');
assert.strictEqual(tellReceived.name, 'Ranger');
assert.strictEqual(tellReceived.text, 'Meet me at the gate.');
assert.strictEqual(tellReceived.canReply, true);

const tellSent = classifyChatText('You tell Ranger: On my way.', you);
assert.strictEqual(tellSent.tab, 'tell');
assert.strictEqual(tellSent.name, 'Ranger');
assert.strictEqual(tellSent.isYou, true);
assert.strictEqual(tellSent.canReply, false);

const tellTyped = classifyChatMessage({ type: 'tell', username: 'Ranger', message: 'psst' }, you);
assert.strictEqual(tellTyped.tab, 'tell');
assert.strictEqual(tellTyped.canReply, true);

const partyChat = classifyChatText('[Party] Ranger: pull the left one', you);
assert.strictEqual(partyChat.tab, 'party');
assert.strictEqual(partyChat.name, 'Ranger');
assert.strictEqual(partyChat.text, 'pull the left one');
assert.strictEqual(partyChat.system, false);

const partyEvent = classifyChatText('[Party] Ranger joined the party.', you);
assert.strictEqual(partyEvent.tab, 'party');
assert.strictEqual(partyEvent.system, true);

const systemLine = classifyChatText('You look at the shelves.', you);
assert.strictEqual(systemLine.tab, 'system');
assert.strictEqual(systemLine.system, true);
assert.strictEqual(systemLine.canReply, false);

const systemUsername = classifyChatMessage({ username: 'system', message: 'A chill wind blows.' }, you);
assert.strictEqual(systemUsername.tab, 'system');

assert.strictEqual(classifyChatMessage('', you), null);
assert.strictEqual(classifyChatText('   ', you), null);

// ── Global channels (B7: field + "[Channel] Name: text" prefix) ─────────────

// Structured channel field wins over the default say routing.
const channelStructured = classifyChatMessage(
  { type: 'message', username: 'Ranger', message: '[General] Ranger: anyone around?', channel: 'general' },
  you
);
assert.strictEqual(channelStructured.tab, 'channel');
assert.strictEqual(channelStructured.channel, 'general');
assert.strictEqual(channelStructured.name, 'Ranger');
assert.strictEqual(channelStructured.text, 'anyone around?');
assert.strictEqual(channelStructured.system, false);
assert.strictEqual(channelStructured.canReply, true);

// Uppercase display tag in the field is normalized.
const channelUpper = classifyChatMessage(
  { username: 'Bryn', message: '[LFG] Bryn: need a healer', channel: 'LFG' },
  you
);
assert.strictEqual(channelUpper.tab, 'channel');
assert.strictEqual(channelUpper.channel, 'lfg');
assert.strictEqual(channelUpper.text, 'need a healer');

// Prefix-only routing when the server omits the structured field.
const channelPrefix = classifyChatText('[Trade] Aster: WTB iron ore', you);
assert.strictEqual(channelPrefix.tab, 'channel');
assert.strictEqual(channelPrefix.channel, 'trade');
assert.strictEqual(channelPrefix.name, 'Aster');
assert.strictEqual(channelPrefix.text, 'WTB iron ore');

// Prefix with no speaker becomes a system-style channel line.
const channelNotice = classifyChatText('[Admin] Server restarting in 5m', you);
assert.strictEqual(channelNotice.tab, 'channel');
assert.strictEqual(channelNotice.channel, 'admin');
assert.strictEqual(channelNotice.name, '');
assert.strictEqual(channelNotice.system, true);
assert.strictEqual(channelNotice.canReply, false);

// A line that merely starts with brackets is not a channel.
assert.strictEqual(classifyChatText('[Guild] hi', you).tab, 'system');

assert.strictEqual(normalizeChannel('General'), 'general');
assert.strictEqual(normalizeChannel(' nope '), '');
assert.strictEqual(channelLabel('round'), '');
assert.strictEqual(channelLabel('lfg'), 'LFG');
assert.deepStrictEqual(parseChannelPrefix('[Roleplay] hello'), { channel: 'roleplay', body: 'hello' });
assert.strictEqual(parseChannelPrefix('hello'), null);
assert.strictEqual(GLOBAL_CHANNELS.length, 6);

assert.strictEqual(buildChannelCommand('general'), 'chat general ');
assert.strictEqual(buildChannelCommand('LFG'), 'chat lfg ');
assert.strictEqual(buildChannelCommand('nope'), null);

// ── tell command builder ────────────────────────────────────────────────────

assert.strictEqual(buildTellCommand('Alice says: hola'), 'tell Alice ');
assert.strictEqual(buildTellCommand('Alice tells you: hola'), 'tell Alice ');
assert.strictEqual(buildTellCommand('You tell Alice: hola'), 'tell Alice ');
assert.strictEqual(buildTellCommand({ name: 'Alice' }), 'tell Alice ');
assert.strictEqual(buildTellCommand('Alice'), 'tell Alice ');
assert.strictEqual(buildTellCommand(''), null);
assert.strictEqual(buildTellCommand(null), null);
assert.strictEqual(buildTellCommand('You look at the shelves.'), null);
assert.strictEqual(buildTellCommand('two words'), null);

// ── Tab filtering + unread badges ───────────────────────────────────────────

let state = createChatState();
assert.strictEqual(state.activeTab, 'say');

state = appendChatLine(state, sayStructured); // active tab, no badge
state = appendChatLine(state, tellReceived); // inactive → tell badge
state = appendChatLine(state, partyChat); // inactive → party badge
state = appendChatLine(state, systemLine); // inactive → system badge
state = appendChatLine(state, channelStructured); // inactive → channel badge
state = appendChatLine(state, channelPrefix); // second Ch line (trade)

assert.strictEqual(filterChatByTab(state.messages, 'say').length, 1);
assert.strictEqual(filterChatByTab(state.messages, 'tell').length, 1);
assert.strictEqual(filterChatByTab(state.messages, 'party').length, 1);
assert.strictEqual(filterChatByTab(state.messages, 'channel').length, 2);
assert.strictEqual(filterChatByTab(state.messages, 'system').length, 1);
assert.strictEqual(filterChatByTab(state.messages, 'clan').length, 0);
assert.strictEqual(filterChatByTab(state.messages).length, 6);

// Channel sub-filter narrows the Ch tab only.
assert.strictEqual(filterChatByChannel(state.messages, 'general').length, 1);
assert.strictEqual(filterChatByChannel(state.messages, 'trade').length, 1);
assert.strictEqual(filterChatByChannel(state.messages, 'lfg').length, 0);
assert.strictEqual(filterChatByChannel(state.messages).length, 2);

assert.strictEqual(unreadForTab(state.unread, 'say'), 0);
assert.strictEqual(unreadForTab(state.unread, 'tell'), 1);
assert.strictEqual(unreadForTab(state.unread, 'party'), 1);
assert.strictEqual(unreadForTab(state.unread, 'channel'), 2);
assert.strictEqual(unreadForTab(state.unread, 'system'), 1);

// Switching to a tab clears only that tab's badge.
state = setActiveTabState(state, 'tell');
assert.strictEqual(state.activeTab, 'tell');
assert.strictEqual(unreadForTab(state.unread, 'tell'), 0);
assert.strictEqual(unreadForTab(state.unread, 'party'), 1);

// A new message on the now-active tab does not raise a badge.
state = appendChatLine(state, classifyChatText('Ranger tells you: again', you));
assert.strictEqual(unreadForTab(state.unread, 'tell'), 0);

// Clan is a disabled placeholder, never a data tab.
const clan = CHAT_TAB_META.find((t) => t.id === 'clan');
assert.ok(clan);
assert.strictEqual(clan.disabled, true);
assert.strictEqual(clan.title, 'Clans coming soon');
assert.ok(!CHAT_TAB_META.some((t) => t.id === 'clan' && !t.disabled));

console.log('chatTabs_test.mjs ok');
