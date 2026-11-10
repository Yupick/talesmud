// Class and skill definition REST helpers (Creator Phase 5.2).
//
// Backend status — all class-definition routes are now served:
//
//   PUBLIC
//     GET    /api/classes          catalog for anonymous callers; a creator
//                                   token on the same path returns the full
//                                   editor definitions (balance + template).
//   CREATOR
//     GET    /api/classes/:id      full class including balance + skills
//     POST   /api/classes          create a class pack file
//     PUT    /api/classes/:id      update a class pack file
//     DELETE /api/classes/:id      delete a class pack file
//     POST   /api/classes/:id/duplicate
//     GET    /api/skills           also aliased as /api/spells
//     GET    /api/skills/:id       also aliased as /api/spells/:id
//     POST   /api/skills           also aliased as /api/spells
//     PUT    /api/skills/:id       also aliased as /api/spells/:id
//     DELETE /api/skills/:id       also aliased as /api/spells/:id
//
// `ClassesEditor.svelte` prefers these routes and falls back to localStorage
// only when the network itself is unavailable.
import { backend } from "./base.js";

/** True once the backend serves POST/PUT/DELETE /api/classes. */
export const CLASS_DEF_CRUD_AVAILABLE = true;

/**
 * Human-readable status of every endpoint this module talks to. Kept as data
 * so the report and the UI can render which actions are backed by the server.
 */
export const CLASS_ENDPOINT_STATUS = {
  listClasses: true,
  getClass: true,
  createClass: true,
  updateClass: true,
  deleteClass: true,
  listSkills: true,
  createSkill: true,
  updateSkill: true,
  deleteSkill: true,
};

async function request(token, method, path, body) {
  const headers = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  const options = { method, credentials: "same-origin", headers };
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
      (data && (data.error || data.message)) || `Request failed (${response.status})`;
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
 * Public class catalog (GET /api/classes).
 * With no token the server returns the anonymous catalog; with a creator token
 * the same path returns the full editor definitions.
 * @returns {Promise<{source:string, classes:Array<object>, races:Array<object>}>}
 */
function fetchClasses() {
  return request(null, "GET", "/classes");
}

// ---------------------------------------------------------------------------
// Class definitions
// ---------------------------------------------------------------------------

/**
 * List full class definitions for the Creator (GET /api/classes with token).
 * @param {string} token session JWT
 */
function listClassDefs(token) {
  return request(token, "GET", "/classes");
}

/**
 * Full class definition including balance and skills.
 * @param {string} token
 * @param {string} id
 */
function getClassDef(token, id) {
  return request(token, "GET", `/classes/${encodeURIComponent(id)}`);
}

/**
 * Create a class.
 * @param {string} token
 * @param {object} classDef
 */
function createClassDef(token, classDef) {
  return request(token, "POST", "/classes", classDef);
}

/**
 * Update a class.
 * @param {string} token
 * @param {string} id
 * @param {object} classDef
 */
function updateClassDef(token, id, classDef) {
  return request(token, "PUT", `/classes/${encodeURIComponent(id)}`, classDef);
}

/**
 * Delete a class.
 * @param {string} token
 * @param {string} id
 */
function deleteClassDef(token, id) {
  return request(token, "DELETE", `/classes/${encodeURIComponent(id)}`);
}

/**
 * Duplicate a class. The body is optional; the server generates
 * `<id>_copy` when it is empty.
 * @param {string} token
 * @param {string} id
 */
function duplicateClassDef(token, id) {
  return request(token, "POST", `/classes/${encodeURIComponent(id)}/duplicate`, {});
}

// ---------------------------------------------------------------------------
// Skill definitions (generic skills service — routes exist)
// ---------------------------------------------------------------------------

/** GET /api/skills */
function listSkillDefs(token) {
  return request(token, "GET", "/skills");
}

/** GET /api/skills/:id */
function getSkillDef(token, id) {
  return request(token, "GET", `/skills/${encodeURIComponent(id)}`);
}

/** POST /api/skills */
function createSkillDef(token, skill) {
  return request(token, "POST", "/skills", skill);
}

/** PUT /api/skills/:id */
function updateSkillDef(token, id, skill) {
  return request(token, "PUT", `/skills/${encodeURIComponent(id)}`, skill);
}

/** DELETE /api/skills/:id */
function deleteSkillDef(token, id) {
  return request(token, "DELETE", `/skills/${encodeURIComponent(id)}`);
}

export {
  fetchClasses,
  withQuery,
  listClassDefs,
  getClassDef,
  createClassDef,
  updateClassDef,
  deleteClassDef,
  duplicateClassDef,
  listSkillDefs,
  getSkillDef,
  createSkillDef,
  updateSkillDef,
  deleteSkillDef,
};
