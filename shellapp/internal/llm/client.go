package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// HTTPClient talks to one OpenAI-compatible server, whose address it asks
// Config.Endpoint for on every request. It is safe for concurrent use.
type HTTPClient struct {
	cfg Config
	sem chan struct{}

	mu     sync.RWMutex
	health Health
}

// route is which of the server's three endpoints a Request selects.
type route int

const (
	routeChat route = iota
	routeCompletions
	routeInfill
)

// routeFor is the whole routing rule. Adding a fourth shape means adding a
// branch here and nowhere else.
func routeFor(req Request) route {
	switch {
	case req.Suffix != "":
		return routeInfill
	case req.Prompt != "":
		return routeCompletions
	default:
		return routeChat
	}
}

func (r route) path() string {
	switch r {
	case routeInfill:
		return "/infill"
	case routeCompletions:
		return "/v1/completions"
	default:
		return "/v1/chat/completions"
	}
}

// maxResponseBytes caps a non-streamed body. A 3B model cannot produce
// anything near this; a proxy serving an error page can.
const maxResponseBytes = 32 << 20

// Complete runs a request to completion and returns the whole text.
func (c *HTTPClient) Complete(ctx context.Context, req Request) (*Response, error) {
	return c.run(ctx, req, nil, false)
}

// Stream runs a request and calls onChunk for every delta as it arrives, then
// once more with Done set. The returned *Response holds the assembled text even
// when the error is non-nil, so a caller that cancels mid-generation keeps what
// it already showed.
func (c *HTTPClient) Stream(ctx context.Context, req Request, onChunk func(Chunk)) (*Response, error) {
	if onChunk == nil {
		onChunk = func(Chunk) {}
	}
	return c.run(ctx, req, onChunk, true)
}

