# Quick Reference Guide

This guide provides quick reference information for developers working with the Chorus database layer and repository patterns.

## Repository Quick Start

### Creating Repositories

```go
import (
    "github.com/clyso/chorus/pkg/repository/db"
    "github.com/rs/zerolog"
)

// Create repositories
config := db.DefaultResilienceConfig()
logger := zerolog.New(os.Stdout)

storageRepo := db.NewStorageRepository(gormDB, config, logger)
jobRepo := db.NewReplicateJobRepository(gormDB, config, logger)
```

### Basic Operations

```go
// Get storage by ID
storage, err := storageRepo.GetByID(ctx, storageID)
if err != nil {
    return err
}

// Get storage by name
storage, err := storageRepo.GetByName(ctx, "main-storage")
if err != nil {
    return err
}

// Get all storages for project
storages, err := storageRepo.GetByProjectID(ctx, projectID)
if err != nil {
    return err
}

// Get job with preloaded storages
job, err := jobRepo.GetByID(ctx, jobID)
if err != nil {
    return err
}

// Update job status
err = jobRepo.UpdateStatus(ctx, jobID, "running")
if err != nil {
    return err
}

// Update job status with reason
err = jobRepo.UpdateStatusWithReason(ctx, jobID, "failed", "S3 connection timeout")
if err != nil {
    return err
}
```

## Configuration Quick Reference

### Default Resilience Configuration

```yaml
database:
  resilience:
    maxRetries: 3           # Retry failed operations 3 times
    retryDelay: "100ms"     # Initial delay between retries
    retryMultiplier: 2.0    # Exponential backoff (100ms, 200ms, 400ms)
    maxRetryDelay: "5s"     # Maximum delay between retries
    failureThreshold: 5     # Circuit opens after 5 failures
    recoveryTimeout: "30s"  # Wait 30s before testing recovery
    halfOpenMaxCalls: 3     # Allow 3 test calls in half-open state
    operationTimeout: "10s" # Maximum time for any DB operation
```

### Environment Variables

```bash
# Database connection
DB_HOST=localhost
DB_PORT=5432
DB_USER=chorus
DB_PASSWORD=secret
DB_NAME=chorus

# Resilience tuning
DB_RESILIENCE_MAX_RETRIES=5
DB_RESILIENCE_RETRY_DELAY=200ms
DB_RESILIENCE_FAILURE_THRESHOLD=10
DB_RESILIENCE_OPERATION_TIMEOUT=15s
```

## Error Handling Quick Reference

### Common Error Patterns

```go
// Check for not found
if storage == nil {
    return errors.New("storage not found")
}

// Check for circuit breaker
if strings.Contains(err.Error(), "circuit breaker is open") {
    return errors.New("service temporarily unavailable")
}

// Check for retry exhaustion
if strings.Contains(err.Error(), "failed after") {
    return errors.New("operation failed after retries")
}

// Check for context cancellation
if errors.Is(err, context.Canceled) {
    return errors.New("operation cancelled")
}
```

### Error Handling Template

```go
func (s *Service) GetStorage(ctx context.Context, id uuid.UUID) (*Storage, error) {
    storage, err := s.storageRepo.GetByID(ctx, id)
    if err != nil {
        // Handle circuit breaker
        if strings.Contains(err.Error(), "circuit breaker is open") {
            return nil, errors.New("storage service temporarily unavailable")
        }
        
        // Handle retry exhaustion
        if strings.Contains(err.Error(), "failed after") {
            return nil, errors.New("storage lookup failed after retries")
        }
        
        // Handle other errors
        return nil, fmt.Errorf("failed to get storage: %w", err)
    }
    
    // Handle not found
    if storage == nil {
        return nil, errors.New("storage not found")
    }
    
    return storage, nil
}
```

## Data Models Quick Reference

### Storage Model

```go
type Storage struct {
    ID                    uuid.UUID `gorm:"primaryKey"`
    Name                  string    `gorm:"not null"`
    ProjectID             uuid.UUID `gorm:"not null;index"`
    Address               string    `gorm:"not null"`
    Provider              string    `gorm:"not null"`
    Type                  string    `gorm:"size:64;default:'both'"`
    IsSecure              bool      `gorm:"default:true"`
    DefaultRegion         string    `gorm:"not null"`
    HealthCheckIntervalMs int64     `gorm:"default:10000"`
    HealthCheckEnabled    bool      `gorm:"default:false"`
    HttpTimeoutMs         int64     `gorm:"default:60000"`
    RateLimitEnabled      bool      `gorm:"default:false"`
    RateLimitRPM          int       `gorm:"default:60"`
    AccessKeyID           string    `gorm:"not null"`
    SecretAccessKey       string    `gorm:"not null"`
    CreatedAt             time.Time `gorm:"autoCreateTime"`
    UpdatedAt             time.Time `gorm:"autoUpdateTime"`
}
```

