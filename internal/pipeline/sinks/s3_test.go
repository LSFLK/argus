package sinks

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockS3Client implements S3ClientAPI for testing.
type MockS3Client struct {
	PutObjectFunc func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

func (m *MockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.PutObjectFunc != nil {
		return m.PutObjectFunc(ctx, params, optFns...)
	}
	return &s3.PutObjectOutput{}, nil
}

func TestS3Sink_WriteSingle(t *testing.T) {
	mockClient := &MockS3Client{}
	cfg := S3SinkConfig{
		Bucket:         "test-audit-compliance-bucket",
		Region:         "us-east-1",
		Prefix:         "argus-logs/",
		ObjectLockMode: "COMPLIANCE",
		RetentionDays:  30,
	}

	sink, err := NewS3Sink(context.Background(), cfg, mockClient)
	require.NoError(t, err)

	logTime := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	logID := uuid.New()
	log := &models.AuditLog{
		ID:        logID,
		Action:    "CREATE",
		ActorID:   "actor-1",
		Timestamp: logTime,
	}

	putCalled := false
	mockClient.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
		putCalled = true

		assert.Equal(t, "test-audit-compliance-bucket", *params.Bucket)
		assert.Equal(t, "argus-logs/2026/05/06/"+logID.String()+".json", *params.Key)
		assert.Equal(t, "application/json", *params.ContentType)
		assert.NotEmpty(t, *params.ContentMD5)
		assert.Equal(t, "COMPLIANCE", string(params.ObjectLockMode))
		assert.NotNil(t, params.ObjectLockRetainUntilDate)

		// Verify retain-until date is in the future
		assert.True(t, params.ObjectLockRetainUntilDate.After(time.Now().UTC()))

		// Verify body content
		var decodedLog models.AuditLog
		err := json.NewDecoder(params.Body).Decode(&decodedLog)
		require.NoError(t, err)
		assert.Equal(t, log.ID, decodedLog.ID)
		assert.Equal(t, log.ActorID, decodedLog.ActorID)

		return &s3.PutObjectOutput{}, nil
	}

	err = sink.Write(context.Background(), log)
	require.NoError(t, err)
	assert.True(t, putCalled, "PutObject should have been called")
}

func TestS3Sink_WriteBatchNDJSON(t *testing.T) {
	mockClient := &MockS3Client{}
	cfg := S3SinkConfig{
		Bucket:         "test-audit-compliance-bucket",
		Region:         "us-east-1",
		Prefix:         "argus-logs/",
		ObjectLockMode: "COMPLIANCE",
		RetentionDays:  365,
	}

	sink, err := NewS3Sink(context.Background(), cfg, mockClient)
	require.NoError(t, err)

	logTime := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	logs := []models.AuditLog{
		{
			ID:        uuid.New(),
			Action:    "CREATE",
			ActorID:   "actor-1",
			Timestamp: logTime,
		},
		{
			ID:        uuid.New(),
			Action:    "UPDATE",
			ActorID:   "actor-2",
			Timestamp: logTime,
		},
	}

	putCalled := false
	mockClient.PutObjectFunc = func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
		putCalled = true

		assert.Equal(t, "test-audit-compliance-bucket", *params.Bucket)
		// Batch prefix pattern
		assert.True(t, strings.HasPrefix(*params.Key, "argus-logs/2026/05/06/batch-"))
		assert.True(t, strings.HasSuffix(*params.Key, ".ndjson"))
		assert.Equal(t, "application/x-ndjson", *params.ContentType)
		assert.NotEmpty(t, *params.ContentMD5)
		assert.Equal(t, "COMPLIANCE", string(params.ObjectLockMode))

		// Verify body is NDJSON (two lines of JSON)
		var buf bytes.Buffer
		_, err := buf.ReadFrom(params.Body)
		require.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		assert.Len(t, lines, 2)

		var log1, log2 models.AuditLog
		err = json.Unmarshal([]byte(lines[0]), &log1)
		require.NoError(t, err)
		err = json.Unmarshal([]byte(lines[1]), &log2)
		require.NoError(t, err)

		assert.Equal(t, logs[0].ID, log1.ID)
		assert.Equal(t, logs[1].ID, log2.ID)

		return &s3.PutObjectOutput{}, nil
	}

	err = sink.WriteBatch(context.Background(), logs)
	require.NoError(t, err)
	assert.True(t, putCalled, "PutObject should have been called")
}

func TestS3Sink_ConfigurationDefaults(t *testing.T) {
	// Missing bucket should error
	_, err := NewS3Sink(context.Background(), S3SinkConfig{}, &MockS3Client{})
	assert.Error(t, err)

	// Valid config should populate defaults
	sink, err := NewS3Sink(context.Background(), S3SinkConfig{
		Bucket: "test-bucket",
	}, &MockS3Client{})
	require.NoError(t, err)
	assert.Equal(t, "COMPLIANCE", sink.cfg.ObjectLockMode)
	assert.Equal(t, 2555, sink.cfg.RetentionDays) // 7 years default
}
