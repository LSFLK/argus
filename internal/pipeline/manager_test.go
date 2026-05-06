package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/LSFLK/argus/internal/api/v1/testutil"
	"github.com/stretchr/testify/assert"
)

func TestManager_Dispatch(t *testing.T) {
	sink1 := testutil.NewMockRepository()
	sink2 := testutil.NewMockRepository()

	// Create a failing sink
	failSink := &failingSink{name: "FailSink"}

	manager := NewManager(nil, sink1, sink2, failSink)

	log := &models.AuditLog{
		Action: "TEST_DISPATCH",
	}

	errs := manager.Dispatch(context.Background(), log)

	// Verify behavior
	assert.Len(t, errs, 1, "Should have exactly one error from FailSink")
	assert.Equal(t, "sink FailSink failed: dispatch error from FailSink", errs[0].Error())

	// Verify that successful sinks still received the log
	assert.Len(t, sink1.GetLogs(), 1)
	assert.Len(t, sink2.GetLogs(), 1)
	assert.Equal(t, "TEST_DISPATCH", sink1.GetLogs()[0].Action)
}

type failingSink struct {
	name string
}

func (s *failingSink) Name() string { return s.name }
func (s *failingSink) Write(ctx context.Context, log *models.AuditLog) error {
	return errors.New("dispatch error from " + s.name)
}
func (s *failingSink) WriteBatch(ctx context.Context, logs []models.AuditLog) error {
	return errors.New("dispatch batch error from " + s.name)
}
func (s *failingSink) Close() error { return nil }
