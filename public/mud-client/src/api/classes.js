// Class and skill definition REST helpers (Creator Phase 5.2).
//
// Backend status — read this before wiring the editor to the network:
//
//   EXISTING
//     GET    /api/classes          public catalog, `handler.GetClasses`
//                                   (balance numbers are deliberately stripped)
//     GET    /api/skills           creator-gated, `skillsHandler.GetSkills`
//     GET    /api/skills/:id       creator-gated
//     POST   /api/skills           creator-gated
//     PUT    /api/skills/:id       creator-gated
//     DELETE /api/skills/:id       creator-gated
//
//   PENDING (design contract, NOT served yet — see the Phase 5.2 report)
//     GET    /api/classes/:id      full class including balance + skills
//     POST   /api/classes          create a class pack file
//     PUT    /api/classes/:id      update a class pack file
//     DELETE /api/classes/:id      delete a class pack file
//     POST   /api/classes/:id/duplicate
//
// Because the class CRUD routes do not exist, `ClassesEditor.svelte` persists
// edits to localStorage via `src/game/classEditor.js`. Flip
// `CLASS_DEF_CRUD_AVAILABLE` to true once the backend routes land and the
// editor will prefer the network. The request functions below already encode
// the intended contract so the switch is a one-line change.
import { backend } from "./base.js";

/** True once the backend serves POST/PUT/DELETE /api/classes. */
export const CLASS_DEF_CRUD_AVAILABLE = false;

/**
 * Human-readable status of every endpoint this module talks to. Kept as data
 * so the report and the UI can render which actions are backed by the server.
 */
export const CLASS_ENDPOINT_STATUS = {
  listClasses: true,
  getClass: false,
  createClass: false,
  updateClass: false,
  deleteClass: false,
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
 * Public class catalog (GET /api/classes — exists).
 * @returns {Promise<{source:string, classes:Array<object>, races:Array<object>}>}
 */
function fetchClasses() {
  return request(null, "GET", "/classes");
}

// ---------------------------------------------------------------------------
// Class definitions
// ---------------------------------------------------------------------------

/**
 * List full class definitions for the Creator (GET /api/classes — exists, but
 * the public payload omits balance/skills until the pending full route lands).
 * @param {string} token session JWT
 */
function listClassDefs(token) {
  return request(token, "GET", "/classes");
}

/**
 * Full class definition including balance and skills.
 * PENDING backend: GET /api/classes/:id
 * @param {string} token
 * @param {string} id
 */
function getClassDef(token, id) {
  return request(token, "GET", `/classes/${encodeURIComponent(id)}`);
}

/**
 * Create a class. PENDING backend: POST /api/classes
 * @param {string} token
 * @param {object} classDef
 */
function createClassDef(token, classDef) {
  return request(token, "POST", "/classes", classDef);
}

/**
 * Update a class. PENDING backend: PUT /api/classes/:id
 * @param {string} token
 * @param {string} id
 * @param {object} classDef
 */
function updateClassDef(token, id, classDef) {
  return request(token, "PUT", `/classes/${encodeURIComponent(id)}`, classDef);
}

/**
 * Delete a class. PENDING backend: DELETE /api/classes/:id
 * @param {string} token
 * @param {string} id
 */
function deleteClassDef(token, id) {
  return request(token, "DELETE", `/classes/${encodeURIComponent(id)}`);
}

/**
 * Duplicate a class. PENDING backend: POST /api/classes/:id/duplicate
 * (the editor currently duplicates in local state instead).
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
