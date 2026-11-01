package main

// auth.go implements the real TalesMUD connection handshake:
//
//	guest/register -> session JWT -> POST /api/ws-ticket -> GET /ws?ticket=...
//
// -tokens-file loads one pre-minted JWT per line and round-robins them across
// workers, which is the only way to run more concurrent sessions than the
// server's per-IP anti-abuse budgets allow (guest cap, register limiter).
//
// WSAuthMiddleware (pkg/server/auth.go) accepts a single-use ticket in the
// query string or an Authorization: Bearer token. A session JWT in ?access_token
// is explicitly rejected, so the harness never uses that.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type authMode string

const (
	authGuest    authMode = "guest"
	authRegister authMode = "register"
	authToken    authMode = "token"
	authTicket   authMode = "ticket"
)

type authConfig struct {
	mode      authMode
	httpBase  string
	client    *http.Client
	origin    string
	token     string // session JWT for -auth token
	tokens    []string // one JWT per worker, round-robin (-tokens-file)
	ticket    string // pre-issued ticket for -auth ticket
	useBearer bool   // skip /api/ws-ticket and dial with the JWT header
}

// tokenFor picks the session JWT for one worker. A tokens list round-robins so
// every worker gets its own identity; otherwise the single -token is shared.
func (a *authConfig) tokenFor(index int) string {
	if len(a.tokens) > 0 {
		return a.tokens[index%len(a.tokens)]
	}
	return a.token
}

// LoadTokensFile reads one session JWT per non-empty, non-# line.
func LoadTokensFile(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no tokens found", path)
	}
	return out, nil
}

type wsCredentials struct {
	token string
}

func newAuthConfig(mode, rawURL, origin, token, ticket string, useBearer bool, timeout time.Duration, tokens []string) (*authConfig, error) {
	base, err := httpBaseFromWS(rawURL)
	if err != nil {
		return nil, err
	}
	return &authConfig{
		mode:      authMode(mode),
		httpBase:  base,
		client:    &http.Client{Timeout: timeout},
		origin:    origin,
		token:     token,
		tokens:    tokens,
		ticket:    ticket,
		useBearer: useBearer,
	}, nil
}

// target returns the URL the worker should dial and the headers to send.
func (a *authConfig) target(index int, wsURL string) (string, http.Header, error) {
	h := http.Header{}
	if a.origin != "" {
		h.Set("Origin", a.origin)
	}

	switch a.mode {
	case authTicket:
		if a.ticket == "" {
			return "", nil, fmt.Errorf("auth=ticket requires -ticket")
		}
		return withTicket(wsURL, a.ticket), h, nil

	case authToken:
		token := a.tokenFor(index)
		if token == "" {
			return "", nil, fmt.Errorf("auth=token requires -token or -tokens-file")
		}
		if a.useBearer {
			h.Set("Authorization", "Bearer "+token)
			return wsURL, h, nil
		}
		ticket, err := a.issueTicket(token)
		if err != nil {
			return "", nil, err
		}
		return withTicket(wsURL, ticket), h, nil

	case authGuest, authRegister:
		token, err := a.createUser(index)
		if err != nil {
			return "", nil, err
		}
		if a.useBearer {
			h.Set("Authorization", "Bearer "+token)
			return wsURL, h, nil
		}
		ticket, err := a.issueTicket(token)
		if err != nil {
			return "", nil, err
		}
		return withTicket(wsURL, ticket), h, nil
	}
	return "", nil, fmt.Errorf("unknown auth mode %q", a.mode)
}

func withTicket(rawURL, ticket string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL + "?ticket=" + url.QueryEscape(ticket)
	}
	q := u.Query()
	q.Set("ticket", ticket)
	u.RawQuery = q.Encode()
	return u.String()
}

func (a *authConfig) createUser(index int) (string, error) {
	switch a.mode {
	case authGuest:
		var out struct {
			Token string `json:"token"`
		}
		if err := a.postJSON("/api/guest", nil, "", &out); err != nil {
			return "", err
		}
		if out.Token == "" {
			return "", fmt.Errorf("guest: empty token")
		}
		return out.Token, nil
	case authRegister:
		body := map[string]string{
			"username": fmt.Sprintf("sim_%d_%d", index, time.Now().UnixNano()),
			"email":    fmt.Sprintf("sim_%d_%d@example.test", index, time.Now().UnixNano()),
			"password": "SimPassword123!",
		}
		var out struct {
			Token string `json:"token"`
		}
		if err := a.postJSON("/api/auth/register", body, "", &out); err != nil {
			return "", err
		}
		if out.Token == "" {
			return "", fmt.Errorf("register: empty token")
		}
		return out.Token, nil
	}
	return "", fmt.Errorf("createUser unsupported for %q", a.mode)
}

func (a *authConfig) issueTicket(token string) (string, error) {
	var out struct {
		Ticket string `json:"ticket"`
	}
	if err := a.postJSON("/api/ws-ticket", nil, token, &out); err != nil {
		return "", err
	}
	if out.Ticket == "" {
		return "", fmt.Errorf("ws-ticket: empty ticket")
	}
	return out.Ticket, nil
}

func (a *authConfig) postJSON(path string, body interface{}, bearer string, out interface{}) error {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(http.MethodPost, a.httpBase+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if a.origin != "" {
		req.Header.Set("Origin", a.origin)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("%s: bad JSON: %w", path, err)
		}
	}
	return nil
}

func httpBaseFromWS(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("bad url %q: %w", rawURL, err)
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
	default:
		return "", fmt.Errorf("unsupported url scheme %q", u.Scheme)
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}
