package sinks

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/LSFLK/argus/internal/api/v1/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

// S3ClientAPI defines the S3 operations required by S3Sink.
// This allows us to mock the AWS S3 client easily during tests.
type S3ClientAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3SinkConfig holds configuration for S3Sink.
type S3SinkConfig struct {
	Bucket         string
	Region         string
	Prefix         string
	Endpoint       string // Useful for local testing (e.g., MinIO / LocalStack)
	UsePathStyle   bool   // Useful for local testing (e.g., MinIO)
	ObjectLockMode string // e.g., "COMPLIANCE" (default) or "GOVERNANCE"
	RetentionDays  int    // Retention period in days (e.g., 2555 days for 7 years)
}

// S3Sink implements the Sink interface using the AWS SDK v2 for S3.
// It uploads signed audit logs directly to S3 with WORM (Object Lock Compliance Mode) enabled.
type S3Sink struct {
	client S3ClientAPI
	cfg    S3SinkConfig
}

// NewS3Sink creates a new S3Sink using the provided configuration.
// If s3Client is nil, it initializes a new client using standard AWS SDK v2 configuration.
func NewS3Sink(ctx context.Context, cfg S3SinkConfig, s3Client S3ClientAPI) (*S3Sink, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket name is required")
	}

	// Set default object lock mode to COMPLIANCE
	if cfg.ObjectLockMode == "" {
		cfg.ObjectLockMode = "COMPLIANCE"
	}

	// Set default retention days if not specified (e.g. 7 years/2555 days)
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 2555
	}

	if s3Client == nil {
		var optFns []func(*config.LoadOptions) error
		if cfg.Region != "" {
			optFns = append(optFns, config.WithRegion(cfg.Region))
		}

		awsCfg, err := config.LoadDefaultConfig(ctx, optFns...)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS default config: %w", err)
		}

		s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			if cfg.Endpoint != "" {
				o.BaseEndpoint = aws.String(cfg.Endpoint)
			}
			if cfg.UsePathStyle {
				o.UsePathStyle = true
			}
		})
	}

	return &S3Sink{
		client: s3Client,
		cfg:    cfg,
	}, nil
}

// Name returns the unique identifier for this sink.
func (s *S3Sink) Name() string {
	return "S3Sink"
}

// Write persists a single audit log to the S3 bucket with Object Lock properties.
func (s *S3Sink) Write(ctx context.Context, log *models.AuditLog) error {
	if log == nil {
		return nil
	}

	data, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal audit log: %w", err)
	}

	key := s.generateKey(log.Timestamp, false, log.ID)
	return s.upload(ctx, key, data, "application/json")
}

// WriteBatch persists multiple audit logs as a Newline Delimited JSON (NDJSON) file
// to the S3 bucket with Object Lock properties.
func (s *S3Sink) WriteBatch(ctx context.Context, logs []models.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	for i := range logs {
		if err := encoder.Encode(logs[i]); err != nil {
			return fmt.Errorf("failed to marshal batch audit log at index %d: %w", i, err)
		}
	}

	// Use the timestamp of the first log for partitioning
	key := s.generateKey(logs[0].Timestamp, true, uuid.New())
	return s.upload(ctx, key, buf.Bytes(), "application/x-ndjson")
}

// Close is a no-op for S3Sink.
func (s *S3Sink) Close() error {
	return nil
}

// generateKey constructs a partitioned S3 key based on the log's timestamp.
func (s *S3Sink) generateKey(timestamp time.Time, isBatch bool, id uuid.UUID) string {
	prefix := s.cfg.Prefix
	if prefix != "" && prefix[len(prefix)-1] != '/' {
		prefix = prefix + "/"
	}

	// Partitioning format: prefix/YYYY/MM/DD
	datePart := timestamp.UTC().Format("2006/01/02")

	if isBatch {
		return fmt.Sprintf("%s%s/batch-%s.ndjson", prefix, datePart, id)
	}
	return fmt.Sprintf("%s%s/%s.json", prefix, datePart, id)
}

// upload performs the PUT request to S3, applying Object Lock headers and transport validation.
func (s *S3Sink) upload(ctx context.Context, key string, data []byte, contentType string) error {
	// Compute MD5 for transport integrity (required/strongly recommended by S3 Object Lock)
	hash := md5.Sum(data)
	contentMD5 := base64.StdEncoding.EncodeToString(hash[:])

	// Calculate object lock retain-until date
	retainUntil := time.Now().UTC().AddDate(0, 0, s.cfg.RetentionDays)

	input := &s3.PutObjectInput{
		Bucket:                    aws.String(s.cfg.Bucket),
		Key:                       aws.String(key),
		Body:                      bytes.NewReader(data),
		ContentType:               aws.String(contentType),
		ContentMD5:                aws.String(contentMD5),
		ObjectLockMode:            types.ObjectLockMode(s.cfg.ObjectLockMode),
		ObjectLockRetainUntilDate: aws.Time(retainUntil),
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload object to S3: %w", err)
	}

	slog.Debug("Uploaded audit log to S3 compliance bucket", "key", key, "bucket", s.cfg.Bucket, "size", len(data))
	return nil
}
