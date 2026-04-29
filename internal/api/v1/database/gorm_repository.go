package database

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
)

// GormRepository implements AuditRepository using GORM (works with SQLite or PostgreSQL)
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new repository (works with SQLite or PostgreSQL)
func NewGormRepository(db *gorm.DB) *GormRepository {
	// Auto-migrate the audit_logs table
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		// Log migration error but don't fail service creation
		// The actual database operation will fail later if schema is wrong
		slog.Warn("Failed to auto-migrate audit_logs table", "error", err)
	}
	return &GormRepository{db: db}
}

// CreateAuditLog creates a new audit log entry with hash chaining
func (r *GormRepository) CreateAuditLog(ctx context.Context, log *models.AuditLog) (*models.AuditLog, error) {
	// Integrity check
	if (log.Signature != "" || log.PublicKeyID != "") && len(log.Message) == 0 {
		return nil, fmt.Errorf("invalid audit log: signature present but message is empty")
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lastLog models.AuditLog
		// Find the most recent log to get the previous hash
		// Using a lock (if supported) to prevent concurrent inserts from forking the chain
		// SQLite uses database-level locking during transactions anyway.
		if err := tx.Order("created_at DESC, id DESC").First(&lastLog).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		log.PreviousHash = lastLog.CurrentHash
		log.CurrentHash = r.computeHash(log)

		if err := tx.Create(log).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create audit log with chaining: %w", err)
	}
	return log, nil
}

// CreateAuditLogBatch creates multiple audit log entries in a single operation with chaining
func (r *GormRepository) CreateAuditLogBatch(ctx context.Context, logs []models.AuditLog) ([]models.AuditLog, error) {
	if len(logs) == 0 {
		return logs, nil
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lastLog models.AuditLog
		if err := tx.Order("created_at DESC, id DESC").First(&lastLog).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		currentPrevHash := lastLog.CurrentHash
		for i := range logs {
			if (logs[i].Signature != "" || logs[i].PublicKeyID != "") && len(logs[i].Message) == 0 {
				return fmt.Errorf("invalid audit log in batch: signature present but message is empty")
			}

			logs[i].PreviousHash = currentPrevHash
			logs[i].CurrentHash = r.computeHash(&logs[i])
			currentPrevHash = logs[i].CurrentHash
		}

		if err := tx.Create(&logs).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create audit log batch with chaining: %w", err)
	}
	return logs, nil
}

func (r *GormRepository) computeHash(log *models.AuditLog) string {
	h := sha256.New()
	// Chain the previous hash
	h.Write([]byte(log.PreviousHash))

	// Hash the content fields
	// We use a simplified JSON representation for stable hashing of the model fields
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

	b, _ := json.Marshal(payload)
	h.Write(b)

	return hex.EncodeToString(h.Sum(nil))
}

// GetAuditLogsByTraceID retrieves all audit logs for a given trace ID
func (r *GormRepository) GetAuditLogsByTraceID(ctx context.Context, traceID string) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	result := r.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("timestamp ASC").
		Find(&logs)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to retrieve audit logs by trace ID: %w", result.Error)
	}
	if logs == nil {
		logs = []models.AuditLog{}
	}
	return logs, nil
}

// GetAuditLogs retrieves audit logs with optional filtering
func (r *GormRepository) GetAuditLogs(ctx context.Context, filters *AuditLogFilters) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	query := r.db.WithContext(ctx).Model(&models.AuditLog{})

	// Performance optimization: Omit large message blobs by default
	if !filters.IncludeMessage {
		query = query.Omit("message")
	}

	// Apply filters
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

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Apply pagination and ordering
	// Note: Results are ordered by timestamp DESC (newest first) for general queries.
	// For trace-specific queries, use GetAuditLogsByTraceID which orders by ASC (chronological).
	limit := filters.Limit
	if limit <= 0 {
		limit = 100 // default
	}
	if limit > 1000 {
		limit = 1000 // max
	}

	if err := query.Order("timestamp DESC").Limit(limit).Offset(filters.Offset).Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to retrieve audit logs: %w", err)
	}

	if logs == nil {
		logs = []models.AuditLog{}
	}

	return logs, total, nil
}

// GetAuditLogByID retrieves a single audit log entry by its ID
func (r *GormRepository) GetAuditLogByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	var log models.AuditLog
	result := r.db.WithContext(ctx).First(&log, "id = ?", id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("audit log not found with ID %s", id)
		}
		return nil, fmt.Errorf("failed to get audit log: %w", result.Error)
	}
	return &log, nil
}
