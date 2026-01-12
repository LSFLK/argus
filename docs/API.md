# API Documentation

Complete API reference for integrating Argus into your microservices architecture.

## Base URL

- **Development**: `http://localhost:3001`
- **Production**: `https://argus.yourdomain.com` or `http://argus-service:3001` (internal)

## Endpoints Overview

| Method | Endpoint          | Description                        |
| ------ | ----------------- | ---------------------------------- |
| POST   | `/api/audit-logs` | Create a new audit log entry       |
| GET    | `/api/audit-logs` | Retrieve audit logs with filtering |
| GET    | `/health`         | Service health check               |
| GET    | `/version`        | Service version information        |

---

## Create Audit Log

**Endpoint:** `POST /api/audit-logs`

**Request Body:**

| Field                | Type          | Required | Description                                                  |
| -------------------- | ------------- | -------- | ------------------------------------------------------------ |
| `timestamp`          | string        | Required | ISO 8601 timestamp (RFC3339 format)                          |
| `status`             | string        | Required | Event status: `SUCCESS` or `FAILURE`                         |
| `actorType`          | string        | Required | Actor type: `SERVICE`, `ADMIN`, `MEMBER`, `SYSTEM`           |
| `actorId`            | string        | Required | Actor identifier (email, UUID, service name)                 |
| `targetType`         | string        | Required | Target type: `SERVICE` or `RESOURCE`                         |
| `traceId`            | string (UUID) | Optional | Trace ID for distributed tracing                             |
| `eventType`          | string        | Optional | Custom event type (e.g., `MANAGEMENT_EVENT`)                 |
| `eventAction`        | string        | Optional | Action: `CREATE`, `READ`, `UPDATE`, `DELETE`                 |
| `targetId`           | string        | Optional | Target identifier                                             |
| `requestMetadata`    | object        | Optional | Request payload (without PII/sensitive data)                 |
| `responseMetadata`   | object        | Optional | Response or error details                                    |
| `additionalMetadata` | object        | Optional | Additional context-specific data                             |

**Example Request:**

```bash
curl -X POST http://localhost:3001/api/audit-logs \
  -H "Content-Type: application/json" \
  -d '{
    "traceId": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": "2024-01-20T10:00:00Z",
    "eventType": "MANAGEMENT_EVENT",
    "eventAction": "READ",
    "status": "SUCCESS",
    "actorType": "SERVICE",
    "actorId": "my-service",
    "targetType": "SERVICE",
    "targetId": "target-service",
    "requestMetadata": {"schemaId": "schema-123"},
    "responseMetadata": {"decision": "ALLOWED"}
  }'
```

**Success Response: 201 Created**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "traceId": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-01-20T10:00:00Z",
  "eventType": "MANAGEMENT_EVENT",
  "status": "SUCCESS",
  "actorType": "SERVICE",
  "actorId": "my-service",
  "targetType": "SERVICE",
  "targetId": "target-service",
  "createdAt": "2024-01-20T10:00:00.123456Z"
}
```

**Error Response: 400 Bad Request**

```json
{
  "error": "Validation error: invalid timestamp format, expected RFC3339"
}
```

---

## Get Audit Logs

**Endpoint:** `GET /api/audit-logs`

**Query Parameters:**

| Parameter     | Type          | Required | Default | Description                               |
| ------------- | ------------- | -------- | ------- | ----------------------------------------- |
| `traceId`     | string (UUID) | Optional | -       | Filter by trace ID                        |
| `eventType`   | string        | Optional | -       | Filter by event type                      |
| `eventAction` | string        | Optional | -       | Filter by event action                    |
| `status`      | string        | Optional | -       | Filter by status (`SUCCESS` or `FAILURE`) |
| `limit`       | integer       | Optional | 100     | Max results per page (1-1000)             |
| `offset`      | integer       | Optional | 0       | Number of results to skip                 |

**Example Requests:**

```bash
# Get all audit logs (paginated)
curl http://localhost:3001/api/audit-logs

