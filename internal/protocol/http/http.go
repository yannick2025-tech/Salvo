// Package http implements the Protocol interface for HTTP-based
// performance testing. It defines HTTP-specific request and response
// types and supports all standard HTTP methods, custom headers,
// request bodies, and per-request timeouts.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/protocol"
)

// Method represents an HTTP method.
type Method string

const (
	MethodGet    Method = "GET"
	MethodPost   Method = "POST"
	MethodPut    Method = "PUT"
	MethodDelete Method = "DELETE"
	MethodPatch  Method = "PATCH"
	MethodHead   Method = "HEAD"
)

// HTTPRequest is the HTTP-specific request type. It implements the
// protocol.Request interface.
type HTTPRequest struct {
	Method  Method
	URL     string
	Headers map[string]string
	Body    []byte
	Timeout time.Duration
}

// GetTimeout implements protocol.Request.
func (r *HTTPRequest) GetTimeout() time.Duration {
	return r.Timeout
}

// HTTPResponse is the HTTP-specific response type. It implements the
// protocol.Response interface.
type HTTPResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	Latency    time.Duration
	Error      error
}

// GetStatusCode implements protocol.Response.
func (r *HTTPResponse) GetStatusCode() int {
	return r.StatusCode
}

// GetLatency implements protocol.Response.
func (r *HTTPResponse) GetLatency() time.Duration {
	return r.Latency
}

// GetError implements protocol.Response.
func (r *HTTPResponse) GetError() error {
	return r.Error
}

// IsError returns true if the status code indicates a client or server
// error (4xx or 5xx).
func (r *HTTPResponse) IsError() bool {
	return r.StatusCode >= 400
}

// IsSuccess returns true if the status code is in the 2xx range.
func (r *HTTPResponse) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// BodyJSON unmarshals the response body into the provided pointer.
func (r *HTTPResponse) BodyJSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

// Protocol executes HTTP requests and returns HTTPResponse results.
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
func (p *Protocol) Execute(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	httpReq, ok := req.(*HTTPRequest)
	if !ok {
		return nil, fmt.Errorf("http: expected *HTTPRequest, got %T", req)
	}

	if httpReq.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, httpReq.Timeout)
		defer cancel()
	}

	bodyReader := io.NopCloser(strings.NewReader(string(httpReq.Body)))
	if httpReq.Body == nil {
		bodyReader = nil
	}

	goReq, err := http.NewRequestWithContext(ctx, string(httpReq.Method), httpReq.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http: create request: %w", err)
	}

	for k, v := range httpReq.Headers {
		goReq.Header.Set(k, v)
	}

	start := time.Now()
	httpResp, err := p.client.Do(goReq)
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

	return &HTTPResponse{
		StatusCode: httpResp.StatusCode,
		Headers:    respHeaders,
		Body:       respBody,
		Latency:    latency,
	}, nil
}
