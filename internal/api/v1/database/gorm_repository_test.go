//go:build integration
// +build integration

package database

import (
	"context"
	"testing"
	"time"

	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/LSFLK/argus/internal/config"
	"github.com/LSFLK/argus/internal/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	config := &database.Config{
		Type:            database.DatabaseTypeSQLite,
		DatabasePath:    ":memory:",
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 15 * time.Minute,
	}

	db, err := database.ConnectGormDB(config)
	require.NoError(t, err)
	return db
}

func TestGormRepository_CreateAuditLog(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)

	enums := &config.AuditEnums{
		EventTypes:   []string{"MANAGEMENT_EVENT", "USER_MANAGEMENT"},
		EventActions: []string{"CREATE", "READ", "UPDATE", "DELETE"},
		ActorTypes:   []string{"SERVICE", "ADMIN", "MEMBER", "SYSTEM"},
		TargetTypes:  []string{"SERVICE", "RESOURCE"},
	}
	enums.InitializeMaps()
	models.SetEnumConfig(enums)

	tests := []struct {
		name    string
		log     *models.AuditLog
		wantErr bool
	}{
		{
			name: "Valid audit log",
			log: &models.AuditLog{
				Timestamp:  time.Now().UTC(),
				Status:     models.StatusSuccess,
				ActorType:  "SERVICE",
				ActorID:    "test-service",
				TargetType: "SERVICE",
				TargetID:   stringPtr("target-service"),
			},
			wantErr: false,
		},
		{
			name: "Audit log with trace ID",
			log: &models.AuditLog{
				Timestamp:  time.Now().UTC(),
				TraceID:    uuidPtr(uuid.New()),
				Status:     models.StatusSuccess,
				ActorType:  "SERVICE",
				ActorID:    "test-service",
				TargetType: "SERVICE",
				TargetID:   stringPtr("target-service"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.CreateAuditLog(context.Background(), tt.log)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEqual(t, uuid.Nil, result.ID)
			}
		})
	}
}

func TestGormRepository_GetAuditLogsByTraceID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)

	traceID := uuid.New()
	now := time.Now().UTC()

	// Create test logs
	log1 := &models.AuditLog{
		Timestamp:  now,
		TraceID:    &traceID,
		Status:     models.StatusSuccess,
		ActorType:  "SERVICE",
		ActorID:    "service-1",
		TargetType: "SERVICE",
		TargetID:   stringPtr("target-1"),
	}
	log2 := &models.AuditLog{
		Timestamp:  now.Add(time.Second),
		TraceID:    &traceID,
		Status:     models.StatusSuccess,
		ActorType:  "SERVICE",
		ActorID:    "service-2",
		TargetType: "SERVICE",
		TargetID:   stringPtr("target-2"),
	}

	_, err := repo.CreateAuditLog(context.Background(), log1)
	require.NoError(t, err)
	_, err = repo.CreateAuditLog(context.Background(), log2)
	require.NoError(t, err)

	logs, err := repo.GetAuditLogsByTraceID(context.Background(), traceID.String())
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}

func TestGormRepository_GetAuditLogs(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)

	now := time.Now().UTC()

	// Create test logs
	for i := 0; i < 5; i++ {
		log := &models.AuditLog{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			Status:     models.StatusSuccess,
			ActorType:  "SERVICE",
			ActorID:    "service-1",
			TargetType: "SERVICE",
			TargetID:   stringPtr("target-1"),
		}
		_, err := repo.CreateAuditLog(context.Background(), log)
		require.NoError(t, err)
	}

	filters := &AuditLogFilters{
		Limit:  2,
		Offset: 0,
	}

	logs, total, err := repo.GetAuditLogs(context.Background(), filters)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, logs, 2)
}

func stringPtr(s string) *string {
	return &s
}

func uuidPtr(u uuid.UUID) *uuid.UUID {
	return &u
}
