package audit

import (
	"context"
	"crypto"
)

// Auditor is the primary interface for audit logging operations
type Auditor interface {
	// LogEvent logs a standard audit event asynchronously
	LogEvent(ctx context.Context, event *AuditLogRequest)

	// SignEvent generates a digital signature for the audit request.
	// It hashes the payload and signs it using the provided private key,
	// populating the signature fields in the request
	SignEvent(event *AuditLogRequest, privateKey crypto.Signer, keyID string) error

	// LogSignedEvent specifically handles the transmission of events that
	// already contain cryptographic signatures and public key metadata
	LogSignedEvent(ctx context.Context, event *AuditLogRequest)

	// IsEnabled returns whether audit logging is currently enabled
	IsEnabled() bool
}

// AuditClient is an alias for Auditor to maintain backward compatibility
// Deprecated: Use Auditor instead. This will be removed in a future version.
type AuditClient = Auditor
