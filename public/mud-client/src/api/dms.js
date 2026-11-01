// Direct-message REST helpers (Phase 4 private messaging).
//
// Authenticates with the session JWT as `Authorization: Bearer <token>`. The
// acting `characterId` is a query param on GET calls and a JSON body field on
// POST calls, matching `pkg/server/handler/dms.go`. Errors come back as
// `{error: "..."}` and are rethrown as `Error`.
import { backend } from "./base.js";

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
 * List the character's DM conversation channels.
 * @param {string} token session JWT
 * @param {string} characterId
 * @returns {Promise<Array<object>>} channels (participantA/participantB ids)
 */
function getDMChannels(token, characterId) {
  return request(token, "GET", withQuery("/dm/channels", { characterId }));
}

/**
 * Find-or-create the conversation channel with another character.
 * @param {string} token
 * @param {{characterId:string, targetCharacterId:string}} payload
 * @returns {Promise<object>} the channel
 */
function createDMChannel(token, { characterId, targetCharacterId }) {
  return request(token, "POST", "/dm/channels", { characterId, targetCharacterId });
}

/**
 * Fetch messages in a channel.
 * @param {string} token
 * @param {string} channelId
 * @param {{characterId:string, limit?:number, since?:string}} params
 * @returns {Promise<Array<object>>} messages
 */
function getDMMessages(token, channelId, { characterId, limit, since } = {}) {
  return request(
    token,
    "GET",
    withQuery(`/dm/channels/${encodeURIComponent(channelId)}/messages`, {
      characterId,
      limit,
      since,
    })
  );
}

/**
 * Send a private message in a channel.
 * @param {string} token
 * @param {string} channelId
 * @param {{characterId:string, body:string}} payload
 * @returns {Promise<object>} the stored message
 */
function sendDMMessage(token, channelId, { characterId, body }) {
  return request(
    token,
    "POST",
    `/dm/channels/${encodeURIComponent(channelId)}/messages`,
    { characterId, body }
  );
}

/**
 * Mark all messages in a channel as read for the character.
 * @param {string} token
 * @param {string} channelId
 * @param {string} characterId
 */
function markDMChannelRead(token, channelId, characterId) {
  return request(
    token,
    "POST",
    `/dm/channels/${encodeURIComponent(channelId)}/read`,
    { characterId }
  );
}

export {
  getDMChannels,
  createDMChannel,
  getDMMessages,
  sendDMMessage,
  markDMChannelRead,
};
