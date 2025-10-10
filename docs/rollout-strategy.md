# Database-Backed Replication Rollout Strategy

This document outlines the safe, gradual rollout plan for the database-backed replication system in Chorus.

## Overview

The rollout strategy ensures a smooth transition from YAML-based configuration to database-backed replication jobs while maintaining system stability and providing rollback capabilities.

## Rollout Phases

### Phase 1: Development Environment ✅

**Objective**: Validate functionality in controlled environment

**Configuration**:
```yaml
# docker-compose/worker-conf-dev.yaml
database:
  useForReplication: true
  resilience:
    maxRetries: 3
    failureThreshold: 5
    operationTimeout: "10s"
```

**Tasks**:
- [x] Enable `useForReplication: true` in development
- [x] Test with sample replication jobs
- [x] Validate CLI `--job-id` functionality
- [x] Test gRPC API with `job_id` parameter
- [x] Verify resilience features (retries, circuit breaker)
- [x] Test fallback to YAML when database unavailable

**Success Criteria**:
- All replication jobs execute successfully
- CLI commands work with both `--job-id` and traditional flags
- gRPC API accepts both `job_id` and traditional parameters
- System gracefully handles database connectivity issues

### Phase 2: Staging Environment

**Objective**: Integration testing and performance validation

**Configuration**:
```yaml
# docker-compose/worker-conf-staging.yaml
database:
  useForReplication: true
  resilience:
    maxRetries: 5
    failureThreshold: 10
    operationTimeout: "15s"
  # Staging-specific settings
  maxOpenConns: 30
  maxIdleConns: 15
```

**Tasks**:
- [ ] Deploy to staging environment
- [ ] Run comprehensive integration tests
- [ ] Performance testing with realistic data volumes
- [ ] Load testing with concurrent replication jobs
- [ ] Test database failover scenarios
- [ ] Validate monitoring and alerting

**Success Criteria**:
- Integration tests pass
- Performance meets requirements
- System handles expected load
- Monitoring provides adequate visibility

### Phase 3: Production Canary

**Objective**: Gradual production rollout with monitoring

**Configuration**:
```yaml
# docker-compose/worker-conf-canary.yaml
database:
  useForReplication: true
  resilience:
    maxRetries: 3
    failureThreshold: 5
    operationTimeout: "10s"
  # Production-ready settings
  maxOpenConns: 50
  maxIdleConns: 25
  connMaxLifetime: "2h"
  connMaxIdleTime: "30m"
```

**Tasks**:
- [ ] Deploy to canary environment (10% traffic)
- [ ] Monitor key metrics
- [ ] Test with real production data
- [ ] Validate error handling
- [ ] Document any issues

**Success Criteria**:
- Error rate < 0.1%
- Response time < 2s for 95th percentile
- No database connection issues
- All replication jobs complete successfully

### Phase 4: Full Production

**Objective**: Complete migration to database-backed replication

**Configuration**:
```yaml
# docker-compose/worker-conf-prod.yaml
database:
  useForReplication: true
  resilience:
    maxRetries: 3
    failureThreshold: 5
    operationTimeout: "10s"
  # Production-optimized settings
  maxOpenConns: 100
  maxIdleConns: 50
  connMaxLifetime: "4h"
  connMaxIdleTime: "1h"
```

**Tasks**:
- [ ] Deploy to full production
- [ ] Monitor system stability
- [ ] Plan YAML configuration deprecation
- [ ] Update documentation
- [ ] Train operations team

**Success Criteria**:
- System stable for 7 days
- All replication jobs using database-backed config
- YAML fallback working as expected
- Operations team trained

## Environment Configuration

### Development
```yaml
database:
  useForReplication: true
  host: "localhost"
  port: 5432
  user: "chorus_dev"
  password: "dev_password"
  name: "chorus_dev"
  sslmode: "disable"
  resilience:
    maxRetries: 3
    retryDelay: "100ms"
    failureThreshold: 5
    recoveryTimeout: "30s"
    operationTimeout: "10s"
```

### Staging
```yaml
database:
  useForReplication: true
  host: "staging-db.example.com"
  port: 5432
  user: "chorus_staging"
  password: "${DB_PASSWORD}"
  name: "chorus_staging"
  sslmode: "require"
  maxOpenConns: 30
  maxIdleConns: 15
  resilience:
    maxRetries: 5
    retryDelay: "200ms"
    failureThreshold: 10
    recoveryTimeout: "60s"
    operationTimeout: "15s"
```

### Production
```yaml
database:
  useForReplication: true
  host: "prod-db.example.com"
  port: 5432
  user: "chorus_prod"
  password: "${DB_PASSWORD}"
  name: "chorus_prod"
  sslmode: "require"
  maxOpenConns: 100
  maxIdleConns: 50
  connMaxLifetime: "4h"
  connMaxIdleTime: "1h"
  resilience:
    maxRetries: 3
    retryDelay: "100ms"
    failureThreshold: 5
    recoveryTimeout: "30s"
    operationTimeout: "10s"
```

## Monitoring & Alerting

### Key Metrics to Monitor

#### Database Metrics
- `db_operations_total` - Total database operations
- `db_operation_duration_seconds` - Operation duration
- `db_operation_errors_total` - Database operation errors
- `db_retries_total` - Retry attempts
- `db_circuit_breaker_state` - Circuit breaker state
- `db_connections_active` - Active connections
- `db_connections_idle` - Idle connections

#### Application Metrics
- `worker_processed_tasks_total` - Processed tasks
- `worker_failed_tasks_total` - Failed tasks
- `worker_task_duration_seconds` - Task duration
- `replication_jobs_total` - Total replication jobs
- `replication_jobs_failed_total` - Failed replication jobs

