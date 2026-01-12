# Argus
**_Generic Audit Logging Service for Microservices_**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.24.6%2B-blue)](https://golang.org/)

**Argus** is a generic, reusable Go service for centralized audit logging and distributed tracing across microservices architectures. Argus provides a clean interface-based design that can be integrated into any microservices ecosystem.

<p align="center">
  •   <a href="#quick-start-using-the-audit-interface">Quick Start</a> •
  <a href="#why-argus">Why Argus?</a> •
  <a href="#integration">Integration</a> •
  <a href="#deployment">Deployment</a> •
  <a href="#contributing">Contributing</a> •
  <a href="#license">License</a> •
</p>

---

## Quick Start: Using the Audit Interface

**Want to use Argus in your Go project?** It's simple - just import the package and start logging!

### Step 1: Add Argus to your project

```bash
go get github.com/LSFLK/argus/pkg/audit
```

### Step 2: Initialize the audit client

```go
package main

import (
    "context"
    "os"
    "time"
    "github.com/LSFLK/argus/pkg/audit"
)

func main() {
    // Initialize audit client (point to your Argus service)
    auditURL := os.Getenv("ARGUS_SERVICE_URL") // e.g., "http://argus:3001"
    auditClient := audit.NewClient(auditURL)
    audit.InitializeGlobalAudit(auditClient)
}
```

### Step 3: Log audit events

```go
// In your handlers or business logic
audit.LogAuditEvent(ctx, &audit.AuditLogRequest{
    Timestamp:  time.Now().UTC().Format(time.RFC3339),
    EventType:  "USER_ACTION",
    Status:     "SUCCESS",
    ActorType:  "SERVICE",
    ActorID:    "my-service",
    TargetType: "RESOURCE",
    TargetID:   "resource-123",
})
```

**That's it!** The audit client works asynchronously, handles failures gracefully, and can be disabled via `ENABLE_AUDIT=false`.

**Note:** You need an Argus service instance running (see [Deployment](#deployment)). The `pkg/audit` package is just the client library.

---

## Why Argus?

Argus provides a clean, interface-based approach to audit logging that makes it easy to integrate into any microservices architecture. The service tracks "who did what, when, and with what result" by providing both a REST API and a reusable Go interface (`pkg/audit`) that can be imported into any service.

**Key Benefits:**
- **Interface-Based Design** – Use the `pkg/audit` package without tight coupling
- **Zero Configuration** – Works out of the box with in-memory database
- **Flexible Backends** – Supports SQLite (in-memory, file-based) and PostgreSQL
- **Graceful Degradation** – Services continue functioning even if audit service is unavailable
- **Distributed Tracing** – Built-in support for trace IDs across service boundaries

## Getting Started

### Prerequisites

- Go 1.24.6 or higher
- (Optional) PostgreSQL for production deployments

### Installation

```bash
git clone https://github.com/LSFLK/argus.git
cd argus
go mod tidy
go run ./cmd/argus
```

Service starts on `http://localhost:3001`

### Quick Test

```bash
curl http://localhost:3001/health
curl -X POST http://localhost:3001/api/audit-logs \
  -H "Content-Type: application/json" \
  -d '{"timestamp": "2024-01-20T10:00:00Z", "status": "SUCCESS", "actorType": "SERVICE", "actorId": "test-service", "targetType": "RESOURCE", "eventType": "TEST_EVENT"}'
```

## Configuration

### Database Options

| Mode                  | Configuration                    | Use Case                     |
| --------------------- | -------------------------------- | ---------------------------- |
| **In-Memory SQLite**  | No config needed                 | Development, testing         |
| **File-Based SQLite** | `DB_TYPE=sqlite` OR `DB_PATH` set | Single-server deployments    |
| **PostgreSQL**        | `DB_TYPE=postgres` + credentials | Production, high concurrency |

**Examples:**
```bash
# In-memory (default)
go run ./cmd/argus

# File-based SQLite
export DB_TYPE=sqlite && export DB_PATH=./data/audit.db && go run ./cmd/argus

# PostgreSQL
export DB_TYPE=postgres && export DB_HOST=localhost && export DB_USERNAME=postgres && export DB_PASSWORD=your_password && export DB_NAME=audit_db && go run ./cmd/argus
```

See [docs/DATABASE_CONFIGURATION.md](docs/DATABASE_CONFIGURATION.md) for complete database setup guide.

### Environment Variables

| Variable               | Default                 | Description                                 |
| ---------------------- | ----------------------- | ------------------------------------------- |
| `PORT`                 | `3001`                  | Service port                                |
| `DB_TYPE`              | -                       | Database type: `sqlite` or `postgres`. If not set, uses in-memory SQLite |
| `DB_PATH`              | `./data/audit.db`       | SQLite database path                        |
| `LOG_LEVEL`            | `info`                  | Log level: `debug`, `info`, `warn`, `error` |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | Allowed CORS origins                        |

### Event Type Configuration

Event types are configurable via `configs/enums.yaml`:

```yaml
enums:
  eventTypes:
    - MANAGEMENT_EVENT
    - USER_MANAGEMENT
    - DATA_FETCH
    - YOUR_CUSTOM_EVENT_TYPE
```

See [internal/config/README.md](internal/config/README.md) for detailed configuration options.

## Integration

### Two Ways to Use Argus

1. **Go Package (Recommended)** – Import `github.com/LSFLK/argus/pkg/audit` in your Go services
2. **REST API** – Make HTTP calls from any language

### Option 1: Go Package

```bash
go get github.com/LSFLK/argus/pkg/audit
```

```go
import (
    "os"
    "github.com/LSFLK/argus/pkg/audit"
)

func init() {
    client := audit.NewClient(os.Getenv("ARGUS_SERVICE_URL"))
    audit.InitializeGlobalAudit(client)
}

// Use anywhere
audit.LogAuditEvent(ctx, &audit.AuditLogRequest{
    Timestamp:  time.Now().UTC().Format(time.RFC3339),
    EventType:  "USER_ACTION",
    Status:     "SUCCESS",
    ActorType:  "SERVICE",
    ActorID:    "my-service",
    TargetType: "RESOURCE",
    TargetID:   "resource-123",
})
```

**Key Features:** Asynchronous, graceful degradation, thread-safe, can be disabled via `ENABLE_AUDIT=false`

### Option 2: REST API

```bash
curl -X POST http://argus-service:3001/api/audit-logs \
  -H "Content-Type: application/json" \
  -d '{"timestamp": "2024-01-20T10:00:00Z", "eventType": "USER_ACTION", "status": "SUCCESS", "actorType": "SERVICE", "actorId": "my-service", "targetType": "RESOURCE", "targetId": "resource-123"}'
```

See [docs/API.md](docs/API.md) for complete API documentation.

### The Auditor Interface

```go
type Auditor interface {
    LogEvent(ctx context.Context, event *AuditLogRequest)
    IsEnabled() bool
}
```

**Benefits:** Loose coupling, easy testing, flexible, graceful degradation

## REST API Endpoints

| Method | Endpoint          | Description                              |
| ------ | ----------------- | ---------------------------------------- |
| POST   | `/api/audit-logs` | Create audit log entry                   |
| GET    | `/api/audit-logs` | Retrieve audit logs (filtered/paginated) |
| GET    | `/health`         | Health check                             |
| GET    | `/version`        | Version information                      |

See [docs/API.md](docs/API.md) for complete API documentation.

## Development

### Running Tests

```bash
go test ./...                    # Unit tests
go test ./... -cover             # With coverage
go test ./... -tags=integration  # Integration tests
```

### Building

```bash
go build -o argus ./cmd/argus
go build -ldflags="-X main.Version=1.0.0 -X main.GitCommit=$(git rev-parse HEAD)" -o argus ./cmd/argus
```

## Deployment

**Important:** If you're using the `pkg/audit` client library, you still need to deploy the Argus service somewhere. The client library sends HTTP requests to the Argus service.

### Quick Deployment Options

#### Option 1: Docker Compose (Easiest)

```bash
docker compose up -d
# Service available at http://localhost:3001
```

#### Option 2: Docker

```bash
docker build -t argus .
docker run -d -p 3001:3001 -e DB_TYPE=sqlite -e DB_PATH=/data/audit.db -v audit-data:/data argus
```

#### Option 3: Binary

```bash
go build -o argus ./cmd/argus
./argus
```

### Production Considerations

1. **Database**: Use PostgreSQL for production
2. **Logging**: Set `LOG_LEVEL=info` or `LOG_LEVEL=warn`
3. **CORS**: Configure `CORS_ALLOWED_ORIGINS` appropriately
4. **Monitoring**: Monitor `/health` endpoint
5. **High Availability**: Deploy multiple instances behind a load balancer
6. **Security**: Implement authentication/authorization if exposing publicly

## Project Structure

Key directories: **`pkg/audit/`** (import this), **`configs/`** (enums.yaml), **`cmd/argus/`** (entry point), **`internal/`** (private), **`docs/`** (documentation)

## Documentation

- [API Documentation](docs/API.md) - Complete API reference
- [Architecture](docs/ARCHITECTURE.md) - Architecture overview
- [Database Configuration](docs/DATABASE_CONFIGURATION.md) - Database setup guide
- [Configuration Guide](internal/config/README.md) - Configuration options

## Contributing

Thank you for wanting to contribute to Argus. Please see [CONTRIBUTING.md](docs/CONTRIBUTING.md) for more details.

## License

Distributed under the Apache 2.0 License. See [LICENSE](LICENSE) for more information.
