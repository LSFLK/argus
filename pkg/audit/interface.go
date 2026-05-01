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
	// Returns true if the event was accepted, false if the client is
	// shutting down, disabled, or the queue is full.
	LogEvent(ctx context.Context, event *AuditLogRequest) bool

	// SignEvent generates a digital signature for the audit request
	// using the registered SignPayloadFunc
	SignEvent(event *AuditLogRequest) error

	// LogSignedEvent queues an event that already contains a cryptographic signature
	LogSignedEvent(ctx context.Context, event *AuditLogRequest)

	// VerifyIntegrity validates a log's signature using a provided public key
	VerifyIntegrity(event *AuditLogRequest, publicKey crypto.PublicKey) (bool, error)

	// IsEnabled returns true if the auditor is configured to process events
	IsEnabled() bool

	// Close gracefully shuts down the client and flushes the queue
	Close(ctx context.Context) error
}

// AuditClient is an alias for Auditor to maintain backward compatibility.
// Deprecated: Use Auditor instead. This will be removed in a future version.
type AuditClient = Auditor
