package services

import (
	"context"
	"testing"
	"time"

	v1models "github.com/LSFLK/argus/internal/api/v1/models"
	v1testutil "github.com/LSFLK/argus/internal/api/v1/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditService_CreateAuditLog(t *testing.T) {
	v1testutil.SetupTestEnums()
	mockRepo := v1testutil.NewMockRepository()
	service := NewAuditService(mockRepo)

	tests := []struct {
		name    string
		req     *v1models.CreateAuditLogRequest
		wantErr bool
	}{
		{
			name: "Valid request with SERVICE actor",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Status:     v1models.StatusSuccess,
				ActorType:  "SERVICE",
				ActorID:    "service-a",
				TargetType: "SERVICE",
				TargetID:   v1testutil.StringPtr("service-b"),
				EventType:  v1testutil.StringPtr("MANAGEMENT_EVENT"),
			},
			wantErr: false,
		},
		{
			name: "Valid request with ADMIN actor",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				Status:      v1models.StatusSuccess,
				ActorType:   "ADMIN",
				ActorID:     "admin@example.com",
				TargetType:  "RESOURCE",
				TargetID:    v1testutil.StringPtr("user-123"),
				EventAction: v1testutil.StringPtr("CREATE"),
			},
			wantErr: false,
		},
		{
			name: "Valid request with trace ID",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Status:     v1models.StatusSuccess,
				ActorType:  "SERVICE",
				ActorID:    "service-1",
				TargetType: "SERVICE",
				TargetID:   v1testutil.StringPtr("service-2"),
				TraceID:    v1testutil.StringPtr(uuid.New().String()),
			},
			wantErr: false,
		},
		{
			name: "Invalid actor type",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Status:     v1models.StatusSuccess,
				ActorType:  "INVALID",
				ActorID:    "actor-1",
				TargetType: "SERVICE",
				TargetID:   v1testutil.StringPtr("service-1"),
			},
			wantErr: true,
		},
		{
			name: "Missing actor ID",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Status:     v1models.StatusSuccess,
				ActorType:  "SERVICE",
				ActorID:    "",
				TargetType: "SERVICE",
				TargetID:   v1testutil.StringPtr("service-1"),
			},
			wantErr: true,
		},
		{
			name: "Invalid event type",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Status:     v1models.StatusSuccess,
				ActorType:  "SERVICE",
				ActorID:    "service-1",
				TargetType: "SERVICE",
				TargetID:   v1testutil.StringPtr("service-2"),
				EventType:  v1testutil.StringPtr("INVALID_EVENT"),
			},
			wantErr: true,
		},
		{
			name: "Missing timestamp",
			req: &v1models.CreateAuditLogRequest{
				Status:     v1models.StatusSuccess,
				ActorType:  "SERVICE",
				ActorID:    "service-1",
				TargetType: "SERVICE",
				TargetID:   v1testutil.StringPtr("service-1"),
			},
			wantErr: true,
		},
		{
			name: "Invalid timestamp format",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:  "invalid-timestamp",
				Status:     v1models.StatusSuccess,
				ActorType:  "SERVICE",
				ActorID:    "service-1",
				TargetType: "SERVICE",
				TargetID:   v1testutil.StringPtr("service-1"),
			},
			wantErr: true,
		},
		{
			name: "Invalid target type",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Status:     v1models.StatusSuccess,
				ActorType:  "SERVICE",
				ActorID:    "service-1",
				TargetType: "INVALID",
				TargetID:   v1testutil.StringPtr("service-1"),
			},
			wantErr: true,
		},
		{
			name: "Invalid event action",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				Status:      v1models.StatusSuccess,
				ActorType:   "SERVICE",
				ActorID:     "service-1",
				TargetType:  "SERVICE",
				TargetID:    v1testutil.StringPtr("service-1"),
				EventAction: v1testutil.StringPtr("INVALID_ACTION"),
			},
			wantErr: true,
		},
		{
			name: "Invalid status",
			req: &v1models.CreateAuditLogRequest{
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Status:     "INVALID_STATUS",
				ActorType:  "SERVICE",
				ActorID:    "service-1",
				TargetType: "SERVICE",
				TargetID:   v1testutil.StringPtr("service-1"),
			},
			wantErr: true,
		},
		{
			name: "Missing required fields - targetType",
			req: &v1models.CreateAuditLogRequest{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Status:    v1models.StatusSuccess,
				ActorType: "SERVICE",
				ActorID:   "service-1",
				// Missing TargetType
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := service.CreateAuditLog(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err, "Expected validation error")
				assert.Nil(t, log)
			} else {
				assert.NoError(t, err, "Expected no validation error")
				assert.NotNil(t, log)
				assert.NotEmpty(t, log.ID)
				assert.Equal(t, tt.req.Status, log.Status)
				assert.Equal(t, tt.req.ActorType, log.ActorType)
				assert.Equal(t, tt.req.ActorID, log.ActorID)
			}
		})
	}
}

func TestAuditService_GetAuditLogs(t *testing.T) {
	mockRepo := v1testutil.NewMockRepository()
	service := NewAuditService(mockRepo)

	tests := []struct {
		name      string
		traceID   *string
		eventType *string
		limit     int
		offset    int
		wantErr   bool
	}{
		{
			name:    "Get all logs",
			traceID: nil,
			limit:   10,
			offset:  0,
			wantErr: false,
		},
		{
			name:    "Get logs by trace ID",
			traceID: v1testutil.StringPtr(uuid.New().String()),
			limit:   10,
			offset:  0,
			wantErr: false,
		},
		{
			name:      "Get logs by event type",
			eventType: v1testutil.StringPtr("MANAGEMENT_EVENT"),
			limit:     10,
			offset:    0,
			wantErr:   false,
		},
		{
			name:    "Pagination",
			limit:   5,
			offset:  10,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs, total, err := service.GetAuditLogs(context.Background(), tt.traceID, tt.eventType, tt.limit, tt.offset)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logs)
				assert.GreaterOrEqual(t, total, int64(0))
			}
		})
	}
}

func TestAuditService_GetAuditLogsByTraceID(t *testing.T) {
	mockRepo := v1testutil.NewMockRepository()
	service := NewAuditService(mockRepo)

	traceID := uuid.New().String()

	logs, err := service.GetAuditLogsByTraceID(context.Background(), traceID)
	require.NoError(t, err)
	assert.NotNil(t, logs)
}

func TestAuditService_CreateAuditLog_InvalidTraceID(t *testing.T) {
	v1testutil.SetupTestEnums()
	mockRepo := v1testutil.NewMockRepository()
	service := NewAuditService(mockRepo)

	req := &v1models.CreateAuditLogRequest{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Status:     v1models.StatusSuccess,
		ActorType:  "SERVICE",
		ActorID:    "service-1",
		TargetType: "SERVICE",
		TargetID:   v1testutil.StringPtr("service-2"),
		TraceID:    v1testutil.StringPtr("invalid-uuid"),
	}

	_, err := service.CreateAuditLog(context.Background(), req)
	assert.Error(t, err)
	assert.True(t, IsValidationError(err))
}
