package sinks

import (
	"context"

	"github.com/LSFLK/argus/internal/api/v1/models"
)

// Sink defines the interface for audit log destinations.
// This architecture allows Argus to fan out a single audit event to multiple
// storage or streaming backends (e.g., PostgreSQL, S3, Splunk, Kafka)
// without modifying the core business logic.
type Sink interface {
	// Name returns the unique identifier for this sink.
	Name() string

	// Write writes an audit log entry to the sink.
	Write(ctx context.Context, log *models.AuditLog) error

	// WriteBatch writes multiple audit log entries to the sink in a single operation.
	WriteBatch(ctx context.Context, logs []models.AuditLog) error

	// Close performs any necessary cleanup, such as flushing buffers
	// or closing database connections.
	Close() error
}
