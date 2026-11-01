import assert from "assert";
import { register, login, forgot, reset } from "./localAuth.js";

// base.js resolves to http://localhost:8010/api when window is undefined.
const BASE = "http://localhost:8010/api";

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

{
  mockFetch(() =>
    jsonResponse(201, { token: "jwt-abc", user: { id: "u1", username: "hero" } })
  );
  const data = await register({
    username: "hero",
    email: "hero@example.com",
    password: "supersecret",
  });
  assert.equal(data.token, "jwt-abc");
  assert.equal(data.user.username, "hero");
  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, `${BASE}/auth/register`);
  assert.equal(calls[0].options.method, "POST");
  assert.equal(calls[0].options.headers["Content-Type"], "application/json");
  assert.deepEqual(JSON.parse(calls[0].options.body), {
    username: "hero",
    email: "hero@example.com",
    password: "supersecret",
  });
}

{
  mockFetch(() =>
    jsonResponse(200, { token: "jwt-login", user: { id: "u2", username: "mage" } })
  );
  const data = await login({ username: "mage", password: "hunter2hunter2" });
  assert.equal(data.token, "jwt-login");
  assert.equal(calls[0].url, `${BASE}/auth/login`);
  assert.deepEqual(JSON.parse(calls[0].options.body), {
    username: "mage",
    password: "hunter2hunter2",
  });
}

{
  mockFetch(() => jsonResponse(409, { error: "account already exists" }));
  await assert.rejects(
    register({ username: "hero", email: "hero@example.com", password: "supersecret" }),
    /account already exists/
  );
}

{
  mockFetch(() => jsonResponse(401, { error: "invalid username or password" }));
  await assert.rejects(login({ username: "mage", password: "wrong" }), /invalid username or password/);
}

{
  mockFetch(() => jsonResponse(429, { error: "too many attempts" }));
  await assert.rejects(login({ username: "mage", password: "wrong" }), /too many attempts/);
}

{
  mockFetch(() => jsonResponse(500, { message: "boom" }));
  await assert.rejects(login({ username: "mage", password: "wrong" }), /boom/);
}

{
  // Non-JSON bodies fall back to a status-based message.
  mockFetch(() => ({
    ok: false,
    status: 502,
    json: async () => {
      throw new Error("not json");
    },
  }));
  await assert.rejects(login({ username: "mage", password: "wrong" }), /Request failed \(502\)/);
}

{
  mockFetch(() => {
    throw new Error("connection refused");
  });
  await assert.rejects(login({ username: "mage", password: "wrong" }), /Network error/);
}

{
  mockFetch(() => jsonResponse(200, { ok: true }));
  const data = await forgot({ email: "hero@example.com" });
  assert.deepEqual(data, { ok: true });
  assert.equal(calls[0].url, `${BASE}/auth/forgot`);
  assert.deepEqual(JSON.parse(calls[0].options.body), { email: "hero@example.com" });
}

{
  mockFetch(() => jsonResponse(200, { ok: true }));
  const data = await reset({ token: "reset-token", password: "brandnewpass" });
  assert.deepEqual(data, { ok: true });
  assert.equal(calls[0].url, `${BASE}/auth/reset`);
  assert.deepEqual(JSON.parse(calls[0].options.body), {
    token: "reset-token",
    password: "brandnewpass",
  });
}

console.log("localAuth_test ok");
