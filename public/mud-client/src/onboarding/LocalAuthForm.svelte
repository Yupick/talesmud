<style>
  .local-auth {
    width: 100%;
    max-width: 320px;
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
  }

  .tabs {
    display: flex;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 6px;
    overflow: hidden;
  }

  .tab {
    flex: 1;
    padding: 0.55rem 0.5rem;
    font-size: 0.8rem;
    font-weight: 500;
    letter-spacing: 0.03em;
    background: transparent;
    color: #9ca3af;
    border: none;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .tab:hover {
    color: #e5e7eb;
    background: rgba(255, 255, 255, 0.04);
  }

  .tab.active {
    background: rgba(22, 163, 74, 0.18);
    color: #e5e7eb;
  }

  .fields {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
    text-align: left;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .field-label {
    font-size: 0.7rem;
    font-weight: 500;
    color: #6b7280;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .field-input {
    width: 100%;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.12);
    outline: none;
    box-shadow: none;
    color: #e5e7eb;
    font-size: 0.95rem;
    padding: 0.6rem 0.75rem;
    border-radius: 6px;
    box-sizing: border-box;
    transition: border-color 0.2s ease;
  }

  .field-input:focus {
    border-color: rgba(255, 255, 255, 0.25);
  }

  .field-input::placeholder {
    color: #4b5563;
  }

  .field-hint {
    font-size: 0.68rem;
    color: #6b7280;
    line-height: 1.3;
  }

  .submit {
    font-size: 0.85rem;
    font-weight: 500;
    letter-spacing: 0.03em;
    padding: 0.7rem 1.5rem;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    color: #fff;
    background: #16a34a;
    transition: all 0.2s ease;
    width: 100%;
    box-sizing: border-box;
  }

  .submit:hover:not(:disabled) {
    background: #15803d;
    box-shadow: 0 4px 12px rgba(22, 163, 74, 0.3);
    transform: translateY(-1px);
  }

  .submit:disabled {
    opacity: 0.4;
    cursor: not-allowed;
    transform: none;
  }

  .link-row {
    text-align: center;
  }

  .link {
    background: none;
    border: none;
    padding: 0;
    font-size: 0.75rem;
    color: #9ca3af;
    cursor: pointer;
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .link:hover {
    color: #e5e7eb;
  }

  .notice {
    font-size: 0.78rem;
    color: #86efac;
    background: rgba(134, 239, 172, 0.08);
    border: 1px solid rgba(134, 239, 172, 0.2);
    padding: 0.5rem 0.9rem;
    border-radius: 6px;
    text-align: center;
  }

  .error-banner {
    font-size: 0.78rem;
    color: #f87171;
    background: rgba(248, 113, 113, 0.1);
    border: 1px solid rgba(248, 113, 113, 0.2);
    padding: 0.5rem 0.9rem;
    border-radius: 6px;
    text-align: center;
  }
</style>

<script>
  import { getAuth } from "../auth.js";
  import { register, login, forgot, reset } from "../api/localAuth.js";
  import { clearGuestToken, writeLocalToken } from "../authSession.js";

  // Context must be read during component initialization.
  const auth = getAuth();

  function resetTokenFromUrl() {
    if (typeof window === "undefined") return "";
    const params = new URLSearchParams(window.location.search);
    return params.get("reset") || params.get("reset_token") || "";
  }

  const initialResetToken = resetTokenFromUrl();

  let tab = initialResetToken ? "reset" : "login"; // login | register | forgot | reset
  let username = "";
  let email = "";
  let password = "";
  let loading = false;
  let error = "";
  let notice = initialResetToken
    ? "Choose a new password for your account."
    : "";

  $: valid =
    tab === "login"
      ? username.trim().length > 0 && password.length > 0
      : tab === "register"
        ? username.trim().length > 0 && email.trim().length > 0 && password.length > 0
        : tab === "forgot"
          ? email.trim().length > 0
          : password.length > 0; // reset

  function selectTab(next) {
    if (loading) return;
    tab = next;
    error = "";
    notice = "";
    password = "";
  }

  function applySession(data) {
    if (!data || !data.token) {
      throw new Error("The server did not return a session. Please try again.");
    }
    // A local account replaces any guest session in this tab.
    clearGuestToken();
    writeLocalToken(data.token);

    if (auth) {
      auth.authToken.set(data.token);
      auth.userInfo.set(data.user || {});
      auth.isAuthenticated.set(true);
      if (auth.authError) auth.authError.set(null);
    }
  }

  async function submit() {
    if (loading || !valid) return;
    loading = true;
    error = "";
    notice = "";

    try {
      if (tab === "login") {
        const data = await login({ username: username.trim(), password });
        applySession(data);
      } else if (tab === "register") {
        const data = await register({
          username: username.trim(),
          email: email.trim(),
          password,
        });
        applySession(data);
      } else if (tab === "forgot") {
        await forgot({ email: email.trim() });
        notice = "If that email has an account, a reset link is on its way.";
      } else {
        await reset({ token: initialResetToken, password });
        tab = "login";
        notice = "Password updated. Sign in with your new password.";
      }
      password = "";
    } catch (err) {
      error = err && err.message ? err.message : "Something went wrong. Please try again.";
    } finally {
      loading = false;
    }
  }
</script>

<div class="local-auth">
  <div class="tabs" role="tablist">
    <button
      type="button"
      role="tab"
      class="tab"
      class:active={tab === "login"}
      aria-selected={tab === "login"}
      on:click={() => selectTab("login")}
    >
      Sign in
    </button>
    <button
      type="button"
      role="tab"
      class="tab"
      class:active={tab === "register"}
      aria-selected={tab === "register"}
      on:click={() => selectTab("register")}
    >
      Register
    </button>
  </div>

  {#if notice}
    <div class="notice" role="status">{notice}</div>
  {/if}

  {#if error}
    <div class="error-banner" role="alert">{error}</div>
  {/if}

  <form class="fields" on:submit|preventDefault={submit}>
    {#if tab === "login"}
      <div class="field">
        <label class="field-label" for="local-login-username">Username</label>
        <input
          id="local-login-username"
          class="field-input"
          type="text"
          name="username"
          autocomplete="username"
          autocapitalize="none"
          spellcheck="false"
          bind:value={username}
          disabled={loading}
          required
        />
      </div>
      <div class="field">
        <label class="field-label" for="local-login-password">Password</label>
        <input
          id="local-login-password"
          class="field-input"
          type="password"
          name="password"
          autocomplete="current-password"
          bind:value={password}
          disabled={loading}
          required
        />
      </div>
    {:else if tab === "register"}
      <div class="field">
        <label class="field-label" for="local-register-username">Username</label>
        <input
          id="local-register-username"
          class="field-input"
          type="text"
          name="username"
          autocomplete="username"
          autocapitalize="none"
          spellcheck="false"
          bind:value={username}
          disabled={loading}
          minlength="3"
          maxlength="20"
          pattern="[a-z0-9_]{3,20}"
          required
        />
        <span class="field-hint">3-20 characters: a-z, 0-9, underscore.</span>
      </div>
      <div class="field">
        <label class="field-label" for="local-register-email">Email</label>
        <input
          id="local-register-email"
          class="field-input"
          type="email"
          name="email"
          autocomplete="email"
          bind:value={email}
          disabled={loading}
          required
        />
      </div>
      <div class="field">
        <label class="field-label" for="local-register-password">Password</label>
        <input
          id="local-register-password"
          class="field-input"
          type="password"
          name="password"
          autocomplete="new-password"
          bind:value={password}
          disabled={loading}
          minlength="8"
          maxlength="128"
          required
        />
        <span class="field-hint">8-128 characters.</span>
      </div>
    {:else if tab === "forgot"}
      <div class="field">
        <label class="field-label" for="local-forgot-email">Email</label>
        <input
          id="local-forgot-email"
          class="field-input"
          type="email"
          name="email"
          autocomplete="email"
          bind:value={email}
          disabled={loading}
          required
        />
      </div>
    {:else}
      <div class="field">
        <label class="field-label" for="local-reset-password">New password</label>
        <input
          id="local-reset-password"
          class="field-input"
          type="password"
          name="password"
          autocomplete="new-password"
          bind:value={password}
          disabled={loading}
          minlength="8"
          maxlength="128"
          required
        />
        <span class="field-hint">8-128 characters.</span>
      </div>
    {/if}

    <button class="submit" type="submit" disabled={loading || !valid}>
      {#if loading}
        Please wait...
      {:else if tab === "login"}
        Sign in
      {:else if tab === "register"}
        Create account
      {:else if tab === "forgot"}
        Send reset link
      {:else}
        Update password
      {/if}
    </button>
  </form>

  <div class="link-row">
    {#if tab === "forgot"}
      <button class="link" type="button" on:click={() => selectTab("login")}>
        Back to sign in
      </button>
    {:else if tab !== "reset"}
      <button class="link" type="button" on:click={() => selectTab("forgot")}>
        Forgot your password?
      </button>
    {/if}
  </div>
</div>