func (c *HTTPClient) run(parent context.Context, req Request, onChunk func(Chunk), stream bool) (*Response, error) {
	rt := routeFor(req)

	base, err := c.base(parent)
	if err != nil {
		return nil, err
	}

	// Our own timeout applies only when the caller did not bring a deadline:
	// a caller that asked for five minutes meant it.
	ctx, cancel := parent, context.CancelFunc(func() {})
	if _, ok := parent.Deadline(); !ok {
		timeout := c.cfg.Timeout
		if rt == routeInfill {
			timeout = c.cfg.FastTimeout
		}
		ctx, cancel = context.WithTimeout(parent, timeout)
	}
	defer cancel()

	// /infill is on the typing path and bypasses the queue for the same reason
	// Priority does: latency, not throughput, is what it is judged on.
	release, err := c.acquire(ctx, req.Priority || rt == routeInfill)
	if err != nil {
		return nil, c.classify(parent, ctx, err)
	}
	defer release()

	payload, err := json.Marshal(buildBody(rt, req, stream))
	if err != nil {
		return nil, fmt.Errorf("llm: encoding request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+rt.path(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("llm: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
		httpReq.Header.Set("Cache-Control", "no-cache")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}

	start := c.cfg.Now()
	resp := &Response{}
	defer func() { resp.Total = c.cfg.Now().Sub(start) }()

	hres, err := c.cfg.HTTPClient.Do(httpReq)
	if err != nil {
		return resp, c.classify(parent, ctx, err)
	}
	defer func() {
		// Drain a little before closing so the connection can be reused; a
		// partially-read body is dropped rather than pooled.
		io.CopyN(io.Discard, hres.Body, 4<<10)
		hres.Body.Close()
	}()

	if hres.StatusCode != http.StatusOK {
		return resp, statusError(hres)
	}

	if stream {
		err = readStream(hres.Body, onChunk, resp, start, c.cfg.Now)
		if err == nil {
			onChunk(Chunk{Done: true})
		}
	} else {
		// Nothing is generated progressively here, so the first text arrives
		// with the whole body: time to headers is the closest honest measure.
		resp.TTFT = c.cfg.Now().Sub(start)
		err = readWhole(hres.Body, resp)
	}
	return resp, c.classify(parent, ctx, err)
}

// Probe reports whether the server is up, caching the answer for probeTTL. The
// cache exists because the status line asks often and a health check is not
// free; the write lock is held across the request so a burst of concurrent
// callers produces exactly one.
//
// Probe must never run during app startup — the endpoint function may spawn a
// server, and startup is not allowed to touch the network.
func (c *HTTPClient) Probe(ctx context.Context) Health {
	c.mu.RLock()
	h := c.health
	c.mu.RUnlock()
	if c.fresh(h) {
		return h
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fresh(c.health) {
		return c.health
	}
	c.health = c.probe(ctx)
	return c.health
}

// InvalidateProbe drops the cached health, so the next Probe really asks. The
// supervisor calls it after spawning or stopping a server.
func (c *HTTPClient) InvalidateProbe() {
	c.mu.Lock()
	c.health = Health{}
	c.mu.Unlock()
}

func (c *HTTPClient) fresh(h Health) bool {
	return !h.Checked.IsZero() && c.cfg.Now().Sub(h.Checked) < probeTTL
}

func (c *HTTPClient) probe(ctx context.Context) Health {
	h := Health{Checked: c.cfg.Now()}

	base, err := c.base(ctx)
	if err != nil {
		h.Err = err
		return h
	}
	h.Endpoint = base

	// A short deadline of its own: "is it up" must answer fast even when a
	// generation is allowed a minute.
	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(pctx, http.MethodGet, base+"/health", nil)
	if err != nil {
		h.Err = err
		return h
	}
	req.Header.Set("Accept", "application/json")

	start := c.cfg.Now()
	res, err := c.cfg.HTTPClient.Do(req)
	h.Latency = c.cfg.Now().Sub(start)
	if err != nil {
		h.Err = c.classify(ctx, pctx, err)
		return h
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode != http.StatusOK {
		h.Err = &StatusError{Code: res.StatusCode, Status: res.Status, Message: errMessage(body)}
		return h
	}

	var parsed struct {
		Model    string `json:"model"`
		Settings struct {
			Model string `json:"model"`
		} `json:"default_generation_settings"`
	}
	json.Unmarshal(body, &parsed)
	h.Model = parsed.Model
	if h.Model == "" {
		h.Model = parsed.Settings.Model
	}
	h.OK = true
	return h
}

// base resolves the server address for this call.
func (c *HTTPClient) base(ctx context.Context) (string, error) {
	if c.cfg.Endpoint == nil {
		return "", ErrDisabled
	}
	ep, err := c.cfg.Endpoint(ctx)
	if err != nil {
		if errors.Is(err, ErrDisabled) || errors.Is(err, ErrUnreachable) || errors.Is(err, ErrModelMissing) {
			return "", err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	ep = strings.TrimRight(strings.TrimSpace(ep), "/")
	if ep == "" {
		return "", ErrDisabled
	}
	return ep, nil
}

// acquire takes a slot from the in-flight semaphore, or nothing at all when the
// request is allowed to skip the queue.
func (c *HTTPClient) acquire(ctx context.Context, priority bool) (func(), error) {
	if priority || c.sem == nil {
		return func() {}, nil
	}
	select {
	case c.sem <- struct{}{}:
		return func() { <-c.sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// classify turns a transport error into one the caller can branch on. The order
// matters: the caller's own cancellation is reported verbatim, because "the
// user pressed esc" and "the model failed" must not look alike.
func (c *HTTPClient) classify(parent, derived context.Context, err error) error {
	if err == nil {
		return nil
	}
	if pe := parent.Err(); pe != nil {
		return pe
	}
	if errors.Is(err, context.DeadlineExceeded) || derived.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%w: %v", ErrTimeout, err)
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("%w: %v", ErrTimeout, err)
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	return err
}

// StatusError is a non-200 answer from the server, with whatever explanation
// could be salvaged from the body — which is JSON from llama-server and HTML
// from anything that got in the way.
type StatusError struct {
	Code    int
	Status  string
	Message string
}

func (e *StatusError) Error() string {
	status := e.Status
	if status == "" {
		status = fmt.Sprintf("%d", e.Code)
	}
	if e.Message != "" {
		return fmt.Sprintf("llm: server returned %s: %s", status, e.Message)
	}
	return fmt.Sprintf("llm: server returned %s", status)
}

// Unwrap maps the status codes that mean something specific onto the package's
// sentinels; everything else is just a StatusError.
func (e *StatusError) Unwrap() error {
	switch e.Code {
	case http.StatusNotFound:
		return ErrModelMissing
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return ErrUnreachable
	case http.StatusBadRequest:
		if strings.Contains(strings.ToLower(e.Message), "model") {
			return ErrModelMissing
		}
	}
	return nil
}

func statusError(res *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	return &StatusError{Code: res.StatusCode, Status: res.Status, Message: errMessage(body)}
}

// errMessage pulls a human explanation out of an error body: the usual JSON
// shapes first, then the raw text collapsed to one line, which is what an HTML
// error page degrades to.
func errMessage(body []byte) string {
	var envelope struct {
		Error json.RawMessage `json:"error"`
		Msg   string          `json:"message"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if len(envelope.Error) > 0 {
			var nested struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(envelope.Error, &nested) == nil && nested.Message != "" {
				return truncate(nested.Message)
			}
			var plain string
			if json.Unmarshal(envelope.Error, &plain) == nil && plain != "" {
				return truncate(plain)
			}
		}
		if envelope.Msg != "" {
			return truncate(envelope.Msg)
		}
	}
	return truncate(strings.Join(strings.Fields(string(body)), " "))
}

func truncate(s string) string {
	const max = 200
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
