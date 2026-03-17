package audit

import (
	"context"
	"crypto"
)

// Auditor is the primary interface for audit logging operations.
// This interface provides a clean abstraction for audit capabilities,
// making it easy to swap implementations and integrate audit logging into any service.
//
// Implementations should handle:
// - Asynchronous logging (fire-and-forget)
// - Graceful degradation when the audit service is unavailable
// - Thread-safe operations
type Auditor interface {
	// LogEvent queues a standard audit event for asynchronous processing.
	// It returns an error only if the internal queue is full or if the
	// event fails initial validation before being queued.
	LogEvent(ctx context.Context, event *AuditLogRequest) error

	// SignEvent generates a digital signature for the audit request.
	// It uses the registered SignPayloadFunc to generate the signature
	// and populates the signature fields in the request.
	SignEvent(ctx context.Context, event *AuditLogRequest, keyID string) error

	// LogSignedEvent queues an event that already contains a cryptographic
	// signature and public key metadata. This is used for non-repudiation
	// workflows where the signature was generated externally or via SignEvent.
	LogSignedEvent(ctx context.Context, event *AuditLogRequest) error

	// VerifyIntegrity performs a public-key based validation of the event's
	// signature against its canonicalized payload to ensure no tampering occurred.
	VerifyIntegrity(event *AuditLogRequest, publicKey crypto.PublicKey) (bool, error)

	// IsEnabled returns true if the auditor is configured to process and
	// transmit events, allowing callers to skip expensive payload preparation.
	IsEnabled() bool

	// Close gracefully shuts down the background workers and flushes any
	// remaining events in the queue to the backend.
	Close(ctx context.Context) error
}

// AuditClient is an alias for Auditor to maintain backward compatibility.
// Deprecated: Use Auditor instead. This will be removed in a future version.
type AuditClient = Auditor
