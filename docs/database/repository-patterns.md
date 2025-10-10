# Repository Patterns and Data Models

This document describes the repository patterns, data models, and database architecture used in the Chorus project.

## Overview

The repository pattern provides a clean abstraction layer between the business logic and data persistence layer. It encapsulates database operations and provides a consistent interface for data access while adding resilience features.

## Repository Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Business      │───▶│   Repository     │───▶│   Database      │
│   Logic         │    │   Layer          │    │   (PostgreSQL)  │
│                 │    │                  │    │                 │
│                 │    │  • ResilientDB   │    │                 │
│                 │    │  • Circuit       │    │                 │
│                 │    │    Breaker       │    │                 │
│                 │    │  • Retry Logic   │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Data Models

### Storage Model

The `Storage` model represents S3-compatible storage endpoints in the system.

```go
type Storage struct {
    ID                    uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    Name                  string    `gorm:"type:varchar(255);not null"`
    ProjectID             uuid.UUID `gorm:"type:uuid;not null;index"`
    Address               string    `gorm:"type:varchar(500);not null"`
    Provider              string    `gorm:"type:varchar(100);not null"`
    IsMain                bool      `gorm:"not null;default:false"`
    IsSecure              bool      `gorm:"not null;default:true"`
    DefaultRegion         string    `gorm:"type:varchar(100);not null"`
    HealthCheckIntervalMs int64     `gorm:"not null;default:10000"`
    HealthCheckEnabled    bool      `gorm:"not null;default:false"`
    HttpTimeoutMs         int64     `gorm:"not null;default:60000"`
    RateLimitEnabled      bool      `gorm:"not null;default:false"`
    RateLimitRPM          int       `gorm:"not null;default:60"`
    AccessKeyID           string    `gorm:"type:varchar(500);not null"`
    SecretAccessKey       string    `gorm:"type:varchar(500);not null"`
    CreatedAt             time.Time `gorm:"autoCreateTime"`
    UpdatedAt             time.Time `gorm:"autoUpdateTime"`
}
```

#### Storage Fields

- **ID**: Unique identifier (UUID)
- **Name**: Human-readable name for the storage
- **ProjectID**: Associated project identifier
- **Address**: S3 endpoint URL (e.g., `s3.amazonaws.com`)
- **Provider**: Storage provider type (`AWS`, `Ceph`, `Minio`, etc.)
- **IsMain**: Whether this is the primary storage for the project
- **IsSecure**: Whether to use HTTPS/TLS
- **DefaultRegion**: Default AWS region for operations
- **HealthCheckIntervalMs**: Health check frequency in milliseconds
- **HealthCheckEnabled**: Whether health checks are enabled
- **HttpTimeoutMs**: HTTP request timeout in milliseconds
- **RateLimitEnabled**: Whether rate limiting is enabled
- **RateLimitRPM**: Rate limit in requests per minute
- **AccessKeyID**: S3 access key ID
- **SecretAccessKey**: S3 secret access key

### ReplicateJob Model

The `ReplicateJob` model represents replication jobs in the system.

```go
type ReplicateJob struct {
    ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    ProjectID   uuid.UUID  `gorm:"type:uuid;not null;index"`
    Bucket      string     `gorm:"type:varchar(255);not null"`
    ToBucket    string     `gorm:"type:varchar(255)"`
    Status      string     `gorm:"type:varchar(50);not null;default:'pending'"`
    StatusReason string    `gorm:"type:varchar(255)"`
    FromStorageID *uuid.UUID `gorm:"type:uuid;index"`
    ToStorageID   *uuid.UUID `gorm:"type:uuid;index"`
    FromStorage   *Storage   `gorm:"foreignKey:FromStorageID"`
    ToStorage     *Storage   `gorm:"foreignKey:ToStorageID"`
    CreatedAt   time.Time  `gorm:"autoCreateTime"`
    UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}
```

#### ReplicateJob Fields

- **ID**: Unique job identifier (UUID)
- **ProjectID**: Associated project identifier
- **Bucket**: Source bucket name
- **ToBucket**: Destination bucket name (defaults to source bucket if empty)
- **Status**: Job status (`pending`, `running`, `completed`, `failed`)
- **StatusReason**: Human-readable reason for status (especially for failures)
- **FromStorageID**: Source storage identifier
- **ToStorageID**: Destination storage identifier
- **FromStorage**: Preloaded source storage object
- **ToStorage**: Preloaded destination storage object

## Repository Interfaces

### StorageRepository

Manages storage endpoint data.

```go
type StorageRepository struct {
    db *ResilientDB
}

// GetByID retrieves a storage by its UUID
func (r *StorageRepository) GetByID(ctx context.Context, id uuid.UUID) (*Storage, error)

// GetByName retrieves a storage by its name
func (r *StorageRepository) GetByName(ctx context.Context, name string) (*Storage, error)

// GetByProjectID retrieves all storages for a project
func (r *StorageRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*Storage, error)
```

### ReplicateJobRepository

Manages replication job data.

