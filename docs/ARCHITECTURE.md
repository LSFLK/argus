# Architecture Overview

Understanding how Argus works and how to integrate it into your microservices architecture.

## Overview

Argus follows a clean, layered architecture designed for easy integration. The service provides both a REST API and a reusable Go interface (`pkg/audit`) that can be imported into any service. This document explains the architecture from an integration perspective.

## Integration Points

When integrating Argus into your architecture, you'll interact with:

1. **REST API** (`/api/audit-logs`) - HTTP endpoints for creating and querying audit logs
2. **Go Interface** (`pkg/audit`) - Reusable client package for Go services
3. **Configuration** (`configs/enums.yaml`) - Customize event types for your use case

## Project Structure

For developers integrating Argus, the key directories are:

```
argus/
├── pkg/audit/              # Import this package in your Go services
│   ├── client.go           # HTTP client implementation
│   ├── interface.go        # Auditor interface
│   └── models.go           # Request/response models
│
├── configs/                # Customize event types here
│   └── enums.yaml          # Define your event types
│
├── docs/                   # Integration documentation
│   ├── API.md              # REST API reference
│   ├── ARCHITECTURE.md     # This file
│   └── DATABASE_CONFIGURATION.md
│
└── cmd/argus/              # Service entry point (for deployment)
    └── main.go
```

**Note:** The `internal/` directory contains private implementation details. You don't need to import anything from `internal/` when integrating Argus.

## How Argus Works

### Integration Flow

When you integrate Argus into your services, here's what happens:

1. **Your Service** → Calls Argus API or uses `pkg/audit` client
2. **Argus Service** → Validates and stores audit log
3. **Database** → Persists audit log (SQLite or PostgreSQL)
4. **Query** → Your services can query audit logs via REST API

### API Layer

Argus exposes a RESTful API with the following endpoints:

- **POST `/api/audit-logs`** - Create audit log entries
- **GET `/api/audit-logs`** - Query audit logs with filtering

The API follows REST conventions:
- Uses standard HTTP methods (POST, GET)
- Resource-based URLs (`/api/audit-logs`)
- JSON request/response format
- Proper HTTP status codes

### Client Library

For Go services, Argus provides a reusable client package:

```go
import "github.com/LSFLK/argus/pkg/audit"

// Initialize client
client := audit.NewClient("http://argus-service:3001")

// Log events (asynchronous)
client.LogEvent(ctx, &audit.AuditLogRequest{...})
```

**Benefits:**
- Asynchronous logging (non-blocking)
- Graceful degradation (works even if Argus is unavailable)
- Clean interface for easy testing
- No tight coupling to Argus implementation

## Integration Patterns

### Pattern 1: Direct HTTP Integration

Use the REST API directly from any language or service:

```bash
curl -X POST http://argus-service:3001/api/audit-logs \
  -H "Content-Type: application/json" \
  -d '{...}'
```

**Use when:**
- Integrating non-Go services
- Need maximum flexibility
- Prefer explicit HTTP calls

### Pattern 2: Go Client Library

Use the `pkg/audit` package for Go services:

```go
import "github.com/LSFLK/argus/pkg/audit"

client := audit.NewClient("http://argus-service:3001")
client.LogEvent(ctx, &audit.AuditLogRequest{...})
```

**Use when:**
- Building Go services
- Want asynchronous, non-blocking logging
- Need graceful degradation

### Pattern 3: Global Middleware

Initialize audit logging once at application startup:

```go
auditClient := audit.NewClient(os.Getenv("ARGUS_SERVICE_URL"))
audit.InitializeGlobalAudit(auditClient)

// Use anywhere in your service
audit.LogAuditEvent(ctx, &audit.AuditLogRequest{...})
```

**Use when:**
- Multiple handlers need audit logging
- Want centralized configuration
- Prefer global access pattern

## Request Flow

### Creating Audit Logs

