// Package llmgen is the generation pipeline shared by every LLM-backed
// feature. It renders a prompt template, applies caching and rate limits,
// calls the provider through llm.Client, filters and validates the output,
// and otherwise degrades to a deterministic fallback. Callers never talk to
// a provider directly.
package llmgen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/talesmud/talesmud/pkg/service/llm"
	"github.com/talesmud/talesmud/pkg/service/llmprompt"
)

// Task identifies one generation use case. The string doubles as the prompt
// template id.
type Task string

const (
	TaskCharacterName        Task = "character.name"
	TaskCharacterDescription Task = "character.description"
	TaskQuestGenerate        Task = "quest.generate"
	TaskNPCDialogue          Task = "npc.dialogue"
	TaskNPCMemorySummary     Task = "npc.memory_summary"
	TaskRoomDescription      Task = "room.description"
	TaskDMNarration          Task = "dm.narration"
)

// Source records where a response came from.
type Source string

const (
	SourceLLM           Source = "llm"
	SourceCache         Source = "cache"
	SourceFallback      Source = "fallback"
	SourceDeterministic Source = "deterministic"
)

// Issue is a single validation or filtering problem with a candidate output.
type Issue struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
}

func (i Issue) String() string {
	if i.Path != "" {
		return i.Path + ": " + i.Message
	}
	return i.Message
}

// Request is one generation call.
type Request struct {
	Task    Task
	Vars    map[string]any
	UserID  string
	RoomID  string
	Timeout time.Duration

	// RequireLLM disables the deterministic fallback (the caller cannot
	// accept canned content). Failure then surfaces as an error.
	RequireLLM bool

	// ReturnParsedOnFailure, when output failed validation, returns the best
	// effort payload with issues attached instead of the generic fallback.
	ReturnParsedOnFailure bool

	// LimitKey overrides the rate-limit bucket key (defaults to task:user).
	LimitKey string

	// Temperature, when > 0, overrides the template temperature for this call
	// (used by per-NPC LLM configuration).
	Temperature float64

	Parse    ParseFunc
	Validate ValidateFunc
	Fallback FallbackFunc
}

// ParseFunc turns raw provider text into the canonical payload handed to
// Validate and returned in Response.JSON.
type ParseFunc func(raw string) (json.RawMessage, error)

// ValidateFunc reports problems with a parsed payload. An empty slice means
// the payload is acceptable.
type ValidateFunc func(ctx context.Context, req Request, payload json.RawMessage) []Issue

// FallbackFunc produces deterministic content for a task.
type FallbackFunc func(req Request, rendered llmprompt.Rendered) (Response, error)

// Response is the pipeline result.
type Response struct {
	Task          Task            `json:"task"`
	Text          string          `json:"text"`
	JSON          json.RawMessage `json:"json,omitempty"`
	Source        Source          `json:"source"`
	PromptVersion string          `json:"promptVersion,omitempty"`
	PromptHash    string          `json:"promptHash,omitempty"`
	Provider      string          `json:"provider,omitempty"`
	Attempts      int             `json:"attempts"`
	LatencyMS     int64           `json:"latencyMs"`
	Cached        bool            `json:"cached"`
	InputTokens   int             `json:"inputTokens,omitempty"`
	OutputTokens  int             `json:"outputTokens,omitempty"`
	Issues        []Issue         `json:"issues,omitempty"`
}

// Generator is the narrow contract consumers depend on.
type Generator interface {
	Generate(ctx context.Context, req Request) (Response, error)
}

// Sentinel errors surfaced to callers.
var (
	ErrLLMUnavailable = errors.New("llmgen: llm unavailable")
	ErrRateLimited    = errors.New("llmgen: rate limited")
	ErrBusy           = errors.New("llmgen: too many concurrent generations")
	ErrGeneration     = errors.New("llmgen: generation failed")
)

// Pipeline implements Generator.
type Pipeline struct {
	client   llm.Client
	reg      llmprompt.Registry
	cfg      Config
	provider string
	sem      chan struct{}
	stats    *Stats
}

