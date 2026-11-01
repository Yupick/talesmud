// Guest tokens live in this tab only. A real Auth0 login must win over one,
// and logout must drop it so the next load is the welcome choice.
//
// Local username/password sessions use localStorage so an account survives a
// reload (and a new tab), mirroring Auth0's cacheLocation: "localstorage".

export const GUEST_TOKEN_KEY = "talesmud_guest_token";
export const PICKER_SEEN_KEY = "talesmud_char_picker_seen";
export const LOCAL_TOKEN_KEY = "talesmud_local_token";

function defaultStorage() {
  try {
    if (typeof sessionStorage === "undefined") return null;
    return sessionStorage;
  } catch (err) {
    return null;
  }
}

function defaultLocalStorage() {
  try {
    if (typeof localStorage === "undefined") return null;
    return localStorage;
  } catch (err) {
    return null;
  }
}

function storeOf(storage) {
  return storage === undefined ? defaultStorage() : storage;
}

function localStoreOf(storage) {
  return storage === undefined ? defaultLocalStorage() : storage;
}

export function readGuestToken(storage) {
  const store = storeOf(storage);
  if (!store) return "";
  try {
    return store.getItem(GUEST_TOKEN_KEY) || "";
  } catch (err) {
    return "";
  }
}

export function clearGuestToken(storage) {
  const store = storeOf(storage);
  if (!store) return;
  try {
    store.removeItem(GUEST_TOKEN_KEY);
  } catch (err) {
    /* ignore */
  }
}

export function readLocalToken(storage) {
  const store = localStoreOf(storage);
  if (!store) return "";
  try {
    return store.getItem(LOCAL_TOKEN_KEY) || "";
  } catch (err) {
    return "";
  }
}

export function writeLocalToken(token, storage) {
  const store = localStoreOf(storage);
  if (!store || !token) return;
  try {
    store.setItem(LOCAL_TOKEN_KEY, token);
  } catch (err) {
    /* ignore */
  }
}

export function clearLocalToken(storage) {
  const store = localStoreOf(storage);
  if (!store) return;
  try {
    store.removeItem(LOCAL_TOKEN_KEY);
  } catch (err) {
    /* ignore */
  }
}

// True only when this tab's token is a stored local (username/password) token.
export function isLocalSession(authToken, storage) {
  const local = readLocalToken(storage);
  return !!authToken && !!local && authToken === local;
}

// True only when this tab's token is the guest token. A leftover guest token
// must not mark an Auth0 session as a guest.
export function isGuestSession(authToken, storage) {
  const guest = readGuestToken(storage);
  return !!authToken && !!guest && authToken === guest;
}

// auth0 | local | guest | welcome
export function restoredSession({ auth0Authenticated, guestToken, localToken }) {
  if (auth0Authenticated) return "auth0";
  if (localToken) return "local";
  if (guestToken) return "guest";
  return "welcome";
}

// Open the character picker once per login when a signed-in player has a choice.
export function shouldAutoOpenCharacterPicker({ guest, seen, characterCount }) {
  if (guest || seen) return false;
  return characterCount > 1;
}

// Drop the guest token, the local session token and the one-shot character
// picker flag. Logout must leave no session behind.
export function clearLocalSession(storage) {
  const store = storeOf(storage);
  clearGuestToken(store);
  clearLocalToken(storage);
  if (!store) return;
  try {
    store.removeItem(PICKER_SEEN_KEY);
  } catch (err) {
    /* ignore */
  }
}
