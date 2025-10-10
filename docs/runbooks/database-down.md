# Database Down Runbook

## Overview
This runbook provides step-by-step instructions for handling database connectivity issues in the Chorus database-backed replication system.

## Symptoms
- Prometheus alert: `DatabaseDown`
- Worker service logs showing database connection errors
- High error rates in database operations
- Circuit breaker in open state

## Immediate Actions

### 1. Verify Database Status
```bash
# Check if PostgreSQL is running
kubectl get pods -l app=postgres
kubectl describe pod <postgres-pod-name>

# Check database logs
kubectl logs <postgres-pod-name> --tail=100

# Test database connectivity
kubectl exec -it <postgres-pod-name> -- psql -U chorus -d chorus -c "SELECT 1;"
```

### 2. Check Network Connectivity
```bash
# Test network connectivity from worker pod
kubectl exec -it <worker-pod-name> -- nc -zv <db-host> 5432

# Check DNS resolution
kubectl exec -it <worker-pod-name> -- nslookup <db-host>
```

### 3. Check Resource Usage
```bash
# Check database pod resources
kubectl top pod <postgres-pod-name>

# Check node resources
kubectl top nodes
```

## Troubleshooting Steps

### Step 1: Database Service Issues
If the database service is down:

```bash
# Restart database service
kubectl rollout restart deployment/postgres

# Check service status
kubectl get svc postgres
kubectl describe svc postgres
```

### Step 2: Database Pod Issues
If the database pod is not running:

```bash
# Check pod events
kubectl describe pod <postgres-pod-name>

# Check pod logs
kubectl logs <postgres-pod-name> --previous

# Restart pod
kubectl delete pod <postgres-pod-name>
```

### Step 3: Database Configuration Issues
If the database is running but not accessible:

```bash
# Check database configuration
kubectl exec -it <postgres-pod-name> -- cat /var/lib/postgresql/data/postgresql.conf

# Check authentication
kubectl exec -it <postgres-pod-name> -- cat /var/lib/postgresql/data/pg_hba.conf

# Test local connection
kubectl exec -it <postgres-pod-name> -- psql -U postgres -c "SELECT 1;"
```

### Step 4: Network Issues
If network connectivity is the problem:

```bash
# Check network policies
kubectl get networkpolicies

# Check ingress/egress rules
kubectl describe networkpolicy <policy-name>

# Test connectivity with different tools
kubectl exec -it <worker-pod-name> -- telnet <db-host> 5432
```

## Recovery Actions

### 1. Restart Worker Service
If database is back online but worker is still having issues:

```bash
# Restart worker deployment
kubectl rollout restart deployment/worker

# Check worker logs
kubectl logs -f deployment/worker
```

### 2. Clear Circuit Breaker
If circuit breaker is stuck in open state:

```bash
# Restart worker pods to reset circuit breaker
kubectl delete pods -l app=worker

# Monitor circuit breaker state
kubectl logs -f deployment/worker | grep "circuit breaker"
```

### 3. Verify Configuration
Ensure database configuration is correct:

```bash
# Check worker configuration
kubectl get configmap worker-config -o yaml

# Verify database connection string
kubectl exec -it <worker-pod-name> -- env | grep DB_
```

## Prevention

### 1. Database Health Checks
Ensure database has proper health checks:

```yaml
# Add to database deployment
livenessProbe:
  exec:
    command:
    - /bin/sh
    - -c
    - "pg_isready -U chorus -d chorus"
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  exec:
    command:
    - /bin/sh
    - -c
    - "pg_isready -U chorus -d chorus"
  initialDelaySeconds: 5
  periodSeconds: 5
```

### 2. Connection Pooling
Configure appropriate connection pool settings:

```yaml
database:
  maxOpenConns: 100
  maxIdleConns: 50
  connMaxLifetime: "4h"
  connMaxIdleTime: "1h"
```

### 3. Monitoring
Set up proper monitoring:

```yaml
# Prometheus alerts
- alert: DatabaseDown
  expr: up{job="postgres"} == 0
  for: 1m
  labels:
    severity: critical
```

## Escalation

### When to Escalate
- Database has been down for more than 15 minutes
- Multiple database pods are failing
- Data corruption is suspected
- Network issues affect multiple services

### Escalation Contacts
- **Primary**: Database Team (db-team@example.com)
- **Secondary**: Platform Team (platform-team@example.com)
- **Emergency**: On-call Engineer (oncall@example.com)

## Post-Incident

### 1. Root Cause Analysis
- Document the root cause
- Identify contributing factors
- Review monitoring and alerting

### 2. Improvements
- Update runbooks based on lessons learned
- Improve monitoring coverage
- Enhance automation

### 3. Communication
- Send incident summary to stakeholders
- Update status page
- Conduct post-mortem meeting