var _ Generator = (*Pipeline)(nil)

// New builds a pipeline. client may be nil (provider disabled); reg is
// required. cfg is normalized with defaults.
func New(client llm.Client, reg llmprompt.Registry, cfg Config) *Pipeline {
	cfg = cfg.withDefaults()
	p := &Pipeline{
		client:   client,
		reg:      reg,
		cfg:      cfg,
		provider: providerName(client),
		sem:      make(chan struct{}, cfg.MaxConcurrent),
		stats:    NewStats(),
	}
	return p
}

// Stats exposes the counters collected so far.
func (p *Pipeline) Stats() StatsSnapshot {
	if p == nil {
		return StatsSnapshot{ByTask: map[string]int64{}, BySource: map[string]int64{}}
	}
	return p.stats.Snapshot()
}

// Registry returns the prompt registry the pipeline renders from.
func (p *Pipeline) Registry() llmprompt.Registry {
	if p == nil {
		return nil
	}
	return p.reg
}

// Client returns the configured provider (may be nil).
func (p *Pipeline) Client() llm.Client {
	if p == nil {
		return nil
	}
	return p.client
}

// Enabled reports whether a provider is configured.
func (p *Pipeline) Enabled() bool {
	return p != nil && p.client != nil
}

// Provider returns the provider/model label used in responses and logs.
func (p *Pipeline) Provider() string {
	if p == nil {
		return "none"
	}
	return p.provider
}

func providerName(c llm.Client) string {
	if c == nil {
		return "none"
	}
	if m := llm.ModelOf(c); m != "" {
		return m
	}
	return fmt.Sprintf("%T", c)
}

// Generate runs the full pipeline for one request.
func (p *Pipeline) Generate(ctx context.Context, req Request) (Response, error) {
	start := time.Now()
	if p == nil || p.reg == nil {
		return Response{}, fmt.Errorf("%w: pipeline is not configured", ErrGeneration)
	}
	if strings.TrimSpace(string(req.Task)) == "" {
		return Response{}, fmt.Errorf("%w: task is required", ErrGeneration)
	}

	rendered, err := p.reg.Render(string(req.Task), req.Vars)
	if err != nil {
		p.stats.recordError(req.Task)
		return Response{}, fmt.Errorf("%w: %v", ErrGeneration, err)
	}

	key := cacheKey(req.Task, rendered, p.provider, req.Vars, req.Temperature)
	if ttl := p.cfg.cacheTTL(req.Task); ttl > 0 {
		if cached, ok := p.cfg.Cache.Get(key); ok {
			cached.Cached = true
			cached.Source = SourceCache
			cached.LatencyMS = time.Since(start).Milliseconds()
			return cached, nil
		}
	}

	if p.client == nil {
		if req.RequireLLM {
			p.stats.recordError(req.Task)
			return Response{}, fmt.Errorf("%w: no provider configured", ErrLLMUnavailable)
		}
		resp, fbErr := p.fallback(req, rendered)
		if fbErr != nil {
			p.stats.recordError(req.Task)
			return Response{}, fbErr
		}
		return p.finishFallback(req, rendered, resp, start)
	}

	if !p.allowRate(req) {
		p.stats.recordRateLimited(req.Task)
		if resp, fbErr := p.fallback(req, rendered); fbErr == nil && !req.RequireLLM {
			return p.finishFallback(req, rendered, resp, start)
		}
		return Response{}, ErrRateLimited
	}

	if !p.acquire(ctx) {
		p.stats.recordError(req.Task)
		if resp, fbErr := p.fallback(req, rendered); fbErr == nil && !req.RequireLLM {
			return p.finishFallback(req, rendered, resp, start)
		}
		return Response{}, ErrBusy
	}
	defer p.release()

	resp, genErr := p.callLLM(ctx, req, rendered, key)
	if genErr != nil {
		p.stats.recordError(req.Task)
		if req.RequireLLM {
			return resp, genErr
		}
		fb, fbErr := p.fallback(req, rendered)
		if fbErr != nil {
			return Response{}, genErr
		}
		return p.finishFallback(req, rendered, fb, start)
	}
	resp.LatencyMS = time.Since(start).Milliseconds()
	p.stats.record(resp, time.Since(start))
	return resp, nil
}

