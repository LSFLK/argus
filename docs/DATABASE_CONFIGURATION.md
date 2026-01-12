# Database Configuration

Configure Argus database settings for your deployment. Argus supports multiple database backends, making it easy to integrate into different environments.

## Overview

When deploying Argus, choose the database backend that fits your needs:

1. **In-memory SQLite** (default) - Zero configuration, perfect for development and testing
2. **File-based SQLite** - Persistent storage, good for single-server deployments
3. **PostgreSQL** - Production-grade, supports high availability and multiple instances

## Configuration Priority

The service determines which database to use based on the following priority:

1. **If `DB_TYPE=postgres`** → Use PostgreSQL (requires DB_HOST, DB_PASSWORD, etc.)
2. **If `DB_TYPE=sqlite` OR `DB_PATH` is set** → Use file-based SQLite
   - Uses `DB_PATH` value if specified
   - Defaults to `./data/audit.db` if not specified
3. **If NO database configuration** → Use in-memory SQLite (`:memory:`)

**Why this design?**
- Setting `DB_PATH` alone implies you want file-based SQLite
- No configuration at all means quick testing with in-memory database
- Explicit `DB_TYPE=sqlite` with no `DB_PATH` uses sensible default path

## Configuration Modes

### 1. In-Memory SQLite (Default)

**Use Case:** Development, testing, or temporary audit logs

**Behavior:**
- No environment variables needed
- Data stored in memory only
- Data lost when service restarts
- Fastest performance for testing

**Example:**

```bash
go run ./cmd/argus
```

### 2. File-Based SQLite

**Use Case:** Single-server deployments, development, small-scale production

**Configuration Options:**

```bash
# Option 1: Set DB_TYPE=sqlite (uses default path)
export DB_TYPE=sqlite
go run ./cmd/argus
# → Uses: ./data/audit.db

# Option 2: Set DB_PATH only (implies SQLite)
export DB_PATH=./data/audit.db
go run ./cmd/argus

# Option 3: Set both for custom path
export DB_TYPE=sqlite
export DB_PATH=/var/lib/audit/production.db
go run ./cmd/argus
```

### 3. PostgreSQL

**Use Case:** Production deployments, high concurrency, multiple instances

**Configuration:**

```bash
export DB_TYPE=postgres
export DB_HOST=localhost
export DB_PORT=5432
export DB_USERNAME=postgres
export DB_PASSWORD=your_secure_password
export DB_NAME=audit_db
export DB_SSLMODE=require  # or disable for local

go run ./cmd/argus
```

## Environment Variables Reference

### Database Type Selection

| Variable  | Values                        | Default | Description                                      |
| --------- | ----------------------------- | ------- | ------------------------------------------------ |
| `DB_TYPE` | `sqlite`, `postgres`, or omit | (none)  | Database type. If omitted, uses in-memory SQLite |

### SQLite Configuration

| Variable            | Default           | Description                                                         |
| ------------------- | ----------------- | ------------------------------------------------------------------- |
| `DB_PATH`           | `./data/audit.db` | Path to SQLite database file. Use `:memory:` for in-memory database |
| `DB_MAX_OPEN_CONNS` | `1`               | Max open connections (SQLite best practice: use 1)                  |
| `DB_MAX_IDLE_CONNS` | `1`               | Max idle connections                                                |

**Note:** SQLite uses `MaxOpenConns=1` by default to prevent "database is locked" errors.

### PostgreSQL Configuration

| Variable            | Default     | Required | Description                                   |
| ------------------- | ----------- | -------- | --------------------------------------------- |
| `DB_HOST`           | `localhost` | No       | PostgreSQL server hostname                    |
| `DB_PORT`           | `5432`      | No       | PostgreSQL server port                        |
| `DB_USERNAME`       | `postgres`  | No       | Database username                             |
| `DB_PASSWORD`       | -           | **Yes**  | Database password                             |
| `DB_NAME`           | `audit_db`  | No       | Database name                                 |
| `DB_SSLMODE`        | `disable`   | No       | SSL mode: `disable`, `require`, `verify-full` |
| `DB_MAX_OPEN_CONNS` | `25`        | No       | Maximum open database connections             |
| `DB_MAX_IDLE_CONNS` | `5`         | No       | Maximum idle connections in pool              |

