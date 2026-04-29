package sinks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostgresSink implements the Sink interface using GORM.
// It supports PostgreSQL and SQLite (useful for local development).
// This sink maintains a partitioned hash chain for non-repudiation.
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

// Write persists an audit log to the database with partitioned hash chaining.
// Chains are partitioned by ActorID to prevent global lock contention.
func (s *PostgresSink) Write(ctx context.Context, log *models.AuditLog) error {
	// Integrity check
	if (log.Signature != "" || log.PublicKeyID != "") && len(log.Message) == 0 {
		return fmt.Errorf("invalid audit log: signature present but message is empty")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lastLog models.AuditLog
		// Fetch the most recent log for THIS ACTOR to continue the partitioned hash chain.
		// Uses SELECT ... FOR UPDATE to prevent race conditions during concurrent ingestion.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("actor_id = ?", log.ActorID).
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
// Note: Batching across different actors in a single transaction is supported,
// but each actor's chain is updated sequentially within the transaction.
func (s *PostgresSink) WriteBatch(ctx context.Context, logs []models.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Group logs by ActorID to handle partitioned chains efficiently
		actorLastHashes := make(map[string]string)

		for i := range logs {
			actorID := logs[i].ActorID

			// If we haven't fetched the last hash for this actor in this transaction yet
			if _, exists := actorLastHashes[actorID]; !exists {
				var lastLog models.AuditLog
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("actor_id = ?", actorID).
					Order("created_at DESC, id DESC").
					First(&lastLog).Error; err != nil && err != gorm.ErrRecordNotFound {
					return err
				}
				actorLastHashes[actorID] = lastLog.CurrentHash
			}

			logs[i].PreviousHash = actorLastHashes[actorID]
			currHash, err := s.computeHash(&logs[i])
			if err != nil {
				return fmt.Errorf("failed to compute batch hash at index %d: %w", i, err)
			}
			logs[i].CurrentHash = currHash
			actorLastHashes[actorID] = currHash
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
		ID                 uuid.UUID
		TraceID            *uuid.UUID
		Timestamp          int64
		EventType          string
		Action             string
		Status             string
		ActorType          string
		ActorID            string
		TargetType         string
		TargetID           *string
		Message            models.JSONBRawMessage
		Metadata           models.JSONBRawMessage
		Signature          string
		SignatureAlgorithm string
		PublicKeyID        string
	}{
		ID:                 log.ID,
		TraceID:            log.TraceID,
		Timestamp:          log.Timestamp.UnixNano(),
		EventType:          log.EventType,
		Action:             log.Action,
		Status:             log.Status,
		ActorType:          log.ActorType,
		ActorID:            log.ActorID,
		TargetType:         log.TargetType,
		TargetID:           log.TargetID,
		Message:            log.Message,
		Metadata:           log.Metadata,
		Signature:          log.Signature,
		SignatureAlgorithm: log.SignatureAlgorithm,
		PublicKeyID:        log.PublicKeyID,
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
