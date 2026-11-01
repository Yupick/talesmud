import assert from "node:assert/strict";
import {
  listDungeons,
  getDungeon,
  createDungeon,
  updateDungeon,
  deleteDungeon,
  isDungeonRouteError,
  DUNGEON_CRUD_AVAILABLE,
  DUNGEON_ENDPOINT_STATUS,
} from "./dungeons.js";

// base.js resolves to http://localhost:8010/api when window is undefined.
const BASE = "http://localhost:8010/api";
const TOKEN = "jwt-dungeon";

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
  assert.equal(DUNGEON_CRUD_AVAILABLE, true);
  assert.deepEqual(Object.keys(DUNGEON_ENDPOINT_STATUS).sort(), [
    "createDungeon",
    "deleteDungeon",
    "getDungeon",
    "listDungeons",
    "updateDungeon",
  ]);
}

// GET /dungeons?name=&author=&limit= (empty filters are dropped)
{
  mockFetch(() => jsonResponse(200, [{ id: "d1", name: "Crypt" }]));
  const rows = await listDungeons(TOKEN, { name: "Crypt", author: "", limit: 25 });
  assert.equal(rows.length, 1);
  assert.equal(last().url, `${BASE}/dungeons?name=Crypt&limit=25`);
  assert.equal(last().options.method, "GET");
  assertAuth(last());
  assert.equal(last().options.body, undefined);
}

// GET /dungeons with no params
{
  mockFetch(() => jsonResponse(200, []));
  await listDungeons(TOKEN);
  assert.equal(last().url, `${BASE}/dungeons`);
}

// GET /dungeons/:id (id is URL-encoded)
{
  mockFetch(() => jsonResponse(200, { id: "a b", name: "Crypt", definition: { version: 1 } }));
  const data = await getDungeon(TOKEN, "a b");
  assert.equal(data.id, "a b");
  assert.equal(last().url, `${BASE}/dungeons/a%20b`);
  assert.equal(last().options.method, "GET");
  assertAuth(last());
}

// POST /dungeons sends the payload built from exportDefinition
{
  const payload = {
    name: "Crypt",
    description: "demo",
    gridWidth: 10,
    gridHeight: 8,
    definition: { version: 1, grid: { width: 10, height: 8 }, cells: [] },
    tags: ["small"],
  };
  mockFetch(() => jsonResponse(201, { id: "srv1", ...payload }));
  const created = await createDungeon(TOKEN, payload);
  assert.equal(created.id, "srv1");
  assert.equal(last().url, `${BASE}/dungeons`);
  assert.equal(last().options.method, "POST");
  assertAuth(last());
  assert.equal(last().options.headers["Content-Type"], "application/json");
  assert.deepEqual(JSON.parse(last().options.body), payload);
}

// PUT /dungeons/:id
{
  const payload = { name: "Crypt II", gridWidth: 12, gridHeight: 12, definition: { version: 1 } };
  mockFetch(() => jsonResponse(200, { id: "srv1", ...payload }));
  const updated = await updateDungeon(TOKEN, "srv1", payload);
  assert.equal(updated.id, "srv1");
  assert.equal(last().url, `${BASE}/dungeons/srv1`);
  assert.equal(last().options.method, "PUT");
  assertAuth(last());
  assert.deepEqual(JSON.parse(last().options.body), payload);
}

// DELETE /dungeons/:id
{
  mockFetch(() => jsonResponse(200, { status: "deleted" }));
  const res = await deleteDungeon(TOKEN, "srv1");
  assert.deepEqual(res, { status: "deleted" });
  assert.equal(last().url, `${BASE}/dungeons/srv1`);
  assert.equal(last().options.method, "DELETE");
  assertAuth(last());
  assert.equal(last().options.body, undefined);
}

// No token → no Authorization header (public list still possible).
{
  mockFetch(() => jsonResponse(200, []));
  await listDungeons("");
  assert.equal(last().options.headers.Authorization, undefined);
}

// Creator-gated write with a non-creator token: {error} message + .status,
// and it counts as a route/role error so the UI falls back to localStorage.
{
  mockFetch(() => jsonResponse(403, { error: "creator role required" }));
  let caught = null;
  try {
    await createDungeon(TOKEN, { name: "x", gridWidth: 1, gridHeight: 1, definition: {} });
  } catch (err) {
    caught = err;
  }
  assert.notEqual(caught, null);
  assert.match(caught.message, /creator role required/);
  assert.equal(caught.status, 403);
  assert.equal(isDungeonRouteError(caught), true);
}

// Route not implemented yet (404) and unauthorised (401) degrade too.
{
  mockFetch(() => jsonResponse(404, null));
  await assert.rejects(listDungeons(TOKEN), /Request failed \(404\)/);
  try {
    await listDungeons(TOKEN);
  } catch (err) {
    assert.equal(err.status, 404);
    assert.equal(isDungeonRouteError(err), true);
  }

  mockFetch(() => jsonResponse(401, { error: "invalid token" }));
  try {
    await deleteDungeon(TOKEN, "srv1");
  } catch (err) {
    assert.equal(err.status, 401);
    assert.equal(isDungeonRouteError(err), true);
  }
}

// A plain server bug (500) is NOT a route error — the UI reports it but does
// not disable the server library.
{
  mockFetch(() => jsonResponse(500, { error: "boom" }));
  try {
    await updateDungeon(TOKEN, "srv1", { name: "x" });
    assert.fail("should reject");
  } catch (err) {
    assert.equal(err.status, 500);
    assert.match(err.message, /boom/);
    assert.equal(isDungeonRouteError(err), false);
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
  await assert.rejects(getDungeon(TOKEN, "srv1"), /Request failed \(502\)/);
}

// Network failure is reported consistently and counts as a route error.
{
  mockFetch(() => {
    throw new Error("connection refused");
  });
  try {
    await listDungeons(TOKEN);
    assert.fail("should reject");
  } catch (err) {
    assert.match(err.message, /Network error/);
    assert.equal(err.code, "network");
    assert.equal(isDungeonRouteError(err), true);
  }
}

// isDungeonRouteError guards
{
  assert.equal(isDungeonRouteError(null), false);
  assert.equal(isDungeonRouteError(undefined), false);
  assert.equal(isDungeonRouteError(new Error("plain")), false);
}

console.log("dungeons_test ok");
