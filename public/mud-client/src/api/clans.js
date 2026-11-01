// Clan REST helpers (Phase 4 clan system).
//
// Every endpoint is authenticated with the session JWT as
// `Authorization: Bearer <token>`. The acting `characterId` travels in the
// query string for GET calls and in the JSON body for POST calls, matching the
// Go handlers in `pkg/server/handler/clans.go`. Backend errors arrive as
// `{error: "..."}`; we surface them as readable `Error` messages.
import { backend } from "./base.js";

// Perform a JSON request and normalise failures. GET passes no body; POST
// always sends `{}` at minimum so Gin's JSON binder is happy.
async function request(token, method, path, body) {
  const options = {
    method,
    credentials: "same-origin",
    headers: { Authorization: `Bearer ${token}` },
  };
  if (body !== undefined) {
    options.headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body || {});
  }

  let response;
  try {
    response = await fetch(`${backend}${path}`, options);
  } catch (err) {
    throw new Error("Network error. Please check your connection and try again.");
  }

  let data = null;
  try {
    data = await response.json();
  } catch (err) {
    data = null;
  }

  if (!response.ok) {
    const message =
      (data && (data.error || data.message)) ||
      `Request failed (${response.status})`;
    throw new Error(message);
  }

  return data;
}

// Drop empty query params so optional filters never reach the backend as "".
function withQuery(path, params) {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params || {})) {
    if (value === undefined || value === null || value === "") continue;
    qs.set(key, String(value));
  }
  const query = qs.toString();
  return query ? `${path}?${query}` : path;
}

/**
 * Create a clan and become its leader.
 * @param {string} token session JWT
 * @param {{characterId:string, name:string, tag:string, description?:string}} payload
 * @returns {Promise<object>} the created clan
 */
function createClan(token, { characterId, name, tag, description }) {
  return request(token, "POST", "/clans", { characterId, name, tag, description });
}

/**
 * Load the character's clan, its members and pending invites.
 * @param {string} token session JWT
 * @param {string} characterId
 * @returns {Promise<{clan:object|null, members:Array<object>, invites:Array<object>}>}
 */
function getMyClan(token, characterId) {
  return request(token, "GET", withQuery("/clans/mine", { characterId }));
}

/**
 * Invite another character to the clan (leader/officer only).
 * @param {string} token
 * @param {string} clanId
 * @param {{characterId:string, targetCharacterId:string}} payload
 */
function inviteToClan(token, clanId, { characterId, targetCharacterId }) {
  return request(token, "POST", `/clans/${encodeURIComponent(clanId)}/invite`, {
    characterId,
    targetCharacterId,
  });
}

/**
 * Accept a pending clan invite for the character.
 * @param {string} token
 * @param {string} inviteId
 * @param {string} characterId
 */
function acceptClanInvite(token, inviteId, characterId) {
  return request(
    token,
    "POST",
    `/clans/invites/${encodeURIComponent(inviteId)}/accept`,
    { characterId }
  );
}

/**
 * Decline a pending clan invite for the character.
 * @param {string} token
 * @param {string} inviteId
 * @param {string} characterId
 */
function declineClanInvite(token, inviteId, characterId) {
  return request(
    token,
    "POST",
    `/clans/invites/${encodeURIComponent(inviteId)}/decline`,
    { characterId }
  );
}

/**
 * Kick a member from the clan.
 * @param {string} token
 * @param {string} clanId
 * @param {{characterId:string, targetCharacterId:string}} payload
 */
function kickFromClan(token, clanId, { characterId, targetCharacterId }) {
  return request(token, "POST", `/clans/${encodeURIComponent(clanId)}/kick`, {
    characterId,
    targetCharacterId,
  });
}

/**
 * Change a member's rank (`member` | `officer` | `leader`).
 * @param {string} token
 * @param {string} clanId
 * @param {{characterId:string, targetCharacterId:string, rank:string}} payload
 */
function changeClanRank(token, clanId, { characterId, targetCharacterId, rank }) {
  return request(token, "POST", `/clans/${encodeURIComponent(clanId)}/rank`, {
    characterId,
    targetCharacterId,
    rank,
  });
}

/**
 * Deposit gold into the clan bank.
 * @param {string} token
 * @param {string} clanId
 * @param {{characterId:string, amount:number}} payload
 */
function depositToClan(token, clanId, { characterId, amount }) {
  return request(token, "POST", `/clans/${encodeURIComponent(clanId)}/deposit`, {
    characterId,
    amount,
  });
}

/**
 * Withdraw gold from the clan bank (leader only server-side).
 * @param {string} token
 * @param {string} clanId
 * @param {{characterId:string, amount:number}} payload
 */
function withdrawFromClan(token, clanId, { characterId, amount }) {
  return request(token, "POST", `/clans/${encodeURIComponent(clanId)}/withdraw`, {
    characterId,
    amount,
  });
}

/**
 * Disband the clan (leader only).
 * @param {string} token
 * @param {string} clanId
 * @param {string} characterId
 */
function disbandClan(token, clanId, characterId) {
  return request(token, "POST", `/clans/${encodeURIComponent(clanId)}/disband`, {
    characterId,
  });
}

export {
  createClan,
  getMyClan,
  inviteToClan,
  acceptClanInvite,
  declineClanInvite,
  kickFromClan,
  changeClanRank,
  depositToClan,
  withdrawFromClan,
  disbandClan,
};
