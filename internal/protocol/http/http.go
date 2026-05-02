// Package http implements the Protocol interface for HTTP-based
// performance testing. It supports all standard HTTP methods, custom
// headers, request bodies, and per-request timeouts.
package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/protocol"
)

// Protocol executes HTTP requests and returns protocol.Response results.
type Protocol struct {
	client *http.Client
}

// Option configures a Protocol during construction.
type Option func(*Protocol)

// WithClient sets a custom http.Client for the Protocol.
func WithClient(client *http.Client) Option {
	return func(p *Protocol) {
		p.client = client
	}
}

// NewProtocol creates an HTTP Protocol with the given options.
// By default, it uses an http.Client with no timeout (timeouts are
// handled per-request via context).
func NewProtocol(opts ...Option) *Protocol {
	p := &Protocol{
		client: &http.Client{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the protocol identifier "http".
func (p *Protocol) Name() string {
	return "http"
}

// Execute sends the HTTP request described by req and returns the
// response. The context controls cancellation and timeout.
func (p *Protocol) Execute(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	bodyReader := io.NopCloser(strings.NewReader(string(req.Body)))
	if req.Body == nil {
		bodyReader = nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, string(req.Method), req.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http: create request: %w", err)
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	start := time.Now()
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: execute request: %w", err)
	}
	latency := time.Since(start)

	respBody, err := io.ReadAll(httpResp.Body)
	closeErr := httpResp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("http: read response body: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("http: close response body: %w", closeErr)
	}

	respHeaders := make(map[string][]string)
	for k, v := range httpResp.Header {
		respHeaders[k] = v
	}

	return &protocol.Response{
		StatusCode: httpResp.StatusCode,
		Headers:    respHeaders,
		Body:       respBody,
		Latency:    latency,
	}, nil
}
