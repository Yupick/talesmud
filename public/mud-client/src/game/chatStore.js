// Reactive chat state for the tabbed player chat panel (Task 3.3).
//
// Every renderer payload is classified once here so all tabs share a single
// ordered log. The store also exposes a tiny command-request channel that the
// terminal widgets listen on for click-to-respond.

import { writable } from 'svelte/store';
import {
  appendChatLine,
  classifyChatMessage,
  createChatState,
  setActiveTabState,
} from './chatTabs.js';

function createChatStore() {
  const { subscribe, set, update } = writable(createChatState());

  let self = null;
  let selfKey = '';
  let sequence = 0;

  return {
    subscribe,

    /** Keep the current character so `isYou` / reply targets are accurate. */
    setSelf(character) {
      const key = `${character && character.id ? character.id : ''}|${character && character.name ? character.name : ''}`;
      if (key === selfKey) return;
      selfKey = key;
      self = character || null;
    },

    /** Classify one renderer payload (`{username,message}` or a string line). */
    ingest(data) {
      const line = classifyChatMessage(data, self);
      if (!line) return;
      sequence += 1;
      update((state) => appendChatLine(state, { ...line, id: `chat-${sequence}` }));
    },

    setActiveTab(tab) {
      update((state) => setActiveTabState(state, tab));
    },

    clear() {
      sequence = 0;
      set(createChatState());
    },
  };
}

export const chatStore = createChatStore();

// ── Click-to-respond channel ─────────────────────────────────────────────────
// ChatPanel requests a prefilled command; every mounted terminal fills its
// input and focuses. `seq` lets repeated identical requests fire again.

const commandRequest = writable({ command: '', seq: 0 });
let commandSeq = 0;

export const chatCommandRequest = { subscribe: commandRequest.subscribe };

export function requestChatCommand(command) {
  const cmd = String(command || '');
  if (!cmd) return;
  commandSeq += 1;
  commandRequest.set({ command: cmd, seq: commandSeq });
}
