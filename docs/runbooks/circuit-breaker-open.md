# Circuit Breaker Open Runbook

## Overview
This runbook provides step-by-step instructions for handling circuit breaker open states in the Chorus database-backed replication system.

## Symptoms
- Prometheus alert: `CircuitBreakerOpen`
- Worker service logs showing "circuit breaker is open" messages
- Database operations being rejected
- High retry rates in database operations

## Understanding Circuit Breaker States

### States
- **Closed (0)**: Normal operation, requests pass through
- **Open (1)**: Circuit breaker is open, requests are rejected
- **Half-Open (2)**: Testing if service has recovered

### Configuration
```yaml
resilience:
  failureThreshold: 5      # Failures before opening
  recoveryTimeout: 30s     # Time before trying half-open
  halfOpenMaxCalls: 3      # Max calls in half-open state
```

## Immediate Actions

### 1. Check Circuit Breaker Status
```bash
# Check current state
kubectl logs deployment/worker | grep "circuit breaker" | tail -10

# Check Prometheus metrics
curl -s http://localhost:9090/api/v1/query?query=db_circuit_breaker_state
```

### 2. Identify Root Cause
```bash
# Check database connectivity
kubectl exec -it <worker-pod-name> -- nc -zv <db-host> 5432

# Check database logs
kubectl logs <postgres-pod-name> --tail=100

# Check worker logs for errors
kubectl logs deployment/worker | grep -E "(error|failed|timeout)" | tail -20
```

## Troubleshooting Steps

### Step 1: Database Connectivity Issues
If database is unreachable:

```bash
# Check database pod status
kubectl get pods -l app=postgres

# Check database service
kubectl get svc postgres
kubectl describe svc postgres

# Test database connection
kubectl exec -it <postgres-pod-name> -- psql -U chorus -d chorus -c "SELECT 1;"
```

### Step 2: Database Performance Issues
If database is slow or overloaded:

```bash
# Check database resource usage
kubectl top pod <postgres-pod-name>

# Check active connections
kubectl exec -it <postgres-pod-name> -- psql -U chorus -d chorus -c "
SELECT count(*) as active_connections 
FROM pg_stat_activity 
WHERE state = 'active';"

# Check slow queries
kubectl exec -it <postgres-pod-name> -- psql -U chorus -d chorus -c "
SELECT query, mean_time, calls 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;"
```

### Step 3: Network Issues
If network connectivity is problematic:

```bash
# Check network policies
kubectl get networkpolicies

# Test connectivity with different tools
kubectl exec -it <worker-pod-name> -- telnet <db-host> 5432
kubectl exec -it <worker-pod-name> -- nc -zv <db-host> 5432
```

## Recovery Actions

### 1. Fix Root Cause
Address the underlying issue:
- Restart database if needed
- Fix network connectivity
- Resolve performance issues
- Update configuration

### 2. Reset Circuit Breaker
Once the root cause is fixed:

```bash
# Restart worker pods to reset circuit breaker
kubectl rollout restart deployment/worker

# Monitor circuit breaker state
kubectl logs -f deployment/worker | grep "circuit breaker"
```

### 3. Verify Recovery
```bash
# Check circuit breaker state
curl -s http://localhost:9090/api/v1/query?query=db_circuit_breaker_state

# Test database operations
kubectl exec -it <worker-pod-name> -- psql -U chorus -d chorus -c "SELECT 1;"

# Monitor error rates
curl -s http://localhost:9090/api/v1/query?query=rate(db_operation_errors_total[5m])
```

## Manual Circuit Breaker Override

### Emergency Override
If you need to force the circuit breaker closed:

```bash
# Restart worker pods
kubectl delete pods -l app=worker

# Wait for pods to restart
kubectl get pods -l app=worker

# Verify circuit breaker is closed
kubectl logs deployment/worker | grep "circuit breaker"
```

### Configuration Override
Temporarily adjust circuit breaker settings:

```yaml
# In worker configuration
resilience:
  failureThreshold: 10     # Increase threshold
  recoveryTimeout: 60s     # Increase recovery time
  halfOpenMaxCalls: 5      # Increase half-open calls
```

## Prevention

### 1. Proper Configuration
Ensure circuit breaker settings are appropriate:

```yaml
resilience:
  failureThreshold: 5      # Not too low
  recoveryTimeout: 30s     # Reasonable recovery time
  halfOpenMaxCalls: 3      # Conservative testing
  operationTimeout: 10s    # Reasonable timeout
```

### 2. Database Health
Maintain database health:
- Regular maintenance
- Proper resource allocation
- Monitoring and alerting
- Backup and recovery procedures

### 3. Network Stability
Ensure network stability:
- Proper network policies
- Load balancing
- Redundancy
- Monitoring

## Monitoring

### Key Metrics
- `db_circuit_breaker_state` - Current state
- `db_operation_errors_total` - Error count
- `db_retries_total` - Retry count
- `db_operation_duration_seconds` - Operation duration

### Alerts
```yaml
- alert: CircuitBreakerOpen
  expr: db_circuit_breaker_state == 1
  for: 2m
  labels:
    severity: critical

- alert: CircuitBreakerHalfOpen
  expr: db_circuit_breaker_state == 2
  for: 1m
  labels:
    severity: warning
```

## Escalation

### When to Escalate
- Circuit breaker has been open for more than 10 minutes
- Multiple circuit breaker resets are needed
- Root cause is not immediately apparent
- System is experiencing cascading failures

### Escalation Contacts
- **Primary**: Database Team (db-team@example.com)
- **Secondary**: Platform Team (platform-team@example.com)
- **Emergency**: On-call Engineer (oncall@example.com)

## Post-Incident

### 1. Root Cause Analysis
- Document the root cause
- Identify contributing factors
- Review circuit breaker configuration
- Analyze failure patterns

### 2. Improvements
- Adjust circuit breaker settings if needed
- Improve monitoring coverage
- Enhance error handling
- Update runbooks

### 3. Communication
- Send incident summary to stakeholders
- Update status page
- Conduct post-mortem meeting