// callLLM performs the attempt loop: provider call, parse, validate, repair.
func (p *Pipeline) callLLM(ctx context.Context, req Request, rendered llmprompt.Rendered, key string) (Response, error) {
	maxAttempts := 1 + rendered.Retry
	if p.cfg.MaxAttempts > 0 && maxAttempts > p.cfg.MaxAttempts {
		maxAttempts = p.cfg.MaxAttempts
	}

	var (
		feedback string
		lastResp Response
		lastRaw  string
		lastErr  error
	)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		timeout := p.effectiveTimeout(req, rendered)
		actx := ctx
		var cancel context.CancelFunc
		if timeout > 0 {
			actx, cancel = context.WithTimeout(ctx, timeout)
		}
		temperature := rendered.Temperature
		if req.Temperature > 0 {
			temperature = req.Temperature
		}
		raw, err := llm.Complete(actx, p.client, rendered.System, repairUser(rendered.User, feedback),
			llm.Options{Temperature: temperature, MaxTokens: rendered.MaxTokens})
		if cancel != nil {
			cancel()
		}
		if err != nil {
			if ctx.Err() != nil {
				return Response{}, ctx.Err()
			}
			lastErr = err
			continue
		}
		lastRaw = raw

		resp, issues, perr := p.assemble(req, rendered, raw)
		resp.PromptVersion = rendered.Version
		resp.PromptHash = rendered.Hash
		resp.Provider = p.provider
		resp.Attempts = attempt
		if perr != nil {
			lastErr = perr
			lastResp = resp
			continue
		}
		if len(issues) == 0 {
			p.cacheStore(key, req.Task, resp)
			return resp, nil
		}
		lastResp = resp
		lastResp.Issues = issues
		lastErr = fmt.Errorf("%w: %s", ErrGeneration, joinIssues(issues))
		feedback = joinIssues(issues)
	}

	if req.ReturnParsedOnFailure && lastResp.Task != "" {
		p.cacheStore(key, req.Task, lastResp)
		return lastResp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: provider returned no usable output", ErrGeneration)
	}
	_ = lastRaw
	return Response{}, lastErr
}

func (p *Pipeline) assemble(req Request, rendered llmprompt.Rendered, raw string) (Response, []Issue, error) {
	var (
		payload json.RawMessage
		err     error
	)
	resp := Response{Task: req.Task, Source: SourceLLM}
	switch {
	case req.Parse != nil:
		payload, err = req.Parse(raw)
	case rendered.Output == llmprompt.OutputJSON:
		payload, err = ExtractJSON(raw)
	default:
		text := CleanTextOutput(raw, 2000)
		payload = json.RawMessage(text)
	}
	if err != nil {
		return resp, nil, err
	}

	resp.JSON = payload
	if rendered.Output == llmprompt.OutputJSON {
		resp.Text = string(payload)
	} else {
		resp.Text = CleanTextOutput(string(payload), 0)
	}

	var issues []Issue
	if !OutputAllowed(req.Task, resp.Text) {
		issues = append(issues, Issue{
			Severity: "error",
			Code:     "output_rejected",
			Message:  "La salida del modelo está vacía o no es aceptable.",
		})
	}
	if req.Validate != nil {
		issues = append(issues, req.Validate(context.Background(), req, payload)...)
	}
	return resp, issues, nil
}

