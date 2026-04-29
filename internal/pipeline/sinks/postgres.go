package sinks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/LSFLK/argus/internal/api/v1/database"
	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostgresSink implements the Sink interface using GORM.
// It supports PostgreSQL and SQLite (useful for local development).
// This sink maintains a hash chain for non-repudiation.
type PostgresSink struct {
	db *gorm.DB
}

// NewPostgresSink creates a new PostgresSink and ensures the schema is migrated.
func NewPostgresSink(db *gorm.DB) *PostgresSink {
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		slog.Warn("Failed to auto-migrate audit_logs table", "error", err)
	}
	return &PostgresSink{db: db}
}

func (s *PostgresSink) Name() string {
	return "PostgresSink"
}

// Write persists an audit log to the database with hash chaining.
func (s *PostgresSink) Write(ctx context.Context, log *models.AuditLog) error {
	// Integrity check
	if (log.Signature != "" || log.PublicKeyID != "") && len(log.Message) == 0 {
		return fmt.Errorf("invalid audit log: signature present but message is empty")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lastLog models.AuditLog
		// Fetch the most recent log to continue the hash chain.
		// Uses SELECT ... FOR UPDATE to prevent race conditions during concurrent ingestion.
		// Ordered by created_at DESC to ensure chronological consistency.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Order("created_at DESC, id DESC").
			First(&lastLog).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		var err error
		log.PreviousHash = lastLog.CurrentHash
		log.CurrentHash, err = s.computeHash(log)
		if err != nil {
			return fmt.Errorf("failed to compute hash: %w", err)
		}

		if err := tx.Create(log).Error; err != nil {
			return err
		}
		return nil
	})
}

// WriteBatch persists multiple audit logs to the database using bulk inserts.
func (s *PostgresSink) WriteBatch(ctx context.Context, logs []models.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// For batch writes, we need to handle the hash chain for each entry.
		var lastLog models.AuditLog
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Order("created_at DESC, id DESC").
			First(&lastLog).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		prevHash := lastLog.CurrentHash
		for i := range logs {
			logs[i].PreviousHash = prevHash
			currHash, err := s.computeHash(&logs[i])
			if err != nil {
				return fmt.Errorf("failed to compute batch hash at index %d: %w", i, err)
			}
			logs[i].CurrentHash = currHash
			prevHash = logs[i].CurrentHash
		}

		// Use GORM's bulk insert feature
		return tx.CreateInBatches(logs, 100).Error
	})
}

// Close is a no-op for the GORM sink as the connection pool is managed externally.
func (s *PostgresSink) Close() error {
	return nil
}

func (s *PostgresSink) computeHash(log *models.AuditLog) (string, error) {
	h := sha256.New()
	if _, err := h.Write([]byte(log.PreviousHash)); err != nil {
		return "", err
	}

	payload := struct {
		ID        uuid.UUID
		Timestamp int64
		ActorID   string
		Action    string
		Status    string
		Signature string
	}{
		ID:        log.ID,
		Timestamp: log.Timestamp.UnixNano(),
		ActorID:   log.ActorID,
		Action:    log.Action,
		Status:    log.Status,
		Signature: log.Signature,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if _, err := h.Write(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// --- Read Methods (for AuditService compatibility) ---
// These methods are kept to support the query API of Argus.

func (s *PostgresSink) GetAuditLogs(ctx context.Context, filters *database.AuditLogFilters) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	if filters == nil {
		filters = &database.AuditLogFilters{}
	}

	query := s.db.WithContext(ctx).Model(&models.AuditLog{})
	if !filters.IncludeMessage {
		query = query.Omit("message")
	}

	if filters.TraceID != nil && *filters.TraceID != "" {
		query = query.Where("trace_id = ?", *filters.TraceID)
	}
	if filters.EventType != nil && *filters.EventType != "" {
		query = query.Where("event_type = ?", *filters.EventType)
	}
	if filters.Action != nil && *filters.Action != "" {
		query = query.Where("action = ?", *filters.Action)
	}
	if filters.Status != nil && *filters.Status != "" {
		query = query.Where("status = ?", *filters.Status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	if err := query.Order("timestamp DESC").Limit(limit).Offset(filters.Offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (s *PostgresSink) GetAuditLogByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	var log models.AuditLog
	if err := s.db.WithContext(ctx).First(&log, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (s *PostgresSink) GetAuditLogsByTraceID(ctx context.Context, traceID string) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	if err := s.db.WithContext(ctx).Where("trace_id = ?", traceID).Order("timestamp ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