### ReplicateJob Model

```go
type ReplicateJob struct {
    ID           uuid.UUID  `gorm:"primaryKey"`
    ProjectID    uuid.UUID  `gorm:"not null;index"`
    Bucket       string     `gorm:"not null"`
    ToBucket     string
    Status       string     `gorm:"not null;default:'pending'"`
    StatusReason string
    FromStorageID *uuid.UUID `gorm:"index"`
    ToStorageID   *uuid.UUID `gorm:"index"`
    FromStorage   *Storage   `gorm:"foreignKey:FromStorageID"`
    ToStorage     *Storage   `gorm:"foreignKey:ToStorageID"`
    CreatedAt    time.Time  `gorm:"autoCreateTime"`
    UpdatedAt    time.Time  `gorm:"autoUpdateTime"`
}
```

## Testing Quick Reference

### Unit Test Setup

```go
func TestStorageRepository(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    // Create repository
    config := db.DefaultResilienceConfig()
    repo := db.NewStorageRepository(db, config, zerolog.Nop())
    
    // Test operations
    storage, err := repo.GetByID(context.Background(), testID)
    assert.NoError(t, err)
    assert.NotNil(t, storage)
}
```

### Integration Test Setup

```go
func TestStorageRepository_Integration(t *testing.T) {
    // Setup real database
    db := setupIntegrationDB(t)
    defer cleanupIntegrationDB(t, db)
    
    // Test with real database
    config := db.DefaultResilienceConfig()
    repo := db.NewStorageRepository(db, config, zerolog.Nop())
    
    // Test operations
    storage, err := repo.GetByID(context.Background(), testID)
    assert.NoError(t, err)
}
```

## Logging Quick Reference

### Structured Logging

```go
// Repository operations are automatically logged
// Example log output:
{
  "level": "warn",
  "time": "2025-01-27T10:30:00Z",
  "message": "Database operation failed, retrying",
  "error": "connection refused",
  "attempt": 2,
  "delay": "200ms"
}
```

### Custom Logging

```go
// Add custom logging in your service layer
logger.Info().
    Str("storage_id", storageID.String()).
    Str("operation", "get_storage").
    Msg("Retrieving storage")

storage, err := storageRepo.GetByID(ctx, storageID)
if err != nil {
    logger.Error().
        Err(err).
        Str("storage_id", storageID.String()).
        Msg("Failed to retrieve storage")
    return err
}

logger.Info().
    Str("storage_id", storageID.String()).
    Str("storage_name", storage.Name).
    Msg("Successfully retrieved storage")
```

## Performance Tuning Quick Reference

### Connection Pool Tuning

```yaml
database:
  maxOpenConns: 20      # Maximum open connections
  maxIdleConns: 10      # Maximum idle connections
  connMaxLifetime: "1h" # Connection lifetime
  connMaxIdleTime: "15m" # Idle connection timeout
```

### Resilience Tuning

```yaml
database:
  resilience:
    # For high-load environments
    maxRetries: 5
    retryDelay: "50ms"
    maxRetryDelay: "2s"
    operationTimeout: "5s"
    
    # For low-latency environments
    maxRetries: 2
    retryDelay: "200ms"
    maxRetryDelay: "1s"
    operationTimeout: "3s"
    
    # For unstable networks
    maxRetries: 10
    retryDelay: "500ms"
    maxRetryDelay: "30s"
    operationTimeout: "60s"
```

## Troubleshooting Quick Reference

### Common Issues

| Issue | Symptoms | Solution |
|-------|----------|----------|
| Circuit breaker open | All operations fail with "circuit breaker is open" | Check database connectivity, adjust `failureThreshold` |
| High retry counts | Many retry attempts in logs | Check database performance, increase connection pool |
| Operation timeouts | Operations fail with timeout errors | Optimize queries, increase `operationTimeout` |
| Connection pool exhaustion | "too many connections" errors | Increase `maxOpenConns`, check for connection leaks |

### Debug Configuration

```yaml
database:
  logLevel: "info"  # Change from "warn" to "info" for more details
```

### Health Check Commands

```bash
# Check database connectivity
psql -h localhost -U chorus -d chorus -c "SELECT 1"

# Check connection pool status
# (Add monitoring queries to your application)

# Check circuit breaker status
# (Add metrics endpoint to your application)
```

## Migration Quick Reference

### Schema Changes

```go
// Add new column
err := db.AutoMigrate(&db.Storage{})
if err != nil {
    return err
}

// Or use SQL migration
// ALTER TABLE storage ADD COLUMN new_field VARCHAR(255);
```

### Data Migration

```go
// Update existing data
err := db.Model(&db.Storage{}).
    Where("new_field IS NULL").
    Update("new_field", "default_value").Error
if err != nil {
    return err
}
```

This quick reference guide provides essential information for developers working with the Chorus database layer and repository patterns.
