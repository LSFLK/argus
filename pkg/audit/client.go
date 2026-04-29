package audit

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// AuditLogsEndpoint is the API endpoint for creating audit logs
	AuditLogsEndpoint = "/api/audit-logs"
	// DefaultHTTPTimeout is the default timeout for HTTP requests to the audit service
	DefaultHTTPTimeout = 10 * time.Second
)

// Config defines the configuration for the Audit Client
type Config struct {
	BaseURL            string
	Signer             SignPayloadFunc
	PublicKeyID        string
	SignatureAlgorithm string        // e.g. "RS256", "EdDSA"
	WorkerCount        int           // Number of background workers, defaults to 5
	QueueSize          int           // Size of the internal channel, defaults to 100
	BatchSize          int           // Number of logs to send in one batch, defaults to 20
	BatchInterval      time.Duration // Max time to wait before sending a batch, defaults to 1s
	HTTPTimeout        time.Duration // Defaults to 10s
}

// Client is a client for sending audit events to the audit service
type Client struct {
	baseURL            string
	httpClient         *http.Client
	enabled            bool
	signer             SignPayloadFunc
	publicKeyID        string
	signatureAlgorithm string
	queue              chan *AuditLogRequest
	quit               chan struct{}
	wg                 sync.WaitGroup
	batchSize          int
	batchInterval      time.Duration
}

// NewClient creates a new audit client using the provided configuration.
// Audit can be disabled by:
//   - Setting ENABLE_AUDIT=false environment variable
//   - Providing an empty baseURL in config
//
// When disabled, all LogEvent calls will be no-ops.
func NewClient(cfg Config) *Client {
	enabled := isAuditEnabled(cfg.BaseURL)

	if !enabled {
		slog.Info("Audit client disabled",
			"reason", "ENABLE_AUDIT=false or audit service URL not configured",
			"impact", "Services will continue running but audit events will not be logged")
		return &Client{
			enabled: false,
		}
	}

	// Algorithm Hardening: Validate SignatureAlgorithm if a signer is provided
	if cfg.Signer != nil {
		switch cfg.SignatureAlgorithm {
		case "RS256", "EdDSA":
			// Valid
		default:
			slog.Error("Unsupported signature algorithm", "algorithm", cfg.SignatureAlgorithm)
			return &Client{
				enabled: false,
			}
		}
	}

	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = 5
	}

	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 100
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}

	batchInterval := cfg.BatchInterval
	if batchInterval <= 0 {
		batchInterval = 1 * time.Second
	}

	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}

	c := &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
			},
		},
		enabled:            true,
		signer:             cfg.Signer,
		publicKeyID:        cfg.PublicKeyID,
		signatureAlgorithm: cfg.SignatureAlgorithm,
		queue:              make(chan *AuditLogRequest, queueSize),
		quit:               make(chan struct{}),
		batchSize:          batchSize,
		batchInterval:      batchInterval,
	}

	// Start background workers
	for i := 0; i < workerCount; i++ {
		c.wg.Add(1)
		go c.worker()
	}

	slog.Info("Audit client initialized with async workers and batching",
		"baseURL", cfg.BaseURL,
		"workers", workerCount,
		"batchSize", batchSize,
		"batchInterval", batchInterval,
		"queueSize", queueSize)

	return c
}

// IsEnabled returns whether the audit client is enabled
func (c *Client) IsEnabled() bool {
	return c.enabled
}

// LogEvent sends an audit event to the audit service asynchronously via worker queue.
func (c *Client) LogEvent(ctx context.Context, event *AuditLogRequest) {
	// Skip if audit client is not enabled
	if !c.enabled {
		return
	}

	// Push to queue
	select {
	case c.queue <- event:
		return
	default:
		slog.Warn("Audit queue full, dropping event", "action", event.Action)
	}
}

// LogSignedEvent logs an audit event that has already been signed.
// This is an alias for LogEvent intended for semantically clearer logging of signed events.
func (c *Client) LogSignedEvent(ctx context.Context, event *AuditLogRequest) {
	c.LogEvent(ctx, event)
}

// SignEvent generates a cryptographic signature for the given request
// using the registered SignPayloadFunc.
func (c *Client) SignEvent(event *AuditLogRequest) error {
	if c.signer == nil {
		return fmt.Errorf("no signer registered with the client")
	}

	payload, err := CanonicalizeRequest(event)
	if err != nil {
		return fmt.Errorf("failed to canonicalize event: %w", err)
	}

	// Using context.Background() for the signing callback as the original caller's context
	// may have expired if called from the background worker.
	sigBase64, err := c.signer(context.Background(), payload)
	if err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}

	event.Signature = sigBase64
	event.SignatureAlgorithm = c.signatureAlgorithm
	event.PublicKeyID = c.publicKeyID

	return nil
}

