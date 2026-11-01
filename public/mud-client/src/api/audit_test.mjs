import assert from 'assert';
import {
  AUDIT_ENDPOINT_STATUS,
  AUDIT_QUERY_SUPPORT,
  AUDIT_MAX_LIMIT,
  listAudit,
  withQuery,
} from './audit.js';

// base.js resolves to http://localhost:8010/api when window is undefined.
const BASE = 'http://localhost:8010/api';
const TOKEN = 'jwt-creator';

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

// Status flags encode what the backend does and does not serve.
assert.equal(AUDIT_ENDPOINT_STATUS.listAudit, true);
assert.equal(AUDIT_ENDPOINT_STATUS.exportAudit, false);
assert.equal(AUDIT_QUERY_SUPPORT.entityType, true);
assert.equal(AUDIT_QUERY_SUPPORT.limit, true);
assert.equal(AUDIT_QUERY_SUPPORT.action, false);
assert.equal(AUDIT_QUERY_SUPPORT.actor, false);
assert.equal(AUDIT_MAX_LIMIT, 500);

// withQuery drops empty values.
assert.equal(withQuery('/audit', {}), '/audit');
assert.equal(withQuery('/audit', { entityType: '', limit: 100, entityId: undefined }), '/audit?limit=100');

// GET /api/audit with a creator token forwards only supported params.
{
  const rows = [{ id: 'a1', action: 'create' }];
  mockFetch(() => jsonResponse(200, rows));
  const data = await listAudit(TOKEN, {
    entityType: 'rooms',
    entityId: 'room-1',
    limit: 500,
    // These are client-side only and must NOT be forwarded.
    action: 'create',
    actor: 'Alice',
    q: 'sword',
  });
  assert.deepEqual(data, rows);
  const url = new URL(last().url);
  assert.equal(url.origin + url.pathname, `${BASE}/audit`);
  assert.equal(url.searchParams.get('entityType'), 'rooms');
  assert.equal(url.searchParams.get('entityId'), 'room-1');
  assert.equal(url.searchParams.get('limit'), '500');
  assert.equal(url.searchParams.has('action'), false);
  assert.equal(url.searchParams.has('actor'), false);
  assert.equal(url.searchParams.has('q'), false);
  assert.equal(last().options.method, 'GET');
  assert.equal(last().options.headers.Authorization, `Bearer ${TOKEN}`);
}

// A null/non-array body normalizes to an empty array.
{
  mockFetch(() => jsonResponse(200, null));
  assert.deepEqual(await listAudit(TOKEN), []);
}

// HTTP errors surface the backend message.
{
  mockFetch(() => jsonResponse(403, { error: 'creator role required' }));
  await assert.rejects(() => listAudit(TOKEN), /creator role required/);
}

// Network failures produce a friendly message.
{
  calls = [];
  globalThis.fetch = async () => {
    throw new Error('boom');
  };
  await assert.rejects(() => listAudit(TOKEN), /Network error/);
}

console.log('audit_test ok');
