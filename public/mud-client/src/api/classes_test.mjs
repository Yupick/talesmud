import assert from 'assert';
import {
  CLASS_DEF_CRUD_AVAILABLE,
  CLASS_ENDPOINT_STATUS,
  fetchClasses,
  listClassDefs,
  getClassDef,
  createClassDef,
  updateClassDef,
  deleteClassDef,
  duplicateClassDef,
  listSkillDefs,
  createSkillDef,
  updateSkillDef,
  deleteSkillDef,
} from './classes.js';

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

function assertAuth(call) {
  assert.equal(call.options.headers.Authorization, `Bearer ${TOKEN}`);
}

// Public catalog: no Authorization header.
{
  mockFetch(() => jsonResponse(200, { source: 'sample', classes: [], races: [] }));
  const data = await fetchClasses();
  assert.deepEqual(data, { source: 'sample', classes: [], races: [] });
  assert.equal(last().url, `${BASE}/classes`);
  assert.equal(last().options.method, 'GET');
  assert.equal(last().options.headers.Authorization, undefined);
}

// GET /classes with a creator token.
{
  mockFetch(() => jsonResponse(200, { classes: [{ id: 'mage' }] }));
  const data = await listClassDefs(TOKEN);
  assert.equal(data.classes[0].id, 'mage');
  assert.equal(last().url, `${BASE}/classes`);
  assertAuth(last());
}

// Creator class CRUD routes build the served contract.
{
  mockFetch(() => jsonResponse(200, { id: 'mage' }));
  const created = await createClassDef(TOKEN, { id: 'mage', name: 'Mage' });
  assert.equal(created.id, 'mage');
  assert.equal(last().url, `${BASE}/classes`);
  assert.equal(last().options.method, 'POST');
  assertAuth(last());
  assert.deepEqual(JSON.parse(last().options.body), { id: 'mage', name: 'Mage' });

  mockFetch(() => jsonResponse(200, { id: 'mage', name: 'Mage II' }));
  await updateClassDef(TOKEN, 'mage', { id: 'mage', name: 'Mage II' });
  assert.equal(last().url, `${BASE}/classes/mage`);
  assert.equal(last().options.method, 'PUT');

  mockFetch(() => jsonResponse(200, { status: 'ok' }));
  await deleteClassDef(TOKEN, 'mage');
  assert.equal(last().url, `${BASE}/classes/mage`);
  assert.equal(last().options.method, 'DELETE');
  assertAuth(last());

  mockFetch(() => jsonResponse(200, { id: 'mage_2' }));
  await duplicateClassDef(TOKEN, 'mage');
  assert.equal(last().url, `${BASE}/classes/mage/duplicate`);
  assert.equal(last().options.method, 'POST');

  mockFetch(() => jsonResponse(200, { id: 'mage' }));
  await getClassDef(TOKEN, 'mage');
  assert.equal(last().url, `${BASE}/classes/mage`);
  assert.equal(last().options.method, 'GET');
}

// Skill routes (creator-gated, already served).
{
  mockFetch(() => jsonResponse(200, [{ id: 's1' }]));
  const skills = await listSkillDefs(TOKEN);
  assert.equal(skills[0].id, 's1');
  assert.equal(last().url, `${BASE}/skills`);
  assertAuth(last());

  mockFetch(() => jsonResponse(200, { id: 's1' }));
  await createSkillDef(TOKEN, { id: 's1', name: 'Sear' });
  assert.equal(last().options.method, 'POST');
  assert.deepEqual(JSON.parse(last().options.body), { id: 's1', name: 'Sear' });

  mockFetch(() => jsonResponse(200, { id: 's1', name: 'Sear II' }));
  await updateSkillDef(TOKEN, 's1', { id: 's1', name: 'Sear II' });
  assert.equal(last().url, `${BASE}/skills/s1`);
  assert.equal(last().options.method, 'PUT');

  mockFetch(() => jsonResponse(200, { status: 'deleted' }));
  await deleteSkillDef(TOKEN, 's1');
  assert.equal(last().url, `${BASE}/skills/s1`);
  assert.equal(last().options.method, 'DELETE');
}

// The capability flags describe the served backend.
{
  assert.equal(CLASS_DEF_CRUD_AVAILABLE, true);
  assert.equal(CLASS_ENDPOINT_STATUS.createClass, true);
  assert.equal(CLASS_ENDPOINT_STATUS.getClass, true);
  assert.equal(CLASS_ENDPOINT_STATUS.updateClass, true);
  assert.equal(CLASS_ENDPOINT_STATUS.deleteClass, true);
  assert.equal(CLASS_ENDPOINT_STATUS.listSkills, true);
}

// Backend error surfaces the {error} message.
{
  mockFetch(() => jsonResponse(403, { error: 'creator role required' }));
  await assert.rejects(createClassDef(TOKEN, { id: 'x' }), /creator role required/);
}

// Non-JSON error falls back to a status message.
{
  mockFetch(() => ({
    ok: false,
    status: 502,
    json: async () => {
      throw new Error('not json');
    },
  }));
  await assert.rejects(listClassDefs(TOKEN), /Request failed \(502\)/);
}

// Network failure is reported consistently.
{
  mockFetch(() => {
    throw new Error('connection refused');
  });
  await assert.rejects(fetchClasses(), /Network error/);
}

console.log('classes_test ok');
