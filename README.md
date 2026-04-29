# Argus
**_Generic, Hardened Audit Logging Microservice with Pipeline Architecture_**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.24.6%2B-blue)](https://golang.org/)
[![Prometheus](https://img.shields.io/badge/Metrics-Prometheus-orange)](http://localhost:3001/metrics)

**Argus** is a highly resilient Go service for centralized audit logging and distributed tracing. It features a pluggable **Pipeline & Sink** architecture, enabling concurrent fan-out to multiple storage backends, and enforces **non-repudiation** through cryptographic signatures and tamper-evident hash chaining.

<p align="center">
  •   <a href="#quick-start-using-the-audit-interface">Quick Start</a> •
  <a href="#pipeline--sink-architecture">Architecture</a> •
  <a href="#security--non-repudiation">Security</a> •
  <a href="#observability">Observability</a> •
  <a href="#integration">Integration</a> •
  <a href="#license">License</a> •
</p>

---

## Key Features

- **Pluggable Pipeline Architecture** – Fan-out logs concurrently to PostgreSQL, S3, SIEMs (Splunk/ELK), or Kafka without changing core logic.
- **Tamper-Evident Hash Chaining** – Every log entry is cryptographically linked to the previous one, creating an immutable, verifiable audit trail.
- **Cryptographic Non-Repudiation** – Server-side verification of RSA/Ed25519 signatures for incoming logs.
- **High-Performance Batching** – Client-side worker pool with buffered batching to minimize HTTP overhead and eliminate goroutine leaks.
- **Production Observability** – Built-in Prometheus metrics for ingestion rates, latencies, and security errors.
- **Secure by Default** – Bearer token authentication and strict validation of log schemas.

## Quick Start: Using the Audit Interface

### Step 1: Add Argus to your project
```bash
go get github.com/LSFLK/argus/pkg/audit
```

### Step 2: Initialize the hardened audit client
```go
func main() {
    client := audit.NewClient("http://argus:3001", 
        audit.WithBatchSize(50), 
        audit.WithBatchInterval(2 * time.Second),
        audit.WithWorkerPool(5),
    )
    audit.InitializeGlobalAudit(client)
}
```

### Step 3: Log audit events
```go
audit.LogAuditEvent(ctx, &audit.AuditLogRequest{
    EventType:  "USER_ACTION",
    Action:     "DELETE",
    Status:     "SUCCESS",
    ActorID:    "admin-user",
    TargetType: "RESOURCE",
    TargetID:   "resource-123",
})
```

---

## Integration with External Applications

Argus is designed to be the centralized audit source of truth for other platforms. By integrating the Argus client, your application gains high-performance, tamper-evident logging with zero impact on core performance.

### 1. Installation
In your application (e.g., `nsw-api`):
```bash
go get github.com/LSFLK/argus/pkg/audit
```

### 2. Global Initialization
Initialize the client in your main entry point. For high-scale systems, tune the batching settings to balance latency and throughput.

```go
func main() {
    // Connect to the centralized Argus service
    client := audit.NewClient("http://argus-service.nsw.svc.cluster.local:3001",
        audit.WithBatchSize(100),
        audit.WithBatchInterval(500 * time.Millisecond),
        audit.WithWorkerPool(10),
    )
    
    // Set as global auditor for the application
    audit.InitializeGlobalAudit(client)
}
```

### 3. Implementing Non-Repudiation (Advanced)
To ensure logs cannot be spoofed, your application can sign requests using a private key. Argus will verify these on the server-side.

```go
// Example: Signing a log in an NSW Submission handler
func HandleSubmission(ctx context.Context, sub *Submission) {
    req := &audit.AuditLogRequest{
        EventType: "SUBMISSION",
        Action:    "CREATE",
        ActorID:   sub.UserID,
        Message:   map[string]interface{}{"submission_id": sub.ID},
    }

    // Attach signature using your service's private key
    // req.Signature = sign(req, myPrivateKey) 
    // req.PublicKeyID = "nsw-api-prod-01"

    audit.LogAuditEvent(ctx, req)
}
```

### 4. Benefits for National-Scale Platforms
- **Centralized Compliance:** Single point of audit for multiple agencies and microservices.
- **WORM Storage Ready:** Using the Pipeline architecture, you can route logs to S3 Object Lock or physical WORM drives for regulatory compliance.
- **Traceability:** Propagate `trace_id` from Argus into your downstream logs for end-to-end observability.

---

## Pipeline & Sink Architecture

Argus uses a **Manager/Sink** pattern. When a log is received, it is validated and then dispatched to all registered "Sinks" concurrently.

| Sink | Description | Status |
| --- | --- | --- |
| **PostgresSink** | Primary storage with hash-chaining and GORM support. | ✅ Included |
| **ConsoleSink** | Failsafe JSON logger to stdout for dev/debugging. | ✅ Included |
| **S3Sink** | Immutable WORM storage for regulatory compliance. | 🔜 Roadmap |
| **KafkaSink** | Real-time event streaming for downstream analytics. | 🔜 Roadmap |

## Security & Non-Repudiation

### Hash Chaining
Argus maintains a `PreviousHash` and `CurrentHash` for every record. If any record in the database is modified or deleted, the chain breaks, making the tampering immediately detectable.

### Signature Verification
The service can be configured with a `PublicKeyRegistry`. If an incoming log includes a `publicKeyId` and `signature`, Argus will verify the authenticity of the log before persisting it to any sink.

## Observability

Argus exports standard Prometheus metrics at `/metrics`:
- `argus_logs_ingested_total`: Count of successfully processed logs.
- `argus_http_request_duration_seconds`: Latency of ingestion requests.
- `argus_signature_verification_errors_total`: Count of invalid signature attempts.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `ARGUS_AUTH_TOKEN` | - | Bearer token required for API access. |
| `DB_TYPE` | `sqlite` | `sqlite` or `postgres`. |
| `AUDIT_ENUMS_CONFIG` | `configs/enums.yaml` | Path to allowed event types/actions. |

## Documentation
- [API Reference](docs/API.md)
- [Architecture Deep Dive](docs/ARCHITECTURE.md)
- [Database Setup](docs/DATABASE_CONFIGURATION.md)

## License
Distributed under the Apache 2.0 License. See [LICENSE](LICENSE) for more information.