### Connection Pool Settings (Both Databases)

| Variable                | Default | Description                             |
| ----------------------- | ------- | --------------------------------------- |
| `DB_CONN_MAX_LIFETIME`  | `1h`    | Maximum time a connection can be reused |
| `DB_CONN_MAX_IDLE_TIME` | `15m`   | Maximum time a connection can be idle   |

Accepts Go duration formats: `1h`, `30m`, `15s`, etc.

## Configuration Examples

### Development (In-Memory)

```bash
go run ./cmd/argus
```

### Local Development (File-Based)

```env
# .env file
DB_TYPE=sqlite
DB_PATH=./data/audit.db
LOG_LEVEL=debug
```

### Production (PostgreSQL)

```env
# .env file
DB_TYPE=postgres
DB_HOST=db.example.com
DB_PORT=5432
DB_USERNAME=audit_user
DB_PASSWORD=secure_password_here
DB_NAME=audit_production
DB_SSLMODE=require
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
```

### Docker with PostgreSQL

```yaml
# docker-compose.yml
version: '3.8'
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: audit_db
      POSTGRES_USER: audit_user
      POSTGRES_PASSWORD: secure_password
    volumes:
      - postgres-data:/var/lib/postgresql/data

  argus:
    build: .
    environment:
      DB_TYPE: postgres
      DB_HOST: postgres
      DB_USERNAME: audit_user
      DB_PASSWORD: secure_password
      DB_NAME: audit_db
    depends_on:
      - postgres
    ports:
      - "3001:3001"

volumes:
  postgres-data:
```

## Switching Between Databases

Simply change the `DB_TYPE` environment variable:

```bash
# Switch to in-memory (remove DB_TYPE)
unset DB_TYPE
go run ./cmd/argus

# Switch to file-based SQLite
export DB_TYPE=sqlite
go run ./cmd/argus

# Switch to PostgreSQL
export DB_TYPE=postgres
export DB_HOST=localhost
export DB_PASSWORD=your_password
go run ./cmd/argus
```

No code changes required - the service automatically adapts to the configuration.

## Database Schema

The service uses GORM auto-migration to create the schema automatically on startup:

- **Table:** `audit_logs`
- **Columns:** id, trace_id, timestamp, event_type, event_action, status, actor_type, actor_id, target_type, target_id, request_metadata, response_metadata, additional_metadata, created_at

The schema is identical across SQLite and PostgreSQL.

## Performance Considerations

### SQLite

- Zero configuration
- No external dependencies
- Fast for read operations
- Limited concurrent writes
- Single-server only

**Best for:** Development, testing, small deployments (<1000 requests/min)

### PostgreSQL

- High concurrency
- Connection pooling
- Multi-instance support
- Production-grade reliability
- Requires external database setup

**Best for:** Production, high-traffic deployments (>1000 requests/min)

## Troubleshooting

### "database is locked" Error (SQLite)

**Cause:** Multiple write operations attempting concurrently

**Solution:** This is prevented by default with `DB_MAX_OPEN_CONNS=1`. If you changed this setting, revert to 1:

```bash
export DB_MAX_OPEN_CONNS=1
```

### PostgreSQL Connection Timeout

**Cause:** Network issues or incorrect credentials

**Solution:**
1. Verify database is running and accessible
2. Check credentials in environment variables
3. Verify SSL mode matches server requirements
4. Check firewall rules

### Permission Denied Creating SQLite Directory

**Cause:** Insufficient permissions for `DB_PATH` directory

**Solution:**

```bash
mkdir -p ./data
chmod 755 ./data
```

## Migration Guide

### From File-Based SQLite to PostgreSQL

1. Export SQLite data: `sqlite3 ./data/audit.db .dump > audit_backup.sql`
2. Setup PostgreSQL: Create database and user with appropriate permissions
3. Update configuration: Set `DB_TYPE=postgres` and PostgreSQL credentials
4. Restart service - schema auto-created by GORM

**Note:** Data migration requires custom scripting or manual export/import.
