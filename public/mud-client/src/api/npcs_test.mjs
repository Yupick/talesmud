import assert from "node:assert/strict";
import {
  listNpcTemplates,
  getNpcTemplate,
  createNpcTemplate,
  updateNpcTemplate,
  deleteNpcTemplate,
  isNpcRouteError,
  NPC_CRUD_AVAILABLE,
  NPC_ENDPOINT_STATUS,
  // Legacy /api/npcs instance routes (deprecated).
  listNpcs,
  listNpcTemplatesLegacy,
  getNpc,
  createNpc,
  updateNpc,
  deleteNpc,
  spawnNpc,
} from "./npcs.js";

// base.js resolves to http://localhost:8010/api when window is undefined.
const BASE = "http://localhost:8010/api";
const TOKEN = "jwt-npc";

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

// Module flags are data, used by the panel banner.
{
  assert.equal(NPC_CRUD_AVAILABLE, true);
  assert.deepEqual(Object.keys(NPC_ENDPOINT_STATUS).sort(), [
    "createNpc",
    "createNpcTemplate",
    "deleteNpc",
    "deleteNpcTemplate",
    "getNpc",
    "getNpcTemplate",
    "listNpcTemplates",
    "listNpcTemplatesLegacy",
    "listNpcs",
    "spawnNpc",
    "updateNpc",
    "updateNpcTemplate",
  ]);
}

// GET /npc-templates?name=&role=&limit= (empty filters are dropped)
{
  mockFetch(() => jsonResponse(200, [{ id: "goblin_scout", name: "Goblin Scout", role: "enemy" }]));
  const rows = await listNpcTemplates(TOKEN, { name: "Goblin", role: "", limit: 50 });
  assert.equal(rows.length, 1);
  assert.equal(last().url, `${BASE}/npc-templates?name=Goblin&limit=50`);
  assert.equal(last().options.method, "GET");
  assertAuth(last());
  assert.equal(last().options.body, undefined);
}

// GET /npc-templates with no params
{
  mockFetch(() => jsonResponse(200, []));
  await listNpcTemplates(TOKEN);
  assert.equal(last().url, `${BASE}/npc-templates`);
  assert.equal(last().options.credentials, "same-origin");
}

// Role filter reaches the query string.
{
  mockFetch(() => jsonResponse(200, []));
  await listNpcTemplates(TOKEN, { role: "boss" });
  assert.equal(last().url, `${BASE}/npc-templates?role=boss`);
}

// GET /npc-templates/:id (id is URL-encoded)
{
  mockFetch(() => jsonResponse(200, { id: "a b", name: "Sparse", loot: [] }));
  const data = await getNpcTemplate(TOKEN, "a b");
  assert.equal(data.id, "a b");
  assert.equal(last().url, `${BASE}/npc-templates/a%20b`);
  assert.equal(last().options.method, "GET");
  assertAuth(last());
}

// POST /npc-templates sends the full template payload.
{
  const payload = {
    id: "goblin_scout",
    name: "Goblin Scout",
    description: "A wiry skirmisher.",
    role: "enemy",
    creatureType: "humanoid",
    combatStyle: "melee",
    difficulty: "easy",
    level: 3,
    maxHp: 24,
    damageMin: 2,
    damageMax: 5,
    aggressive: true,
    aggroRadius: 1,
    xpReward: 12,
    goldMin: 0,
    goldMax: 4,
    lootTableId: "",
    loot: [{ itemId: "goblin_ear", chance: 60, quantity: 1 }],
    spawnRoomId: "forest_edge",
    spawnCount: 1,
    respawnSeconds: 120,
    isTemplate: true,
    behavior: { stateMachine: [], aggro: { radius: 1, conditions: [] }, abilities: [] },
    scaling: { targetLevel: null },
  };
  mockFetch(() => jsonResponse(201, { id: "goblin_scout", ...payload }));
  const created = await createNpcTemplate(TOKEN, payload);
  assert.equal(created.id, "goblin_scout");
  assert.equal(last().url, `${BASE}/npc-templates`);
  assert.equal(last().options.method, "POST");
  assertAuth(last());
  assert.equal(last().options.headers["Content-Type"], "application/json");
  assert.deepEqual(JSON.parse(last().options.body), payload);
}

// PUT /npc-templates/:id
{
  const payload = { id: "goblin_scout", name: "Goblin Scout II", level: 4 };
  mockFetch(() => jsonResponse(200, { id: "goblin_scout", ...payload }));
  const updated = await updateNpcTemplate(TOKEN, "goblin_scout", payload);
  assert.equal(updated.id, "goblin_scout");
  assert.equal(last().url, `${BASE}/npc-templates/goblin_scout`);
  assert.equal(last().options.method, "PUT");
  assertAuth(last());
  assert.deepEqual(JSON.parse(last().options.body), payload);
}

// DELETE /npc-templates/:id (no body)
{
  mockFetch(() => jsonResponse(200, { status: "deleted" }));
  const res = await deleteNpcTemplate(TOKEN, "goblin_scout");
  assert.deepEqual(res, { status: "deleted" });
  assert.equal(last().url, `${BASE}/npc-templates/goblin_scout`);
  assert.equal(last().options.method, "DELETE");
  assertAuth(last());
  assert.equal(last().options.body, undefined);
}