// Close gracefully shuts down the client, flushing the queue.
func (c *Client) Close(ctx context.Context) error {
	if !c.enabled {
		return nil
	}
	close(c.quit)
	close(c.queue)

	// Wait for workers to finish, but honor context timeout if provided
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// VerifyIntegrity verifies the signature of a log request.
// It uses the public key provided by the caller to verify the signature.
func (c *Client) VerifyIntegrity(event *AuditLogRequest, publicKey crypto.PublicKey) (bool, error) {
	if event.Signature == "" {
		return false, fmt.Errorf("event has no signature")
	}

	payload, err := CanonicalizeRequest(event)
	if err != nil {
		return false, fmt.Errorf("failed to canonicalize event: %w", err)
	}

	err = VerifyPayload(payload, event.Signature, event.SignatureAlgorithm, publicKey)
	if err != nil {
		return false, fmt.Errorf("verification failed: %w", err)
	}

	return true, nil
}

func (c *Client) worker() {
	defer c.wg.Done()

	buffer := make([]*AuditLogRequest, 0, c.batchSize)
	ticker := time.NewTicker(c.batchInterval)
	defer ticker.Stop()

	flush := func() {
		if len(buffer) == 0 {
			return
		}
		// Create a context with timeout for the batch
		ctx, cancel := context.WithTimeout(context.Background(), c.httpClient.Timeout)
		defer cancel()

		c.logBatch(ctx, buffer)
		buffer = make([]*AuditLogRequest, 0, c.batchSize)
	}

	for {
		select {
		case event, ok := <-c.queue:
			if !ok {
				flush()
				return
			}

			// Automatic signing if required
			if event.ShouldSign {
				var signErr error
				for attempt := 1; attempt <= 3; attempt++ {
					if signErr = c.SignEvent(event); signErr == nil {
						break
					}
					slog.Warn("Failed to sign event in worker, retrying",
						"attempt", attempt,
						"maxAttempts", 3,
						"error", signErr)
					time.Sleep(100 * time.Millisecond)
				}

				if signErr != nil {
					slog.Error("Failed to sign event in worker after retries, dropping event", "error", signErr)
					continue
				}
			}

			buffer = append(buffer, event)
			if len(buffer) >= c.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-c.quit:
			flush()
			return
		}
	}
}

// logBatch sends a batch of audit events to the audit service API
func (c *Client) logBatch(ctx context.Context, events []*AuditLogRequest) {
	if c.httpClient == nil || len(events) == 0 {
		return
	}

	// For now, we'll use a new bulk endpoint if it exists, or just loop if not.
	// But the requirement says "send to the server in bulk rather than via individual HTTP requests".
	// So I should implement the bulk endpoint on the server.
	payloadBytes, err := json.Marshal(events)
	if err != nil {
		slog.Error("Failed to marshal audit batch request", "error", err)
		return
	}

	// Construct URL safely - use /bulk endpoint
	endpointURL, err := url.JoinPath(c.baseURL, AuditLogsEndpoint, "bulk")
	if err != nil {
		slog.Error("Failed to construct audit service bulk URL", "error", err, "baseURL", c.baseURL)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(payloadBytes))
	if err != nil {
		slog.Error("Failed to create audit batch request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to send audit batch request", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("Audit service returned error for batch",
			"status", resp.StatusCode, "body", string(bodyBytes))
		return
	}

	slog.Info("Audit batch logged successfully", "count", len(events))
}

// logEvent sends a single audit event to the audit service API
// Deprecated: Use logBatch for production. Kept for single event logging if needed.
func (c *Client) logEvent(ctx context.Context, event *AuditLogRequest) {
	c.logBatch(ctx, []*AuditLogRequest{event})
}

// isAuditEnabled checks if audit logging is enabled via environment variable
// Audit is enabled by default unless explicitly disabled via ENABLE_AUDIT=false
// or if baseURL is empty
func isAuditEnabled(baseURL string) bool {
	// If URL is explicitly empty, audit is disabled
	if baseURL == "" {
		return false
	}

	// Check ENABLE_AUDIT environment variable (default: true)
	enableAudit := os.Getenv("ENABLE_AUDIT")
	if enableAudit == "" {
		// Default to enabled if URL is provided
		return true
	}

	// Parse boolean value (case-insensitive)
	enableAuditLower := strings.ToLower(strings.TrimSpace(enableAudit))
	return enableAuditLower == "true" || enableAuditLower == "1" || enableAuditLower == "yes"
}
