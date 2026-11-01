// Local username/password auth. These endpoints are public (no Auth0) and
// return a session JWT that later travels as `Authorization: Bearer`.
import { backend } from "./base.js";

// POST a JSON body and resolve with the parsed response. Backend errors are
// delivered as `{error: "..."}`; turn them into readable Error messages.
async function post(path, body) {
  let response;
  try {
    response = await fetch(`${backend}${path}`, {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
    });
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

// register resolves with {token, user}.
function register({ username, email, password }) {
  return post("/auth/register", { username, email, password });
}

// login resolves with {token, user}.
function login({ username, password }) {
  return post("/auth/login", { username, password });
}

// forgot resolves with {ok: true}. Unknown emails answer the same way.
function forgot({ email }) {
  return post("/auth/forgot", { email });
}

// reset resolves with {ok: true}.
function reset({ token, password }) {
  return post("/auth/reset", { token, password });
}

export { register, login, forgot, reset };
