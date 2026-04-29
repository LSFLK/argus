package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LSFLK/argus/internal/api/v1/database"
	v1models "github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/google/uuid"
)

// AuditService handles generalized audit log operations
type AuditService struct {
	repo database.AuditRepository
}

// NewAuditService creates a new audit service instance using the database repository
func NewAuditService(repo database.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

// CreateAuditLog creates a new audit log entry from a request
func (s *AuditService) CreateAuditLog(ctx context.Context, req *v1models.CreateAuditLogRequest) (*v1models.AuditLog, error) {
	// Marshal metadata to JSONB
	metaBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to marshal metadata: %w", ErrValidation, err)
	}

	// Convert request to model
	auditLog := &v1models.AuditLog{
		EventType:          req.EventType,
		Action:             req.Action,
		Status:             req.Status,
		ActorType:          req.ActorType,
		ActorID:            req.ActorID,
		TargetType:         req.TargetType,
		TargetID:           req.TargetID,
		Message:            v1models.JSONBRawMessage(req.Message),
		Metadata:           v1models.JSONBRawMessage(metaBytes),
		Signature:          req.Signature,
		SignatureAlgorithm: req.SignatureAlgorithm,
		PublicKeyID:        req.PublicKeyID,
	}

	// Parse and validate timestamp (required)
	timestamp, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid timestamp format, expected RFC3339: %w", ErrValidation, err)
	}
	auditLog.Timestamp = timestamp.UTC()

	// Handle trace ID
	if req.TraceID != nil && *req.TraceID != "" {
		traceUUID, err := uuid.Parse(*req.TraceID)
		if err != nil {
			// Wrap as domain validation error
			return nil, fmt.Errorf("%w: invalid traceId format: %w", ErrValidation, err)
		}
		auditLog.TraceID = &traceUUID
	}

	// Validate before creating
	if err := auditLog.Validate(); err != nil {
		// All validation errors from the model are treated as domain validation errors
		// This abstracts away the implementation details (GORM or other validation)
		return nil, fmt.Errorf("%w: %w", ErrValidation, err)
	}

	// Create in database using repository
	createdLog, err := s.repo.CreateAuditLog(ctx, auditLog)
	if err != nil {
		return nil, err
	}

	return createdLog, nil
}

// GetAuditLogs retrieves audit logs with optional filtering
func (s *AuditService) GetAuditLogs(ctx context.Context, traceID *string, eventType *string, limit, offset int, includeMessage bool) ([]v1models.AuditLog, int64, error) {
	filters := &database.AuditLogFilters{
		TraceID:        traceID,
		EventType:      eventType,
		Limit:          limit,
		Offset:         offset,
		IncludeMessage: includeMessage,
	}

	return s.repo.GetAuditLogs(ctx, filters)
}

// GetAuditLogByID retrieves a single audit log entry by its ID
func (s *AuditService) GetAuditLogByID(ctx context.Context, id uuid.UUID) (*v1models.AuditLog, error) {
	log, err := s.repo.GetAuditLogByID(ctx, id)
	if err != nil {
		// Map repository errors to domain errors
		if strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		return nil, err
	}
	return log, nil
}

// GetAuditLogsByTraceID retrieves audit logs by trace ID (convenience method)
func (s *AuditService) GetAuditLogsByTraceID(ctx context.Context, traceID string) ([]v1models.AuditLog, error) {
	return s.repo.GetAuditLogsByTraceID(ctx, traceID)
}