```go
type ReplicateJobRepository struct {
    db *ResilientDB
}

// GetByID retrieves a job with preloaded storage relationships
func (r *ReplicateJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*ReplicateJob, error)

// GetByProjectID retrieves all jobs for a project
func (r *ReplicateJobRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*ReplicateJob, error)

// UpdateStatus updates the job status
func (r *ReplicateJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error

// UpdateStatusWithReason updates the job status with a reason
func (r *ReplicateJobRepository) UpdateStatusWithReason(ctx context.Context, id uuid.UUID, status, reason string) error

// Create creates a new replication job
func (r *ReplicateJobRepository) Create(ctx context.Context, job *ReplicateJob) error
```

## Database Schema

### Storage Table

```sql
CREATE TABLE storage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    project_id UUID NOT NULL,
    address VARCHAR(500) NOT NULL,
    provider VARCHAR(100) NOT NULL,
    is_main BOOLEAN NOT NULL DEFAULT FALSE,
    is_secure BOOLEAN NOT NULL DEFAULT TRUE,
    default_region VARCHAR(100) NOT NULL,
    health_check_interval_ms BIGINT NOT NULL DEFAULT 10000,
    health_check_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    http_timeout_ms BIGINT NOT NULL DEFAULT 60000,
    rate_limit_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    rate_limit_rpm INTEGER NOT NULL DEFAULT 60,
    access_key_id VARCHAR(500) NOT NULL,
    secret_access_key VARCHAR(500) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_storage_project_id ON storage(project_id);
```

### ReplicateJob Table

```sql
CREATE TABLE replicate_job (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    bucket VARCHAR(255) NOT NULL,
    to_bucket VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    status_reason VARCHAR(255),
    from_storage_id UUID,
    to_storage_id UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    FOREIGN KEY (from_storage_id) REFERENCES storage(id),
    FOREIGN KEY (to_storage_id) REFERENCES storage(id)
);

CREATE INDEX idx_replicate_job_project_id ON replicate_job(project_id);
CREATE INDEX idx_replicate_job_from_storage_id ON replicate_job(from_storage_id);
CREATE INDEX idx_replicate_job_to_storage_id ON replicate_job(to_storage_id);
```

## Usage Patterns

### Creating Repositories

```go
// Create repositories with resilience configuration
config := db.DefaultResilienceConfig()
logger := zerolog.New(os.Stdout)

storageRepo := db.NewStorageRepository(gormDB, config, logger)
jobRepo := db.NewReplicateJobRepository(gormDB, config, logger)
```

### Basic CRUD Operations

```go
// Create a new storage
storage := &db.Storage{
    Name:           "main-storage",
    ProjectID:      projectID,
    Address:        "s3.amazonaws.com",
    Provider:       "AWS",
    IsMain:         true,
    DefaultRegion:  "us-east-1",
    AccessKeyID:    "AKIA...",
    SecretAccessKey: "secret...",
}

err := storageRepo.Create(ctx, storage)
if err != nil {
    return fmt.Errorf("failed to create storage: %w", err)
}

// Get storage by ID
storage, err := storageRepo.GetByID(ctx, storageID)
if err != nil {
    return fmt.Errorf("failed to get storage: %w", err)
}

// Update storage status
err = storageRepo.UpdateStatus(ctx, storageID, "active")
if err != nil {
    return fmt.Errorf("failed to update storage: %w", err)
}
```

### Complex Queries with Preloading

```go
// Get job with preloaded storage relationships
job, err := jobRepo.GetByID(ctx, jobID)
if err != nil {
    return fmt.Errorf("failed to get job: %w", err)
}

// Access preloaded relationships
fmt.Printf("Job %s replicates from %s to %s\n", 
    job.ID, 
    job.FromStorage.Name, 
    job.ToStorage.Name)
```

### Batch Operations

```go
// Get all jobs for a project
jobs, err := jobRepo.GetByProjectID(ctx, projectID)
if err != nil {
    return fmt.Errorf("failed to get jobs: %w", err)
}

// Process each job
for _, job := range jobs {
    if job.Status == "pending" {
        // Start processing
        err := jobRepo.UpdateStatus(ctx, job.ID, "running")
        if err != nil {
            log.Error().Err(err).Str("job_id", job.ID.String()).Msg("Failed to update job status")
        }
    }
}
```

## Error Handling Patterns

### Repository Error Handling

```go
func (r *StorageRepository) GetByID(ctx context.Context, id uuid.UUID) (*Storage, error) {
    var s Storage
    var result *Storage
    
    err := r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
        if err := r.db.GetDB().WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                result = nil
                return nil  // Not found is not an error
            }
            return err
        }
        result = &s
        return nil
    })
    
    return result, err
}
```

### Business Logic Error Handling

```go
storage, err := storageRepo.GetByID(ctx, storageID)
if err != nil {
    // Handle different error types
    if strings.Contains(err.Error(), "circuit breaker is open") {
        return errors.New("storage service temporarily unavailable")
    }
    
    if strings.Contains(err.Error(), "failed after") {
        return errors.New("storage lookup failed after retries")
    }
    
    return fmt.Errorf("failed to get storage: %w", err)
}

if storage == nil {
    return errors.New("storage not found")
}
```

