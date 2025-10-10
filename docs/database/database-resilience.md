# Database Resilience and Repository Patterns

This document describes the resilience features implemented in the Chorus database layer, including retry mechanisms, circuit breakers, and repository patterns.

## Overview

The database layer in Chorus implements several resilience patterns to handle transient failures, network issues, and database unavailability gracefully. These patterns ensure that the system remains stable and responsive even when the database experiences temporary problems.

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Application   │───▶│  ResilientDB     │───▶│   GORM DB       │
│   Layer         │    │  (Circuit        │    │   (PostgreSQL)  │
│                 │    │   Breaker +      │    │                 │
│                 │    │   Retry Logic)   │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Resilience Components

### 1. Circuit Breaker

The circuit breaker prevents cascading failures by monitoring database operation success rates and temporarily blocking requests when the failure rate exceeds a threshold.

#### States

- **Closed**: Normal operation, requests pass through
- **Open**: Circuit is open, requests are immediately rejected
- **Half-Open**: Testing if the service has recovered, limited requests allowed

#### Configuration

```yaml
database:
  resilience:
    failureThreshold: 5      # Failures before opening circuit
    recoveryTimeout: "30s"   # Time to wait before testing recovery
    halfOpenMaxCalls: 3      # Max calls in half-open state
```

#### Behavior

```go
// Circuit opens after 5 consecutive failures
// Stays open for 30 seconds
// Allows 3 test calls in half-open state
// Returns to closed if all test calls succeed
```

### 2. Retry Logic

Implements exponential backoff retry for transient failures.

#### Configuration

```yaml
database:
  resilience:
    maxRetries: 3           # Maximum retry attempts
    retryDelay: "100ms"     # Initial delay between retries
    retryMultiplier: 2.0    # Exponential backoff multiplier
    maxRetryDelay: "5s"     # Maximum delay between retries
```

#### Retry Sequence

```
Attempt 1: Immediate
Attempt 2: 100ms delay
Attempt 3: 200ms delay (100ms * 2.0)
Attempt 4: 400ms delay (200ms * 2.0)
Attempt 5: 800ms delay (400ms * 2.0)
```

#### Retryable Errors

The system automatically retries these types of errors:

- Connection refused
- Connection reset
- Broken pipe
- Timeout errors
- Temporary failures
- Too many connections
- Server closed connection
- Network unreachable
- No route to host

#### Non-Retryable Errors

These errors are not retried:

- Context cancellation
- Context timeout
- Record not found
- Validation errors
- Permission denied
- Business logic errors

### 3. Operation Timeouts

Each database operation has a configurable timeout to prevent hanging requests.

```yaml
database:
  resilience:
    operationTimeout: "10s"  # Maximum time for any DB operation
```

## Repository Pattern

The repository pattern provides a clean abstraction over database operations while adding resilience features.

### Repository Structure

```go
type StorageRepository struct {
    db *ResilientDB  // Wraps GORM with resilience
}

type ReplicateJobRepository struct {
    db *ResilientDB  // Wraps GORM with resilience
}
```

### Repository Methods

All repository methods use the resilient wrapper:

```go
func (r *StorageRepository) GetByID(ctx context.Context, id uuid.UUID) (*Storage, error) {
    var s Storage
    var result *Storage
    
    err := r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
        if err := r.db.GetDB().WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                result = nil
                return nil
            }
            return err
        }
        result = &s
        return nil
    })
    
    return result, err
}
```

### Available Repositories

#### StorageRepository

- `GetByID(ctx, id)` - Get storage by UUID
- `GetByName(ctx, name)` - Get storage by name
- `GetByProjectID(ctx, projectID)` - Get all storages for a project

#### ReplicateJobRepository

- `GetByID(ctx, id)` - Get job with preloaded storages
- `GetByProjectID(ctx, projectID)` - Get all jobs for a project
- `UpdateStatus(ctx, id, status)` - Update job status
- `UpdateStatusWithReason(ctx, id, status, reason)` - Update status with reason
- `Create(ctx, job)` - Create new job

## Configuration

### Default Configuration

```yaml
database:
  resilience:
    # Retry configuration
    maxRetries: 3
    retryDelay: "100ms"
    retryMultiplier: 2.0
    maxRetryDelay: "5s"
    
    # Circuit breaker configuration
    failureThreshold: 5
    recoveryTimeout: "30s"
    halfOpenMaxCalls: 3
    
    # Timeout configuration
    operationTimeout: "10s"
```

### Environment Variables

You can override configuration using environment variables:

```bash
# Retry configuration
DB_RESILIENCE_MAX_RETRIES=5
DB_RESILIENCE_RETRY_DELAY=200ms
DB_RESILIENCE_RETRY_MULTIPLIER=1.5
DB_RESILIENCE_MAX_RETRY_DELAY=10s

# Circuit breaker configuration
DB_RESILIENCE_FAILURE_THRESHOLD=10
DB_RESILIENCE_RECOVERY_TIMEOUT=60s
DB_RESILIENCE_HALF_OPEN_MAX_CALLS=5

# Timeout configuration
DB_RESILIENCE_OPERATION_TIMEOUT=15s
```

## Usage Examples

### Basic Repository Usage

