package sinks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/LSFLK/argus/internal/api/v1/models"
)

// ConsoleSink implements the Sink interface by logging entries to stdout.
// This is useful for local development, debugging, or as a failsafe logging destination.
type ConsoleSink struct{}

// NewConsoleSink creates a new ConsoleSink.
func NewConsoleSink() *ConsoleSink {
	return &ConsoleSink{}
}

func (s *ConsoleSink) Name() string {
	return "ConsoleSink"
}

// Write marshals the audit log to JSON and prints it to the console.
func (s *ConsoleSink) Write(ctx context.Context, log *models.AuditLog) error {
	b, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal log for console: %w", err)
	}

	slog.Info("Audit Log (ConsoleSink)", "log", string(b))
	return nil
}

// WriteBatch logs multiple audit logs to the console.
func (s *ConsoleSink) WriteBatch(ctx context.Context, logs []models.AuditLog) error {
	for i := range logs {
		if err := s.Write(ctx, &logs[i]); err != nil {
			return err
		}
	}
	return nil
}

// Close is a no-op for the ConsoleSink.
func (s *ConsoleSink) Close() error {
	return nil
}