# Filter by trace ID
curl http://localhost:3001/api/audit-logs?traceId=550e8400-e29b-41d4-a716-446655440000

# Filter by event type
curl http://localhost:3001/api/audit-logs?eventType=MANAGEMENT_EVENT

# Multiple filters with pagination
curl http://localhost:3001/api/audit-logs?eventType=MANAGEMENT_EVENT&status=SUCCESS&limit=20&offset=0
```

**Success Response: 200 OK**

```json
{
  "logs": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440001",
      "traceId": "550e8400-e29b-41d4-a716-446655440000",
      "timestamp": "2024-01-20T10:00:00Z",
      "eventType": "MANAGEMENT_EVENT",
      "status": "SUCCESS",
      "actorType": "SERVICE",
      "actorId": "my-service",
      "targetType": "SERVICE",
      "targetId": "target-service",
      "createdAt": "2024-01-20T10:00:00.123456Z"
    }
  ],
  "total": 100,
  "limit": 100,
  "offset": 0
}
```

---

## System Endpoints

### Health Check

**Endpoint:** `GET /health`

```bash
curl http://localhost:3001/health
```

**Response: 200 OK**

```json
{
  "service": "argus",
  "status": "healthy"
}
```

### Version Information

**Endpoint:** `GET /version`

```bash
curl http://localhost:3001/version
```

**Response: 200 OK**

```json
{
  "service": "argus",
  "version": "1.0.0",
  "buildTime": "2024-01-20T10:00:00Z",
  "gitCommit": "abc123def456"
}
```

---

## Data Types & Enums

### Event Status
- `SUCCESS` - Event completed successfully
- `FAILURE` - Event failed or encountered an error

### Actor Types
- `SERVICE` - Internal service
- `ADMIN` - Administrator user
- `MEMBER` - End user/member
- `SYSTEM` - System-level operations

### Target Types
- `SERVICE` - Target is a service
- `RESOURCE` - Target is a resource

### Event Actions (Optional)
- `CREATE` - Resource creation
- `READ` - Data retrieval
- `UPDATE` - Resource modification
- `DELETE` - Resource deletion

### Event Types (Examples)
Custom event types can be defined per use case:
- `MANAGEMENT_EVENT` - Administrative action
- `USER_MANAGEMENT` - User management operations
- `DATA_FETCH` - Data retrieval operation

---

## Best Practices

### Timestamp Format

Always use RFC3339 format (ISO 8601):
- Correct: `2024-01-20T10:00:00Z`
- Correct: `2024-01-20T10:00:00.123456Z`
- Wrong: `2024-01-20 10:00:00`

### Trace IDs

- Use UUIDs (RFC 4122) for trace IDs
- Generate at the entry point of a distributed flow
- Pass through all services in the request chain
- Use `null` for standalone events

### Metadata Guidelines

**DO:**
- Include operation context in `requestMetadata`
- Include decision/result in `responseMetadata`
- Use `additionalMetadata` for service-specific context

**DON'T:**
- Store PII (Personally Identifiable Information)
- Store sensitive data (passwords, tokens, keys)
- Store full response payloads with user data

**Example:**

```json
{
  "requestMetadata": {
    "schemaId": "schema-123",
    "requestedFields": ["name", "address"]
  },
  "responseMetadata": {
    "decision": "ALLOWED",
    "fieldsReturned": 2
  }
}
```

### Error Handling

Always log failed operations:

```json
{
  "status": "FAILURE",
  "responseMetadata": {
    "error": "operation_failed",
    "errorMessage": "Resource not found",
    "errorCode": "404"
  }
}
```

### Pagination

For large result sets, use pagination:

```bash
# First page
GET /api/audit-logs?limit=100&offset=0

# Second page
GET /api/audit-logs?limit=100&offset=100
```

Use the `total` field in the response to calculate total pages.

---

## API Testing

You can test the API using:
- **curl** - Command-line HTTP client (examples provided above)
- **Postman** - Import the API endpoints manually
- **HTTPie** - User-friendly HTTP client
