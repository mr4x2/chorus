# Health Check Configuration

## Overview

Chorus worker performs health checks on S3 storage endpoints to ensure they are available before attempting replication operations. By default, health checks are enabled and will cause the worker to shut down if a storage is unreachable during startup.

## Configuration

### YAML Configuration

You can control health check behavior in your storage configuration:

```yaml
storage:
  storages:
    my-storage:
      address: "http://s3.example.com:9000"
      credentials:
        user1:
          accessKeyID: "access-key"
          secretAccessKey: "secret-key"
      provider: "Minio"
      isMain: true
      healthCheckInterval: 10s        # How often to check (default: 5s)
      healthCheckEnabled: true        # Enable/disable health checks (default: true)
      httpTimeout: 1m
      isSecure: false
```

### Database Configuration

When using database-backed configuration, the `health_check_enabled` field in the `storage` table controls this behavior:

```sql
-- Disable health checks for a storage
UPDATE storage SET health_check_enabled = false WHERE name = 'my-storage';
```

## Use Cases

### Disable Health Checks

Set `healthCheckEnabled: false` when:

1. **Misconfigured Storages**: Storage endpoints are temporarily misconfigured but you want the worker to start
2. **Development/Testing**: Using fake or mock S3 endpoints that don't respond to health checks
3. **Network Issues**: Storage is behind a firewall or has intermittent connectivity
4. **Gradual Rollout**: Want to start the worker before all storages are fully configured

### Example: Development Setup

```yaml
storage:
  storages:
    fake-main:
      address: "http://fake-s3-main:9000"  # This might be offline
      credentials:
        user1:
          accessKeyID: "minioadmin"
          secretAccessKey: "minioadmin"
      provider: "Minio"
      isMain: true
      healthCheckEnabled: false  # Prevent shutdown on health check failure
      healthCheckInterval: 10s
      httpTimeout: 1m
      isSecure: false
```

## Behavior

### With Health Checks Enabled (Default)

- Worker performs initial health check during startup
- If health check fails, worker shuts down with error
- Periodic health checks run in background
- Storage marked as "offline" if health checks fail
- Worker continues running but logs warnings for offline storages

### With Health Checks Disabled

- No initial health check during startup
- Worker assumes storage is online
- No periodic health checks
- Worker starts successfully even if storage is unreachable
- Storage operations will fail at runtime if storage is actually offline

## Logging

Health check events are logged with appropriate levels:

- **Info**: Health checks disabled, storage back online
- **Warn**: Initial health check failed (non-fatal)
- **Debug**: Periodic health check failed (storage remains offline)

## Migration

To migrate existing configurations:

1. **YAML**: Add `healthCheckEnabled: false` to problematic storages
2. **Database**: Update `health_check_enabled` column in `storage` table
3. **Restart**: Restart worker to apply changes

## Best Practices

1. **Production**: Keep health checks enabled for reliable operation
2. **Development**: Disable for fake/mock endpoints
3. **Troubleshooting**: Temporarily disable to isolate issues
4. **Monitoring**: Monitor logs for health check failures even when disabled