#### Business Metrics
- Replication job success rate
- Average replication time
- Data transfer throughput
- Error rates by error type

### Alerting Rules

#### Critical Alerts
```yaml
# Database connectivity
- alert: DatabaseDown
  expr: up{job="postgres"} == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Database is down"

# Circuit breaker open
- alert: CircuitBreakerOpen
  expr: db_circuit_breaker_state == 1
  for: 2m
  labels:
    severity: critical
  annotations:
    summary: "Database circuit breaker is open"

# High error rate
- alert: HighDatabaseErrorRate
  expr: rate(db_operation_errors_total[5m]) > 0.1
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High database error rate"
```

#### Warning Alerts
```yaml
# High retry rate
- alert: HighRetryRate
  expr: rate(db_retries_total[5m]) > 0.05
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "High database retry rate"

# Connection pool exhaustion
- alert: ConnectionPoolExhaustion
  expr: db_connections_active / db_connections_max > 0.8
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Database connection pool near exhaustion"
```

## Rollback Procedures

### Quick Rollback (Feature Flag)
```bash
# Disable database-backed replication
kubectl patch configmap worker-config -p '{"data":{"config.yaml":"database:\n  useForReplication: false"}}'

# Restart worker pods
kubectl rollout restart deployment/worker
```

### Emergency Rollback (Code Rollback)
```bash
# Rollback to previous version
kubectl rollout undo deployment/worker

# Verify rollback
kubectl rollout status deployment/worker
```

### Database Rollback
```bash
# If database schema changes need rollback
kubectl exec -it postgres-pod -- psql -U chorus -d chorus -f rollback-schema.sql
```

## Migration Strategy

### From YAML to Database

#### Step 1: Export Existing Configurations
```bash
# Export current YAML configurations
./scripts/export-yaml-configs.sh > yaml-configs-backup.json
```

#### Step 2: Transform to Database Schema
```bash
# Transform YAML configs to database records
./scripts/yaml-to-db-migration.sh yaml-configs-backup.json
```

#### Step 3: Validate Migration
```bash
# Validate migrated data
./scripts/validate-migration.sh
```

#### Step 4: Enable Database-Backed Config
```yaml
database:
  useForReplication: true
```

### Migration Scripts

#### Export Script
```bash
#!/bin/bash
# scripts/export-yaml-configs.sh

echo "Exporting YAML configurations..."

# Extract storage configurations
jq -n --argjson storages "$(yq eval '.storage.storages' config.yaml)" \
  --argjson jobs "$(yq eval '.replication.jobs' config.yaml)" \
  '{storages: $storages, jobs: $jobs}' > yaml-configs-backup.json

echo "Export completed: yaml-configs-backup.json"
```

#### Migration Script
```bash
#!/bin/bash
# scripts/yaml-to-db-migration.sh

CONFIG_FILE=$1
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_NAME=${DB_NAME:-chorus}
DB_USER=${DB_USER:-chorus}

echo "Migrating configurations to database..."

# Run migration
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f migration.sql

echo "Migration completed"
```

## Testing Strategy

### Unit Tests
- Repository layer tests
- Config loader tests
- Resilience feature tests
- CLI command tests
- gRPC API tests

### Integration Tests
- Database connectivity tests
- End-to-end replication workflow tests
- Fallback mechanism tests
- Performance tests

### Load Tests
- Concurrent replication job tests
- Database connection pool tests
- Memory usage tests
- CPU usage tests

### Chaos Engineering
- Database failure simulation
- Network partition tests
- Resource exhaustion tests
- Circuit breaker tests

## Success Criteria

### Phase 1 (Development)
- [x] All unit tests pass
- [x] Integration tests pass
- [x] CLI functionality works
- [x] gRPC API works
- [x] Resilience features work

### Phase 2 (Staging)
- [ ] Load tests pass
- [ ] Performance meets requirements
- [ ] Monitoring works
- [ ] Alerting works
- [ ] Documentation complete

### Phase 3 (Canary)
- [ ] Error rate < 0.1%
- [ ] Response time < 2s (95th percentile)
- [ ] No database issues
- [ ] All replication jobs succeed

### Phase 4 (Production)
- [ ] System stable for 7 days
- [ ] All traffic using database-backed config
- [ ] Operations team trained
- [ ] YAML deprecation planned

## Risk Mitigation

### Technical Risks
- **Database connectivity issues**: Circuit breaker and retry logic
- **Performance degradation**: Connection pooling and monitoring
- **Data corruption**: Transaction management and validation
- **Schema changes**: Migration scripts and rollback procedures

### Operational Risks
- **Team knowledge gap**: Training and documentation
- **Deployment issues**: Staged rollout and rollback procedures
- **Monitoring gaps**: Comprehensive alerting and dashboards
- **Support issues**: Runbooks and escalation procedures

## Timeline

### Week 1-2: Development
- [x] Implement database-backed replication
- [x] Add resilience features
- [x] Create CLI and gRPC support
- [x] Write unit tests

### Week 3: Staging
- [ ] Deploy to staging
- [ ] Run integration tests
- [ ] Performance testing
- [ ] Load testing

### Week 4: Canary
- [ ] Deploy to canary
- [ ] Monitor metrics
- [ ] Test with production data
- [ ] Document issues

### Week 5: Production
- [ ] Deploy to production
- [ ] Monitor stability
- [ ] Plan YAML deprecation
- [ ] Train operations team

## Conclusion

This rollout strategy ensures a safe, gradual transition to database-backed replication while maintaining system stability and providing comprehensive monitoring and rollback capabilities. The phased approach allows for thorough testing at each stage and minimizes risk to production systems.
