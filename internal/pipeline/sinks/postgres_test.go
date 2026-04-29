package sinks

import (
	"context"
	"testing"
	"time"

	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPostgresSink_HashChaining(t *testing.T) {
	// Setup in-memory SQLite for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	sink := NewPostgresSink(db)

	ctx := context.Background()

	// 1. Write the first log
	log1 := &models.AuditLog{
		Action:    "CREATE",
		ActorID:   "user-1",
		Timestamp: time.Now().UTC(),
	}
	err = sink.Write(ctx, log1)
	require.NoError(t, err)
	assert.Empty(t, log1.PreviousHash, "First log should have no previous hash")
	assert.NotEmpty(t, log1.CurrentHash, "First log should have a current hash")

	// 2. Write a second log
	log2 := &models.AuditLog{
		Action:    "UPDATE",
		ActorID:   "user-2",
		Timestamp: time.Now().UTC(),
	}
	err = sink.Write(ctx, log2)
	require.NoError(t, err)
	assert.Equal(t, log1.CurrentHash, log2.PreviousHash, "Second log should link to the first one")
	assert.NotEmpty(t, log2.CurrentHash)
	assert.NotEqual(t, log1.CurrentHash, log2.CurrentHash)

	// 3. Write a third log
	log3 := &models.AuditLog{
		Action:    "DELETE",
		ActorID:   "user-3",
		Timestamp: time.Now().UTC(),
	}
	err = sink.Write(ctx, log3)
	require.NoError(t, err)
	assert.Equal(t, log2.CurrentHash, log3.PreviousHash, "Third log should link to the second one")
}

func TestPostgresSink_ReadMethods(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	sink := NewPostgresSink(db)
	ctx := context.Background()

	log := &models.AuditLog{
		Action:    "READ_TEST",
		ActorID:   "tester",
		Timestamp: time.Now().UTC(),
	}
	_ = sink.Write(ctx, log)

	// Test GetAuditLogByID
	found, err := sink.GetAuditLogByID(ctx, log.ID)
	require.NoError(t, err)
	assert.Equal(t, log.Action, found.Action)

	// Test GetAuditLogs (list)
	logs, total, err := sink.GetAuditLogs(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, logs, 1)
}