func (p *Pipeline) fallback(req Request, rendered llmprompt.Rendered) (Response, error) {
	if !p.cfg.AllowDeterministic {
		return Response{}, fmt.Errorf("%w: deterministic fallback disabled", ErrGeneration)
	}
	fn := req.Fallback
	if fn == nil && p.cfg.FallbackFuncs != nil {
		fn = p.cfg.FallbackFuncs[req.Task]
	}
	if fn == nil {
		fn = builtinFallback
	}
	resp, err := fn(req, rendered)
	if err != nil {
		return Response{}, err
	}
	if resp.Source == "" {
		resp.Source = SourceFallback
	}
	resp.Task = req.Task
	resp.PromptVersion = rendered.Version
	resp.PromptHash = rendered.Hash
	resp.Provider = "deterministic"
	if resp.Text == "" && len(resp.JSON) > 0 {
		resp.Text = string(resp.JSON)
	}
	return resp, nil
}

func (p *Pipeline) finishFallback(req Request, rendered llmprompt.Rendered, resp Response, start time.Time) (Response, error) {
	resp.Task = req.Task
	resp.PromptVersion = rendered.Version
	resp.PromptHash = rendered.Hash
	if resp.Source == "" {
		resp.Source = SourceDeterministic
	}
	if resp.Provider == "" {
		resp.Provider = "deterministic"
	}
	resp.LatencyMS = time.Since(start).Milliseconds()
	p.stats.record(resp, time.Since(start))
	return resp, nil
}

func (p *Pipeline) cacheStore(key string, task Task, resp Response) {
	ttl := p.cfg.cacheTTL(task)
	if p.cfg.Cache == nil || ttl <= 0 {
		return
	}
	stored := resp
	stored.Cached = false
	p.cfg.Cache.Set(key, stored, ttl)
}

func (p *Pipeline) allowRate(req Request) bool {
	if p.cfg.Limiter == nil {
		return true
	}
	key := strings.TrimSpace(req.LimitKey)
	if key == "" {
		if user := strings.TrimSpace(req.UserID); user != "" {
			key = string(req.Task) + ":" + user
		} else {
			key = string(req.Task)
		}
	}
	return p.cfg.Limiter.Allow(key)
}

func (p *Pipeline) acquire(ctx context.Context) bool {
	if p.sem == nil {
		return true
	}
	wait := p.cfg.SemaphoreWait
	if wait <= 0 {
		wait = 2 * time.Second
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case p.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	case <-timer.C:
		return false
	}
}

func (p *Pipeline) release() {
	if p.sem == nil {
		return
	}
	select {
	case <-p.sem:
	default:
	}
}

func (p *Pipeline) effectiveTimeout(req Request, rendered llmprompt.Rendered) time.Duration {
	eff := p.cfg.PromptTimeout
	if rendered.Timeout > 0 && (eff <= 0 || rendered.Timeout < eff) {
		eff = rendered.Timeout
	}
	if req.Timeout > 0 && (eff <= 0 || req.Timeout < eff) {
		eff = req.Timeout
	}
	return eff
}

func repairUser(user, feedback string) string {
	if strings.TrimSpace(feedback) == "" {
		return user
	}
	return user + "\n\nLa respuesta anterior no pasó la validación. Corrige estos problemas y devuelve solo la salida pedida:\n- " +
		strings.ReplaceAll(strings.TrimSpace(feedback), "\n", "\n- ")
}

func joinIssues(issues []Issue) string {
	parts := make([]string, 0, len(issues))
	for _, i := range issues {
		parts = append(parts, i.String())
	}
	return strings.Join(parts, "; ")
}

func cacheKey(task Task, rendered llmprompt.Rendered, provider string, vars map[string]any, temperature float64) string {
	canonical := make(map[string]any, len(vars))
	for k, v := range vars {
		if strings.HasPrefix(k, "_") {
			continue
		}
		canonical[k] = v
	}
	raw, err := json.Marshal(canonical)
	if err != nil {
		raw = []byte(fmt.Sprintf("%v", canonical))
	}
	key := string(task) + "|" + rendered.Hash + "|" + provider + "|" + string(raw)
	// Keep legacy keys unchanged for the common zero-value request while
	// making per-NPC temperatures cache-distinct.
	if temperature > 0 {
		key += fmt.Sprintf("|temp=%.3f", temperature)
	}
	return key
}
