# Event Type Configuration

Customize Argus for your use case by defining custom event types, actions, and actor/target types. This allows you to tailor audit logging to your specific domain requirements.

## Configuration File

**File:** `configs/enums.yaml`

When integrating Argus, you can customize the allowed values for:

- **Event Types**: Define your own event types (e.g., `MANAGEMENT_EVENT`, `USER_MANAGEMENT`)
- **Event Actions**: CRUD operations (e.g., `CREATE`, `READ`, `UPDATE`, `DELETE`)
- **Actor Types**: Types of actors in your system (e.g., `SERVICE`, `ADMIN`, `MEMBER`, `SYSTEM`)
- **Target Types**: Types of targets (e.g., `SERVICE`, `RESOURCE`, `USER`, `ORDER`)

## Usage

When you deploy Argus, the configuration is automatically loaded at startup:

1. Argus looks for `configs/enums.yaml` in the working directory
2. If not found, uses sensible defaults
3. If the file is invalid, logs a warning and uses defaults

**For Docker deployments**, mount your custom `enums.yaml`:

```bash
docker run -v /path/to/your/enums.yaml:/app/configs/enums.yaml argus
```

### Custom Configuration Path

You can specify a custom path using the `AUDIT_ENUMS_CONFIG` environment variable:

```bash
AUDIT_ENUMS_CONFIG=/path/to/custom/enums.yaml ./argus
```

## Customizing for Your Integration

Add your own event types and values to match your domain:

```yaml
enums:
  eventTypes:
    - MANAGEMENT_EVENT
    - MANAGEMENT_EVENT
    - ORDER_CREATED          # Your custom event
    - USER_REGISTERED        # Your custom event
  
  actorTypes:
    - SERVICE
    - ADMIN
    - MEMBER
    - SYSTEM
    - CUSTOMER               # Your custom actor type
  
  targetTypes:
    - SERVICE
    - RESOURCE
    - ORDER                  # Your custom target type
```

After updating `configs/enums.yaml`, restart Argus to apply changes.

## Validation Behavior

When your services send audit events to Argus:

- **Invalid enum values** → Rejected with 400 Bad Request
- **Optional fields** (`eventType`, `eventAction`) → Can be null/empty
- **Required fields** (`actorType`, `targetType`) → Must match configured values

This ensures data consistency and helps catch integration errors early.
