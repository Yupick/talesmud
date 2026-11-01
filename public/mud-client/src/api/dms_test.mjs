import assert from "assert";
import {
  getDMChannels,
  createDMChannel,
  getDMMessages,
  sendDMMessage,
  markDMChannelRead,
} from "./dms.js";

// base.js resolves to http://localhost:8010/api when window is undefined.
const BASE = "http://localhost:8010/api";
const TOKEN = "jwt-dm";

let calls = [];

function jsonResponse(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  };
}

function mockFetch(handler) {
  calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return handler(url, options);
  };
}

function last() {
  return calls[calls.length - 1];
}

function assertAuth(call) {
  assert.equal(call.options.headers.Authorization, `Bearer ${TOKEN}`);
}

// GET /dm/channels
{
  mockFetch(() => jsonResponse(200, [{ id: "ch1" }]));
  const data = await getDMChannels(TOKEN, "char1");
  assert.equal(data.length, 1);
  assert.equal(last().url, `${BASE}/dm/channels?characterId=char1`);
  assert.equal(last().options.method, "GET");
  assertAuth(last());
  assert.equal(last().options.body, undefined);
}

// POST /dm/channels (find-or-create)
{
  mockFetch(() => jsonResponse(200, { id: "ch1", participantA: "char1", participantB: "char2" }));
  const data = await createDMChannel(TOKEN, {
    characterId: "char1",
    targetCharacterId: "char2",
  });
  assert.equal(data.id, "ch1");
  assert.equal(last().url, `${BASE}/dm/channels`);
  assert.equal(last().options.method, "POST");
  assertAuth(last());
  assert.equal(last().options.headers["Content-Type"], "application/json");
  assert.deepEqual(JSON.parse(last().options.body), {
    characterId: "char1",
    targetCharacterId: "char2",
  });
}

// GET /dm/channels/:id/messages (limit + since)
{
  mockFetch(() => jsonResponse(200, [{ id: "m1", body: "hi" }]));
  const data = await getDMMessages(TOKEN, "ch1", {
    characterId: "char1",
    limit: 25,
    since: "2026-01-01T00:00:00Z",
  });
  assert.equal(data.length, 1);
  assert.equal(
    last().url,
    `${BASE}/dm/channels/ch1/messages?characterId=char1&limit=25&since=2026-01-01T00%3A00%3A00Z`
  );
  assert.equal(last().options.method, "GET");
  assertAuth(last());
  assert.equal(last().options.body, undefined);
}

// GET /dm/channels/:id/messages without optional filters.
{
  mockFetch(() => jsonResponse(200, []));
  await getDMMessages(TOKEN, "ch1", { characterId: "char1" });
  assert.equal(last().url, `${BASE}/dm/channels/ch1/messages?characterId=char1`);
}

// POST /dm/channels/:id/messages
{
  mockFetch(() => jsonResponse(200, { id: "m2", body: "hello" }));
  const data = await sendDMMessage(TOKEN, "ch1", { characterId: "char1", body: "hello" });
  assert.equal(data.body, "hello");
  assert.equal(last().url, `${BASE}/dm/channels/ch1/messages`);
  assert.equal(last().options.method, "POST");
  assertAuth(last());
  assert.deepEqual(JSON.parse(last().options.body), { characterId: "char1", body: "hello" });
}

// POST /dm/channels/:id/read
{
  mockFetch(() => jsonResponse(200, { status: "read" }));
  const data = await markDMChannelRead(TOKEN, "ch1", "char1");
  assert.deepEqual(data, { status: "read" });
  assert.equal(last().url, `${BASE}/dm/channels/ch1/read`);
  assert.equal(last().options.method, "POST");
  assertAuth(last());
  assert.deepEqual(JSON.parse(last().options.body), { characterId: "char1" });
}

// Channel ids are encoded in the path.
{
  mockFetch(() => jsonResponse(200, { id: "a/b" }));
  await getDMMessages(TOKEN, "a/b", { characterId: "char1" });
  assert.equal(last().url, `${BASE}/dm/channels/a%2Fb/messages?characterId=char1`);
}

// Backend error surfaces the {error} message.
{
  mockFetch(() => jsonResponse(403, { error: "not a participant of this channel" }));
  await assert.rejects(
    sendDMMessage(TOKEN, "ch1", { characterId: "char1", body: "x" }),
    /not a participant of this channel/
  );
}

// Non-JSON error falls back to a status message.
{
  mockFetch(() => ({
    ok: false,
    status: 500,
    json: async () => {
      throw new Error("not json");
    },
  }));
  await assert.rejects(getDMChannels(TOKEN, "char1"), /Request failed \(500\)/);
}

// Network failure is reported consistently.
{
  mockFetch(() => {
    throw new Error("connection refused");
  });
  await assert.rejects(markDMChannelRead(TOKEN, "ch1", "char1"), /Network error/);
}

console.log("dms_test ok");
