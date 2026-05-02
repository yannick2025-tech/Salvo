// Package protocol defines the minimal generic interfaces for all
// supported protocols in Salvo (HTTP, DB, FTP, gRPC, etc.).
//
// Design principle: only truly universal concepts belong here.
// Protocol-specific types (e.g. HTTPRequest, DBRequest) live in their
// own sub-packages. This keeps the core interface small and each
// protocol free to define its own request/response shape.
package protocol

import (
	"context"
	"time"
)

// Request is the minimal interface that every protocol-specific request
// must implement. It exposes only the fields that the engine needs to
// manage timeouts and lifecycle.
type Request interface {
	// GetTimeout returns the per-request timeout. Zero means no timeout.
	GetTimeout() time.Duration
}

// Response is the minimal interface that every protocol-specific
// response must implement. It exposes the fields the engine needs for
// result aggregation and error handling.
type Response interface {
	// GetStatusCode returns the protocol-specific status code.
	GetStatusCode() int
	// GetLatency returns the round-trip duration.
	GetLatency() time.Duration
	// GetError returns the transport-level error, if any.
	GetError() error
}

// Protocol is the core interface that every protocol implementation
// must satisfy. It executes a single request and returns the response.
//
// The concrete request and response types are defined by each protocol
// sub-package (e.g. http.HTTPRequest, http.HTTPResponse). Callers
// type-assert to access protocol-specific fields.
type Protocol interface {
	// Execute sends the request and returns the response. The context
	// carries timeout and cancellation signals.
	Execute(ctx context.Context, req Request) (Response, error)
	// Name returns the protocol identifier (e.g. "http", "grpc").
	Name() string
}