When your service sends an audit event to Argus:

```
Your Service
    ↓ (HTTP POST or pkg/audit client)
Argus API Handler
    ↓ (validates request)
Argus Service Layer
    ↓ (validates business rules)
Database Repository
    ↓ (persists to database)
Database (SQLite/PostgreSQL)
    ↓
Response (201 Created) → Your Service
```

**Key Points:**
- Validation happens at multiple layers
- Errors return appropriate HTTP status codes
- Database operations are transactional
- Response includes created audit log with ID

### Querying Audit Logs

When your service queries audit logs:

```
Your Service
    ↓ (HTTP GET with query params)
Argus API Handler
    ↓ (parses filters)
Argus Service Layer
    ↓ (builds query)
Database Repository
    ↓ (executes query)
Database (SQLite/PostgreSQL)
    ↓
Response (200 OK with logs) → Your Service
```

**Key Points:**
- Supports filtering by trace ID, event type, status
- Pagination via `limit` and `offset` parameters
- Returns total count for pagination calculations
- Results ordered by timestamp (newest first)

## Testing Your Integration

When integrating Argus, you can test your integration in several ways:

### 1. Unit Testing with Mocks

Mock the `pkg/audit` interface in your service tests:

```go
type MockAuditor struct {
    LogEventFunc func(ctx context.Context, event *audit.AuditLogRequest)
}

func (m *MockAuditor) LogEvent(ctx context.Context, event *audit.AuditLogRequest) {
    if m.LogEventFunc != nil {
        m.LogEventFunc(ctx, event)
    }
}

func (m *MockAuditor) IsEnabled() bool { return true }
```

### 2. Integration Testing

Run Argus locally and test against real API:

```bash
# Start Argus with in-memory database
go run ./cmd/argus

# Run your service tests that call Argus
go test ./...
```

### 3. End-to-End Testing

Deploy Argus in your test environment and verify audit logs are created:

```bash
# Query audit logs to verify your service is logging correctly
curl http://argus-test:3001/api/audit-logs?traceId=your-trace-id
```

## Customization

### Custom Event Types

Define your own event types in `configs/enums.yaml`:

```yaml
enums:
  eventTypes:
    - YOUR_CUSTOM_EVENT
    - ANOTHER_EVENT_TYPE
```

See [Configuration Guide](../internal/config/README.md) for details.

### Custom Auditor Implementation

Implement your own `Auditor` interface for custom behavior (e.g., batch logging, custom transport).

## Deployment Considerations

### Database Choice

- **SQLite (in-memory)**: Development and testing
- **SQLite (file-based)**: Single-server deployments
- **PostgreSQL**: Production, high-availability deployments

See [Database Configuration](DATABASE_CONFIGURATION.md) for setup details.

### High Availability

For production deployments:

1. Deploy multiple Argus instances behind a load balancer
2. Use PostgreSQL for shared database
3. Configure health checks on `/health` endpoint
4. Monitor service metrics

### Security

When exposing Argus publicly:

1. Implement authentication/authorization
2. Use HTTPS/TLS
3. Configure CORS appropriately
4. Rate limit API endpoints
5. Monitor for abuse

## Integration Best Practices

1. **Use the Go client library** - Prefer `pkg/audit` over direct HTTP calls
2. **Handle errors gracefully** - Audit failures shouldn't break your service
3. **Use trace IDs** - Pass trace IDs through your service chain
4. **Don't store PII** - Keep sensitive data out of audit metadata
5. **Configure event types** - Customize `configs/enums.yaml` for your use case
6. **Monitor audit service** - Check `/health` endpoint in your monitoring
7. **Test your integration** - Verify audit logs are created correctly

## Related Documentation

- [API Documentation](API.md) - REST API reference
- [Database Configuration](DATABASE_CONFIGURATION.md) - Database setup
- [Configuration Guide](../internal/config/README.md) - Event type customization
