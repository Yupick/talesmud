import assert from "assert";
import {
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
} from "./clans.js";

// base.js resolves to http://localhost:8010/api when window is undefined.
const BASE = "http://localhost:8010/api";
const TOKEN = "jwt-clan";

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

// POST /clans
{
  mockFetch(() => jsonResponse(200, { id: "c1", name: "Dragons", tag: "DRK" }));
  const data = await createClan(TOKEN, {
    characterId: "char1",
    name: "Dragons",
    tag: "DRK",
    description: "Fire",
  });
  assert.equal(data.id, "c1");
  assert.equal(calls.length, 1);
  assert.equal(last().url, `${BASE}/clans`);
  assert.equal(last().options.method, "POST");
  assertAuth(last());
  assert.equal(last().options.headers["Content-Type"], "application/json");
  assert.deepEqual(JSON.parse(last().options.body), {
    characterId: "char1",
    name: "Dragons",
    tag: "DRK",
    description: "Fire",
  });
}

// GET /clans/mine
{
  mockFetch(() => jsonResponse(200, { clan: null, members: [], invites: [] }));
  const data = await getMyClan(TOKEN, "char1");
  assert.deepEqual(data, { clan: null, members: [], invites: [] });
  assert.equal(last().url, `${BASE}/clans/mine?characterId=char1`);
  assert.equal(last().options.method, "GET");
  assertAuth(last());
  assert.equal(last().options.body, undefined);
}

// POST /clans/:id/invite
{
  mockFetch(() => jsonResponse(200, { id: "inv1", status: "pending" }));
  const data = await inviteToClan(TOKEN, "c1", {
    characterId: "char1",
    targetCharacterId: "char2",
  });
  assert.equal(data.id, "inv1");
  assert.equal(last().url, `${BASE}/clans/c1/invite`);
  assert.equal(last().options.method, "POST");
  assertAuth(last());
  assert.deepEqual(JSON.parse(last().options.body), {
    characterId: "char1",
    targetCharacterId: "char2",
  });
}

// POST /clans/invites/:inviteId/accept
{
  mockFetch(() => jsonResponse(200, { status: "accepted" }));
  const data = await acceptClanInvite(TOKEN, "inv1", "char2");
  assert.deepEqual(data, { status: "accepted" });
  assert.equal(last().url, `${BASE}/clans/invites/inv1/accept`);
  assert.equal(last().options.method, "POST");
  assert.deepEqual(JSON.parse(last().options.body), { characterId: "char2" });
}

// POST /clans/invites/:inviteId/decline
{
  mockFetch(() => jsonResponse(200, { status: "declined" }));
  const data = await declineClanInvite(TOKEN, "inv1", "char2");
  assert.deepEqual(data, { status: "declined" });
  assert.equal(last().url, `${BASE}/clans/invites/inv1/decline`);
  assert.equal(last().options.method, "POST");
  assert.deepEqual(JSON.parse(last().options.body), { characterId: "char2" });
}

// POST /clans/:id/kick
{
  mockFetch(() => jsonResponse(200, { status: "kicked" }));
  await kickFromClan(TOKEN, "c1", {
    characterId: "char1",
    targetCharacterId: "char2",
  });
  assert.equal(last().url, `${BASE}/clans/c1/kick`);
  assert.deepEqual(JSON.parse(last().options.body), {
    characterId: "char1",
    targetCharacterId: "char2",
  });
}

// POST /clans/:id/rank
{
  mockFetch(() => jsonResponse(200, { status: "rank updated" }));
  await changeClanRank(TOKEN, "c1", {
    characterId: "char1",
    targetCharacterId: "char2",
    rank: "officer",
  });
  assert.equal(last().url, `${BASE}/clans/c1/rank`);
  assert.deepEqual(JSON.parse(last().options.body), {
    characterId: "char1",
    targetCharacterId: "char2",
    rank: "officer",
  });
}

// POST /clans/:id/deposit
{
  mockFetch(() => jsonResponse(200, { status: "deposited", balance: 150 }));
  const data = await depositToClan(TOKEN, "c1", { characterId: "char1", amount: 100 });
  assert.equal(data.balance, 150);
  assert.equal(last().url, `${BASE}/clans/c1/deposit`);
  assert.deepEqual(JSON.parse(last().options.body), { characterId: "char1", amount: 100 });
}

// POST /clans/:id/withdraw
{
  mockFetch(() => jsonResponse(200, { status: "withdrawn", balance: 50 }));
  const data = await withdrawFromClan(TOKEN, "c1", { characterId: "char1", amount: 50 });
  assert.equal(data.balance, 50);
  assert.equal(last().url, `${BASE}/clans/c1/withdraw`);
  assert.deepEqual(JSON.parse(last().options.body), { characterId: "char1", amount: 50 });
}

// POST /clans/:id/disband
{
  mockFetch(() => jsonResponse(200, { status: "disbanded" }));
  const data = await disbandClan(TOKEN, "c1", "char1");
  assert.deepEqual(data, { status: "disbanded" });
  assert.equal(last().url, `${BASE}/clans/c1/disband`);
  assert.deepEqual(JSON.parse(last().options.body), { characterId: "char1" });
}

// Backend error surfaces the {error} message.
{
  mockFetch(() => jsonResponse(403, { error: "insufficient rank to invite" }));
  await assert.rejects(
    inviteToClan(TOKEN, "c1", { characterId: "char1", targetCharacterId: "char2" }),
    /insufficient rank to invite/
  );
}

// Non-JSON error falls back to a status message.
{
  mockFetch(() => ({
    ok: false,
    status: 502,
    json: async () => {
      throw new Error("not json");
    },
  }));
  await assert.rejects(getMyClan(TOKEN, "char1"), /Request failed \(502\)/);
}

// Network failure is reported consistently.
{
  mockFetch(() => {
    throw new Error("connection refused");
  });
  await assert.rejects(disbandClan(TOKEN, "c1", "char1"), /Network error/);
}

console.log("clans_test ok");