// No token → no Authorization header (list may still be readable).
{
  mockFetch(() => jsonResponse(200, []));
  await listNpcTemplates("");
  assert.equal(last().options.headers.Authorization, undefined);
}

// Creator-gated write with a non-creator token: {error} message + .status,
// and it counts as a route/role error so the UI falls back to localStorage.
{
  mockFetch(() => jsonResponse(403, { error: "creator role required" }));
  let caught = null;
  try {
    await createNpcTemplate(TOKEN, { id: "x", name: "x" });
  } catch (err) {
    caught = err;
  }
  assert.notEqual(caught, null);
  assert.match(caught.message, /creator role required/);
  assert.equal(caught.status, 403);
  assert.equal(isNpcRouteError(caught), true);
}

// Route not implemented yet (404), unauthorised (401) and 501 degrade too.
{
  mockFetch(() => jsonResponse(404, null));
  await assert.rejects(listNpcTemplates(TOKEN), /Request failed \(404\)/);
  try {
    await listNpcTemplates(TOKEN);
  } catch (err) {
    assert.equal(err.status, 404);
    assert.equal(isNpcRouteError(err), true);
  }

  mockFetch(() => jsonResponse(401, { error: "invalid token" }));
  try {
    await deleteNpcTemplate(TOKEN, "goblin_scout");
  } catch (err) {
    assert.equal(err.status, 401);
    assert.equal(isNpcRouteError(err), true);
  }

  mockFetch(() => jsonResponse(501, null));
  try {
    await updateNpcTemplate(TOKEN, "goblin_scout", {});
    assert.fail("should reject");
  } catch (err) {
    assert.equal(err.status, 501);
    assert.equal(isNpcRouteError(err), true);
  }
}

// A plain server bug (500) is NOT a route error — the UI reports it but does
// not disable the server library.
{
  mockFetch(() => jsonResponse(500, { error: "boom" }));
  try {
    await updateNpcTemplate(TOKEN, "goblin_scout", { name: "x" });
    assert.fail("should reject");
  } catch (err) {
    assert.equal(err.status, 500);
    assert.match(err.message, /boom/);
    assert.equal(isNpcRouteError(err), false);
  }
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
  await assert.rejects(getNpcTemplate(TOKEN, "goblin_scout"), /Request failed \(502\)/);
}

// Network failure is reported consistently and counts as a route error.
{
  mockFetch(() => {
    throw new Error("connection refused");
  });
  try {
    await listNpcTemplates(TOKEN);
    assert.fail("should reject");
  } catch (err) {
    assert.match(err.message, /Network error/);
    assert.equal(err.code, "network");
    assert.equal(isNpcRouteError(err), true);
  }
}

// isNpcRouteError guards
{
  assert.equal(isNpcRouteError(null), false);
  assert.equal(isNpcRouteError(undefined), false);
  assert.equal(isNpcRouteError(new Error("plain")), false);
}

// ---------------------------------------------------------------------------
// Legacy /api/npcs instance routes (deprecated aliases keep their old paths)
// ---------------------------------------------------------------------------

{
  mockFetch(() => jsonResponse(200, []));
  await listNpcs(TOKEN, { roomID: "town_gate", isTemplate: true });
  assert.equal(last().url, `${BASE}/npcs?roomID=town_gate&isTemplate=true`);
  assert.equal(last().options.method, "GET");
  assertAuth(last());

  mockFetch(() => jsonResponse(200, []));
  await listNpcTemplatesLegacy(TOKEN);
  assert.equal(last().url, `${BASE}/npcs/templates`);

  mockFetch(() => jsonResponse(200, { id: "x" }));
  await getNpc(TOKEN, "x y");
  assert.equal(last().url, `${BASE}/npcs/x%20y`);
  assert.equal(last().options.method, "GET");

  mockFetch(() => jsonResponse(201, { id: "x" }));
  await createNpc(TOKEN, { id: "x" });
  assert.equal(last().url, `${BASE}/npcs`);
  assert.equal(last().options.method, "POST");
  assert.deepEqual(JSON.parse(last().options.body), { id: "x" });

  mockFetch(() => jsonResponse(200, { id: "x" }));
  await updateNpc(TOKEN, "x", { id: "x", level: 2 });
  assert.equal(last().url, `${BASE}/npcs/x`);
  assert.equal(last().options.method, "PUT");

  mockFetch(() => jsonResponse(200, { status: "deleted" }));
  await deleteNpc(TOKEN, "x");
  assert.equal(last().url, `${BASE}/npcs/x`);
  assert.equal(last().options.method, "DELETE");

  mockFetch(() => jsonResponse(200, { spawned: true }));
  await spawnNpc(TOKEN, "goblin_scout", { roomId: "forest_edge" });
  assert.equal(last().url, `${BASE}/npcs/goblin_scout/spawn`);
  assert.equal(last().options.method, "POST");
  assert.deepEqual(JSON.parse(last().options.body), { roomId: "forest_edge" });
}

console.log("npcs_test ok");
