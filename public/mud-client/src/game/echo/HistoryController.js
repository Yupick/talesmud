/**
 * History wrapper used by the classic xterm terminal (LocalEchoController).
 *
 * Delegates to the pure helpers in `../commandHistory.js` so the desktop
 * terminal and TerminalX share the same rules: consecutive dedup, ~50 entry
 * cap, localStorage persistence and draft-preserving ArrowUp/ArrowDown.
 */

import {
  HISTORY_LIMIT,
  createHistoryState,
  historyNext,
  historyPrev,
  loadHistory,
  pushHistory,
  resetHistoryCursor,
  saveHistory,
} from "../commandHistory.js";

export class HistoryController {
  constructor(sizeOrOptions, storage) {
    let size = HISTORY_LIMIT;
    let store = storage;
    if (typeof sizeOrOptions === "number") {
      size = sizeOrOptions > 0 ? sizeOrOptions : HISTORY_LIMIT;
    } else if (sizeOrOptions && typeof sizeOrOptions === "object") {
      size = sizeOrOptions.size > 0 ? sizeOrOptions.size : HISTORY_LIMIT;
      store = sizeOrOptions.storage;
    }
    this.size = size;
    this.storage = store;
    this.state = createHistoryState(loadHistory(store, size));
  }

  /** Expose the raw entries (some call sites/tests read `.entries`). */
  get entries() {
    return this.state.list;
  }

  get cursor() {
    return this.state.cursor;
  }

  push(entry) {
    const list = pushHistory(this.state.list, entry, this.size);
    this.state = createHistoryState(list);
    saveHistory(list, this.storage);
  }

  /** Rewind the cursor to the newest entry (used by CTRL+C). */
  rewind() {
    this.state = resetHistoryCursor(this.state);
  }

  /**
   * Returns the previous entry while preserving the current draft.
   * `currentInput` is what the user had typed before pressing ArrowUp.
   */
  getPrevious(currentInput = "") {
    const next = historyPrev(this.state, currentInput);
    this.state = { list: next.list, cursor: next.cursor, draft: next.draft };
    return next.text;
  }

  /**
   * Returns the next entry, restoring the saved draft past the newest entry.
   */
  getNext(currentInput = "") {
    const next = historyNext(this.state, currentInput);
    this.state = { list: next.list, cursor: next.cursor, draft: next.draft };
    return next.text;
  }
}