## Configuration Management

### Database Configuration

```yaml
database:
  # Connection settings
  host: "localhost"
  port: 5432
  user: "chorus"
  password: "secret"
  name: "chorus"
  sslmode: "disable"
  
  # Connection pool settings
  maxOpenConns: 20
  maxIdleConns: 10
  connMaxLifetime: "1h"
  connMaxIdleTime: "15m"
  
  # Resilience settings
  resilience:
    maxRetries: 3
    retryDelay: "100ms"
    retryMultiplier: 2.0
    maxRetryDelay: "5s"
    failureThreshold: 5
    recoveryTimeout: "30s"
    halfOpenMaxCalls: 3
    operationTimeout: "10s"
```

### Environment Variables

```bash
# Database connection
DB_HOST=localhost
DB_PORT=5432
DB_USER=chorus
DB_PASSWORD=secret
DB_NAME=chorus
DB_SSLMODE=disable

# Connection pool
DB_MAX_OPEN_CONNS=20
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=1h
DB_CONN_MAX_IDLE_TIME=15m

# Resilience
DB_RESILIENCE_MAX_RETRIES=3
DB_RESILIENCE_RETRY_DELAY=100ms
DB_RESILIENCE_FAILURE_THRESHOLD=5
DB_RESILIENCE_OPERATION_TIMEOUT=10s
```

## Testing Patterns

### Unit Testing Repositories

```go
func TestStorageRepository_GetByID(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    // Create test data
    storage := &db.Storage{
        Name: "test-storage",
        // ... other fields
    }
    err := db.Create(storage).Error
    require.NoError(t, err)
    
    // Create repository
    config := db.DefaultResilienceConfig()
    repo := db.NewStorageRepository(db, config, zerolog.Nop())
    
    // Test GetByID
    result, err := repo.GetByID(context.Background(), storage.ID)
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, storage.ID, result.ID)
    assert.Equal(t, "test-storage", result.Name)
}
```

### Integration Testing

```go
func TestStorageRepository_Integration(t *testing.T) {
    // Setup real database connection
    db := setupIntegrationDB(t)
    defer cleanupIntegrationDB(t, db)
    
    // Test with real database
    config := db.DefaultResilienceConfig()
    repo := db.NewStorageRepository(db, config, zerolog.Nop())
    
    // Test operations
    storage, err := repo.GetByID(context.Background(), testStorageID)
    assert.NoError(t, err)
    assert.NotNil(t, storage)
}
```

## Best Practices

### 1. Context Usage

Always pass context with appropriate timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := repo.GetByID(ctx, id)
```

### 2. Error Handling

Handle different error types appropriately:

```go
result, err := repo.GetByID(ctx, id)
if err != nil {
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, nil  // Not found is not an error
    }
    
    if strings.Contains(err.Error(), "circuit breaker") {
        return nil, errors.New("service temporarily unavailable")
    }
    
    return nil, fmt.Errorf("database error: %w", err)
}
```

### 3. Transaction Management

Use transactions for multi-step operations:

```go
err := db.Transaction(func(tx *gorm.DB) error {
    // Create storage
    if err := tx.Create(storage).Error; err != nil {
        return err
    }
    
    // Create job
    if err := tx.Create(job).Error; err != nil {
        return err
    }
    
    return nil
})
```

### 4. Preloading Relationships

Use preloading for related data:

```go
// In repository method
err := r.db.GetDB().WithContext(ctx).
    Preload("FromStorage").
    Preload("ToStorage").
    First(&job, "id = ?", id).Error
```

### 5. Indexing Strategy

Ensure proper database indexes:

```sql
-- Primary indexes
CREATE INDEX idx_storage_project_id ON storage(project_id);
CREATE INDEX idx_replicate_job_project_id ON replicate_job(project_id);

-- Foreign key indexes
CREATE INDEX idx_replicate_job_from_storage_id ON replicate_job(from_storage_id);
CREATE INDEX idx_replicate_job_to_storage_id ON replicate_job(to_storage_id);

-- Status indexes for common queries
CREATE INDEX idx_replicate_job_status ON replicate_job(status);
```

## Migration Strategy

### Schema Migrations

Use GORM AutoMigrate for development:

```go
err := db.AutoMigrate(
    &db.Storage{},
    &db.ReplicateJob{},
)
```

For production, use proper migration scripts:

```sql
-- Migration: Add status_reason column
ALTER TABLE replicate_job ADD COLUMN status_reason VARCHAR(255);

-- Migration: Add health_check_enabled column
ALTER TABLE storage ADD COLUMN health_check_enabled BOOLEAN NOT NULL DEFAULT FALSE;
```

### Data Migrations

```go
// Migrate existing data
func migrateData(db *gorm.DB) error {
    // Update existing storages
    err := db.Model(&db.Storage{}).
        Where("health_check_enabled IS NULL").
        Update("health_check_enabled", false).Error
    if err != nil {
        return err
    }
    
    return nil
}
```

This repository pattern provides a robust, resilient, and maintainable data access layer for the Chorus project, with clear separation of concerns and comprehensive error handling.