```go
// Create repository with resilience
config := db.DefaultResilienceConfig()
storageRepo := db.NewStorageRepository(gormDB, config, logger)

// Get storage - automatically retries on transient failures
storage, err := storageRepo.GetByID(ctx, storageID)
if err != nil {
    log.Error().Err(err).Msg("Failed to get storage")
    return err
}
```

### Custom Resilience Configuration

```go
// Custom resilience configuration
config := db.ResilienceConfig{
    MaxRetries:        5,
    RetryDelay:        200 * time.Millisecond,
    RetryMultiplier:   1.5,
    MaxRetryDelay:     10 * time.Second,
    FailureThreshold:  10,
    RecoveryTimeout:   60 * time.Second,
    HalfOpenMaxCalls:  5,
    OperationTimeout:  15 * time.Second,
}

storageRepo := db.NewStorageRepository(gormDB, config, logger)
```

### Error Handling

```go
storage, err := storageRepo.GetByID(ctx, storageID)
if err != nil {
    // Check if it's a circuit breaker error
    if strings.Contains(err.Error(), "circuit breaker is open") {
        log.Warn().Msg("Database circuit breaker is open, service temporarily unavailable")
        return errors.New("service temporarily unavailable")
    }
    
    // Check if it's a retry exhaustion error
    if strings.Contains(err.Error(), "failed after") {
        log.Error().Err(err).Msg("Database operation failed after all retries")
        return errors.New("database operation failed")
    }
    
    // Other errors
    log.Error().Err(err).Msg("Database operation failed")
    return err
}
```

## Monitoring and Observability

### Logging

The resilience layer provides structured logging for all operations:

```json
{
  "level": "warn",
  "time": "2025-01-27T10:30:00Z",
  "message": "Database operation failed, retrying",
  "error": "connection refused",
  "attempt": 2,
  "delay": "200ms"
}
```

```json
{
  "level": "warn",
  "time": "2025-01-27T10:30:00Z",
  "message": "Circuit breaker transitioning to open state",
  "failures": 5
}
```

### Metrics (Future Enhancement)

Planned metrics for monitoring:

- `db_operations_total` - Total database operations
- `db_operations_duration_seconds` - Operation duration histogram
- `db_retries_total` - Total retry attempts
- `db_circuit_breaker_state` - Current circuit breaker state
- `db_circuit_breaker_failures_total` - Total circuit breaker failures

## Best Practices

### 1. Context Usage

Always pass context with appropriate timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := repo.GetByID(ctx, id)
```

### 2. Error Handling

Handle different types of errors appropriately:

```go
result, err := repo.GetByID(ctx, id)
if err != nil {
    if errors.Is(err, gorm.ErrRecordNotFound) {
        // Handle not found case
        return nil, nil
    }
    
    if strings.Contains(err.Error(), "circuit breaker") {
        // Handle circuit breaker
        return nil, errors.New("service temporarily unavailable")
    }
    
    // Handle other errors
    return nil, fmt.Errorf("database error: %w", err)
}
```

### 3. Configuration Tuning

Tune configuration based on your environment:

- **Development**: Lower thresholds for faster feedback
- **Production**: Higher thresholds for stability
- **High-load**: Increase timeouts and retry counts
- **Low-latency**: Decrease timeouts and retry counts

### 4. Monitoring

Monitor these key metrics:

- Circuit breaker state changes
- Retry attempt counts
- Operation duration percentiles
- Error rates by type

## Troubleshooting

### Common Issues

#### Circuit Breaker Stuck Open

**Symptoms**: All database operations fail with "circuit breaker is open"

**Causes**:
- Database is actually down
- Network connectivity issues
- Configuration too aggressive

**Solutions**:
- Check database connectivity
- Verify network configuration
- Adjust `failureThreshold` and `recoveryTimeout`

#### High Retry Counts

**Symptoms**: Many retry attempts in logs

**Causes**:
- Database performance issues
- Network instability
- Connection pool exhaustion

**Solutions**:
- Check database performance
- Increase connection pool size
- Adjust retry configuration

#### Operation Timeouts

**Symptoms**: Operations fail with timeout errors

**Causes**:
- Slow database queries
- Network latency
- Database locks

**Solutions**:
- Optimize database queries
- Increase `operationTimeout`
- Check for database locks

### Debug Configuration

Enable debug logging to troubleshoot issues:

```yaml
database:
  logLevel: "info"  # Change from "warn" to "info" for more details
```

## Future Enhancements

### Planned Features

1. **Metrics Integration**: Prometheus metrics for monitoring
2. **Distributed Circuit Breaker**: Shared state across instances
3. **Adaptive Timeouts**: Dynamic timeout adjustment based on performance
4. **Health Checks**: Proactive database health monitoring
5. **Graceful Degradation**: Fallback mechanisms for critical operations

### Configuration Evolution

Future configuration options:

```yaml
database:
  resilience:
    # Advanced retry configuration
    retryJitter: true          # Add randomness to retry delays
    retryMaxJitter: "1s"       # Maximum jitter amount
    
    # Advanced circuit breaker
    successThreshold: 3        # Successes needed to close circuit
    volumeThreshold: 10        # Minimum requests before circuit opens
    
    # Health check integration
    healthCheckInterval: "30s" # Health check frequency
    healthCheckTimeout: "5s"   # Health check timeout
```

This resilience layer ensures that Chorus can handle database issues gracefully while maintaining system stability and providing clear observability into the health of database operations.
