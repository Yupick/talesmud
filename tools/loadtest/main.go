package main

// Command loadtest drives concurrent WebSocket or HTTP load against a running
// TalesMUD server and prints latency percentiles as JSON.
//
// Usage (from the repository root):
//
//	go run ./tools/loadtest -url ws://localhost:8080/ws -workers 50 -rate 1 -duration 30s
//	go run ./tools/loadtest -mode http -url http://localhost:8080 -path /api/characters -workers 20
//
// Auth modes mirror the real handshake (see auth.go): guest, register, token,
// ticket. -bearer sends the session JWT straight to the WebSocket instead of
// minting a single-use ticket.
//
// -mint pre-acuña N cuentas nuevas (registro) y escribe sus JWTs a -mint-out,
// para poder después correr -auth token -tokens-file con más sesiones que las
// que los límites anti-abuse del servidor permitirían crear en caliente.

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	rawURL := flag.String("url", "ws://localhost:8080/ws", "server URL (WebSocket, or base URL for -mode http)")
	mode := flag.String("mode", "ws", "worker mode: ws or http")
	path := flag.String("path", "/api/characters", "HTTP endpoint path for -mode http")
	workers := flag.Int("workers", 10, "concurrent workers")
	rate := flag.Float64("rate", 0, "probes per second per worker (0 = connect/hold only)")
	duration := flag.Duration("duration", 30*time.Second, "how long each worker runs")
	timeout := flag.Duration("timeout", 10*time.Second, "per-operation timeout")
	authMode := flag.String("auth", "guest", "auth mode: guest, register, token, ticket")
	origin := flag.String("origin", "", "Origin header to send (empty omits it)")
	token := flag.String("token", "", "session JWT for -auth token")
	tokensFile := flag.String("tokens-file", "", "file with one session JWT per line for -auth token (round-robin across workers)")
	ticket := flag.String("ticket", "", "pre-issued WebSocket ticket for -auth ticket")
	bearer := flag.Bool("bearer", false, "send the JWT as Authorization: Bearer instead of minting a ticket")
	mint := flag.Int("mint", 0, "mint N session tokens and write to -mint-out (register mode; paces under the server's 12/min/IP limiter)")
	mintOut := flag.String("mint-out", "tokens.txt", "output file for -mint")
	mintPerMin := flag.Int("mint-per-min", 12, "per-IP registrations per minute when minting (server limit 12)")
	mintIPs := flag.Int("mint-ips", 1, "distinct client IPs to rotate via X-Forwarded-For when minting (1 = single IP; >1 only valid on trusted-loopback QA setups)")
	flag.Parse()

	if *mint > 0 {
		if err := mintTokens(mintOptions{
			baseURL: *rawURL,
			outFile: *mintOut,
			count:   *mint,
			timeout: *timeout,
			perMin:  *mintPerMin,
			ips:     *mintIPs,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "loadtest: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "loadtest: minted %d tokens to %s\n", *mint, *mintOut)
		return
	}

	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "loadtest: -workers must be >= 1")
		os.Exit(2)
	}
	m := strings.ToLower(strings.TrimSpace(*mode))
	if m != "ws" && m != "http" {
		fmt.Fprintf(os.Stderr, "loadtest: unsupported -mode %q (want ws or http)\n", *mode)
		os.Exit(2)
	}

	var tokens []string
	if *tokensFile != "" {
		loaded, terr := LoadTokensFile(*tokensFile)
		if terr != nil {
			fmt.Fprintf(os.Stderr, "loadtest: %v\n", terr)
			os.Exit(1)
		}
		tokens = loaded
	}

	auth, err := newAuthConfig(
		strings.ToLower(strings.TrimSpace(*authMode)),
		*rawURL, *origin, *token, *ticket, *bearer, *timeout, tokens,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadtest: %v\n", err)
		os.Exit(1)
	}

	st := &stats{}
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			runWorker(workerConfig{
				mode:     m,
				index:    index,
				wsURL:    *rawURL,
				httpPath: *path,
				rate:     *rate,
				duration: *duration,
				timeout:  *timeout,
				auth:     auth,
			}, st)
		}(i)
	}
	wg.Wait()

	fmt.Fprintf(os.Stderr, "loadtest: %d worker(s) finished in %s\n",
		*workers, time.Since(start).Round(time.Millisecond))
	fmt.Println(st.snapshot().JSON())
}
