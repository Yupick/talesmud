package main

// mint.go implements -mint: pre-acuñar N session JWTs via
// POST /api/auth/register, porque los límites anti-abuse del servidor
// (MaxGuestAccounts=20, 12 registros/min/IP) impiden mintar >20 identidades
// en caliente con una sola IP.
//
// El rate-limit del servidor es una ventana deslizante por IP
// (newIPLimiter(12, time.Minute) en pkg/server/handler/localauth.go, claveada
// en c.ClientIP()). Dos estrategias, ambas compatibles con ese límite:
//
//  1. Default (-mint-ips 1): pacing estricto de -mint-per-min registros por
//     minuto desde la IP real. Respeta el límite sin trucos; tarda ~n/12 min.
//  2. -mint-ips K: rota K direcciones sintéticas RFC1918 vía X-Forwarded-For.
//     Cada IP sintética tiene su propio presupuesto de 12/min, así que el
//     throughput agregado es K×12/min. Válido aquí porque el servidor corre en
//     confianza loopback (TrustedProxies = 127.0.0.1/::1 por defecto), que es
//     exactamente el setup de QA local; en producción detrás de un proxy
//     real esto es el vector de bypass documentado en §B19.4.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// mintWindowReset is how long to wait after a 429 before retrying: the server
// keeps a sliding 1-minute window per IP, so a full window must elapse.
const mintWindowReset = 61 * time.Second

// mintMinInterval floors the pacing delay so the harness never busy-loops the
// local server even with a huge synthetic-IP budget.
const mintMinInterval = 20 * time.Millisecond

// mintInterval computes the delay between registration requests so the
// aggregate rate stays under perMin registrations/minute across ipCount
// distinct client IPs (1 = the real source IP).
func mintInterval(ipCount, perMin int) time.Duration {
	if ipCount < 1 {
		ipCount = 1
	}
	if perMin < 1 {
		perMin = 12
	}
	budgetPerSec := float64(ipCount*perMin) / 60.0
	iv := time.Duration(float64(time.Second) / budgetPerSec)
	if iv < mintMinInterval {
		iv = mintMinInterval
	}
	return iv
}

// xffIP derives the i-th synthetic client IP for X-Forwarded-For rotation.
// The 10/8 range keeps the addresses recognisably test-only in server logs.
func xffIP(i int) string {
	i = i + 1 // avoid x.x.x.0
	return fmt.Sprintf("10.%d.%d.%d", (i>>16)&0xff, (i>>8)&0xff, i&0xff)
}

// mintRegisterResponse parses one /api/auth/register response.
func mintRegisterResponse(status int, body []byte) (token string, rateLimited bool, err error) {
	if status == http.StatusTooManyRequests {
		return "", true, nil
	}
	if status < 200 || status >= 300 {
		return "", false, fmt.Errorf("register: HTTP %d: %s", status, strings.TrimSpace(string(body)))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", false, fmt.Errorf("register: bad JSON: %w", err)
	}
	if out.Token == "" {
		return "", false, fmt.Errorf("register: empty token")
	}
	return out.Token, false, nil
}

// mintBody builds the registration payload for one identity. The username
// must satisfy the server's validation (3-20 chars: lowercase letters,
// digits, underscore), so we use a compact base36 encoding of the uniqueness
// seed rather than a raw decimal timestamp.
func mintBody(index int, nano int64) map[string]string {
	seed := fmt.Sprintf("%x%x", nano&0xffffffff, index)
	// Keep it under 20 chars: lt_ (3) + up to 16 hex digits.
	if len(seed) > 16 {
		seed = seed[len(seed)-16:]
	}
	uname := "lt_" + seed
	return map[string]string{
		"username": uname,
		"email":    uname + "@example.test",
		"password": "SimPassword123!",
	}
}

// mintOptions configures one mint run.
type mintOptions struct {
	baseURL string
	outFile string
	count   int
	timeout time.Duration
	perMin  int // per-IP budget (server default: 12/min)
	ips     int // distinct client IPs to rotate (1 = just the real one)
}

// mintTokens registers opts.count distinct accounts and writes one session JWT
// per line to opts.outFile. baseURL may be a ws:// URL or an http(s) base.
func mintTokens(opts mintOptions) error {
	if opts.count <= 0 {
		return fmt.Errorf("mint count must be > 0")
	}
	if opts.perMin < 1 {
		opts.perMin = 12
	}
	if opts.ips < 1 {
		opts.ips = 1
	}
	base, err := httpBaseFromWS(opts.baseURL)
	if err != nil {
		base = strings.TrimRight(strings.TrimSpace(opts.baseURL), "/")
	}
	f, err := os.Create(opts.outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	client := &http.Client{Timeout: opts.timeout}
	interval := mintInterval(opts.ips, opts.perMin)
	minted := 0
	consecutiveRateLimits := 0
	// Attempt budget: 429 waits do not consume it, only hard failures do.
	for i := 0; minted < opts.count && i < 5*opts.count; i++ {
		raw, err := json.Marshal(mintBody(i, time.Now().UnixNano()))
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, base+"/api/auth/register", bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if opts.ips > 1 {
			req.Header.Set("X-Forwarded-For", xffIP(i%opts.ips))
		}
		resp, err := client.Do(req)
		if err != nil {
			// Network error: brief pause, then retry without burning a slot.
			time.Sleep(time.Second)
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()

		token, rateLimited, merr := mintRegisterResponse(resp.StatusCode, body)
		if rateLimited {
			consecutiveRateLimits++
			if consecutiveRateLimits > 3 {
				return fmt.Errorf("register persistently rate-limited after %d tokens (wait ≥1 min and retry)", minted)
			}
			// With XFF rotation a 429 means the budget model is off (or the
			// limiter saw the real IP): wait out the window and retry.
			time.Sleep(mintWindowReset)
			i--
			continue
		}
		consecutiveRateLimits = 0
		if merr != nil {
			// Hard failure (validation/conflict): try the next identity.
			time.Sleep(time.Second)
			continue
		}
		if _, err := f.WriteString(token + "\n"); err != nil {
			return err
		}
		minted++
		if minted < opts.count {
			time.Sleep(interval)
		}
	}
	if minted < opts.count {
		return fmt.Errorf("minted only %d of %d tokens", minted, opts.count)
	}
	return nil
}
