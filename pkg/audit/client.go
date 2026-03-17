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
	SignatureAlgorithm string        // e.g., "RS256", "EdDSA"
	WorkerCount        int           // Number of background workers, defaults to 5
	QueueSize          int           // Size of the internal channel, defaults to 100
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

	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = 5
	}

	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 100
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
	}

	// Start background workers
	for i := 0; i < workerCount; i++ {
		c.wg.Add(1)
		go c.worker()
	}

	slog.Info("Audit client initialized with async workers",
		"baseURL", cfg.BaseURL,
		"workers", workerCount,
		"queueSize", queueSize)

	return c
}

// IsEnabled returns whether the audit client is enabled
func (c *Client) IsEnabled() bool {
	return c.enabled
}

// LogEvent sends an audit event to the audit service asynchronously via worker queue.
func (c *Client) LogEvent(ctx context.Context, event *AuditLogRequest) error {
	// Skip if audit client is not enabled
	if !c.enabled {
		return nil
	}

	// Push to queue
	select {
	case c.queue <- event:
		return nil
	default:
		slog.Warn("Audit queue full, dropping event", "eventType", event.EventType)
		return fmt.Errorf("audit queue full")
	}
}

// LogSignedEvent logs an audit event that has already been signed.
// This is an alias for LogEvent intended for semantically clearer logging of signed events.
func (c *Client) LogSignedEvent(ctx context.Context, event *AuditLogRequest) error {
	return c.LogEvent(ctx, event)
}

// SignEvent generates a cryptographic signature for the given request
// using the registered SignPayloadFunc.
func (c *Client) SignEvent(ctx context.Context, event *AuditLogRequest, keyID string) error {
	if c.signer == nil {
		return fmt.Errorf("no signer registered with the client")
	}

	payload, err := CanonicalizeRequest(event)
	if err != nil {
		return fmt.Errorf("failed to canonicalize event: %w", err)
	}

	sigBase64, err := c.signer(ctx, payload)
	if err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}

	event.Signature = sigBase64
	event.SignatureAlgorithm = c.signatureAlgorithm
	event.PublicKeyID = keyID

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
	for {
		select {
		case event, ok := <-c.queue:
			if !ok {
				return
			}

			// Create a context with timeout for this specific event's processing
			// Using the HTTP client's timeout as the base
			ctx, cancel := context.WithTimeout(context.Background(), c.httpClient.Timeout)

			// Automatic signing if required
			if event.ShouldSign {
				if err := c.SignEvent(ctx, event, c.publicKeyID); err != nil {
					slog.Error("Failed to sign event in worker", "error", err)
					cancel()
					continue
				}
			}
			c.logEvent(ctx, event)
			cancel()
		case <-c.quit:
			return
		}
	}
}

// logEvent sends the audit event to the audit service API
func (c *Client) logEvent(ctx context.Context, event *AuditLogRequest) {
	if c.httpClient == nil {
		return
	}

	payloadBytes, err := json.Marshal(event)
	if err != nil {
		slog.Error("Failed to marshal audit request", "error", err)
		return
	}

	// Construct URL safely
	endpointURL, err := url.JoinPath(c.baseURL, AuditLogsEndpoint)
	if err != nil {
		slog.Error("Failed to construct audit service URL", "error", err, "baseURL", c.baseURL)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		slog.Error("Failed to create audit request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("Failed to send audit request", "error", err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("Failed to close audit response body", "error", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			slog.Error("Audit service returned non-201 status and failed to read body",
				"status", resp.StatusCode, "readError", readErr)
		} else {
			slog.Error("Audit service returned non-201 status",
				"status", resp.StatusCode, "body", string(bodyBytes))
		}
		return
	}

	slog.Info("Audit event logged successfully",
		"eventType", event.EventType,
		"actorType", event.ActorType,
		"actorId", event.ActorID,
		"targetType", event.TargetType,
		"status", event.Status,
		"additionalMetadata", string(event.AdditionalMetadata))
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
