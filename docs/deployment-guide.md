# Database-Backed Replication Deployment Guide

This guide provides step-by-step instructions for deploying the database-backed replication system in Chorus.

## Prerequisites

### System Requirements
- Kubernetes cluster (v1.20+)
- PostgreSQL 13+ with UUID extension
- Redis 6+ for task queue
- Prometheus and Grafana for monitoring
- kubectl configured for your cluster

### Required Tools
- `kubectl` - Kubernetes command-line tool
- `helm` - Package manager for Kubernetes
- `psql` - PostgreSQL client
- `yq` - YAML processor
- `jq` - JSON processor

## Deployment Steps

### Step 1: Database Setup

#### 1.1 Create Database
```bash
# Connect to PostgreSQL
psql -h <db-host> -U postgres

# Create database and user
CREATE DATABASE chorus;
CREATE USER chorus WITH PASSWORD 'your-secure-password';
GRANT ALL PRIVILEGES ON DATABASE chorus TO chorus;

# Enable UUID extension
\c chorus
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
```

#### 1.2 Run Database Schema
```bash
# Apply database schema
psql -h <db-host> -U chorus -d chorus -f pkg/repository/db/setup-db.sql

# Verify schema
psql -h <db-host> -U chorus -d chorus -c "\dt"
```

#### 1.3 Seed Initial Data
```bash
# Insert sample data
psql -h <db-host> -U chorus -d chorus -f pkg/repository/db/scripts/insert-db.sql

# Verify data
psql -h <db-host> -U chorus -d chorus -c "SELECT COUNT(*) FROM storage;"
psql -h <db-host> -U chorus -d chorus -c "SELECT COUNT(*) FROM replicate_job;"
```

### Step 2: Configuration

#### 2.1 Environment-Specific Configs
Choose the appropriate configuration for your environment:

```bash
# Development
cp docker-compose/worker-conf-dev.yaml worker-conf.yaml

# Staging
cp docker-compose/worker-conf-staging.yaml worker-conf.yaml

# Production
cp docker-compose/worker-conf-prod.yaml worker-conf.yaml
```

#### 2.2 Update Configuration
Edit the configuration file for your environment:

```yaml
database:
  host: "your-db-host"
  port: 5432
  user: "chorus"
  password: "your-secure-password"
  name: "chorus"
  sslmode: "require"  # or "disable" for development
  useForReplication: true
```

#### 2.3 Create ConfigMap
```bash
# Create ConfigMap from configuration
kubectl create configmap worker-config --from-file=config.yaml=worker-conf.yaml

# Verify ConfigMap
kubectl get configmap worker-config -o yaml
```

### Step 3: Deploy Worker Service

#### 3.1 Create Deployment
```yaml
# worker-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  labels:
    app: worker
spec:
  replicas: 3
  selector:
    matchLabels:
      app: worker
  template:
    metadata:
      labels:
        app: worker
    spec:
      containers:
      - name: worker
        image: chorus/worker:latest
        ports:
        - containerPort: 9670
          name: grpc
        - containerPort: 9671
          name: http
        - containerPort: 9090
          name: metrics
        env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: db-secret
              key: password
        volumeMounts:
        - name: config
          mountPath: /etc/worker
        livenessProbe:
          httpGet:
            path: /health
            port: 9671
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 9671
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
      volumes:
      - name: config
        configMap:
          name: worker-config
```

#### 3.2 Create Service
```yaml
# worker-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: worker
  labels:
    app: worker
spec:
  selector:
    app: worker
  ports:
  - name: grpc
    port: 9670
    targetPort: 9670
  - name: http
    port: 9671
    targetPort: 9671
  - name: metrics
    port: 9090
    targetPort: 9090
  type: ClusterIP
```

#### 3.3 Deploy
```bash
# Apply deployment
kubectl apply -f worker-deployment.yaml
kubectl apply -f worker-service.yaml

# Verify deployment
kubectl get pods -l app=worker
kubectl get svc worker
```

### Step 4: Monitoring Setup

#### 4.1 Deploy Prometheus
```bash
# Add Prometheus Helm repository
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install Prometheus
helm install prometheus prometheus-community/kube-prometheus-stack \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false
```

#### 4.2 Configure ServiceMonitor
```yaml
# servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: worker
  labels:
    app: worker
spec:
  selector:
    matchLabels:
      app: worker
  endpoints:
  - port: metrics
    path: /metrics
    interval: 30s
```

#### 4.3 Deploy Grafana Dashboard
```bash
# Create ConfigMap for dashboard
kubectl create configmap grafana-dashboard-chorus \
  --from-file=chorus-dashboard.json=monitoring/grafana-dashboard.json

# Apply dashboard
kubectl apply -f monitoring/grafana-dashboard.json
```

### Step 5: Testing

#### 5.1 Verify Deployment
```bash
# Check pod status
kubectl get pods -l app=worker

# Check logs
kubectl logs -l app=worker --tail=100

# Test health endpoint
kubectl port-forward svc/worker 9671:9671
curl http://localhost:9671/health
```

#### 5.2 Test Database Connectivity
```bash
# Test database connection
kubectl exec -it <worker-pod> -- psql -h <db-host> -U chorus -d chorus -c "SELECT 1;"

# Check circuit breaker state
kubectl logs -l app=worker | grep "circuit breaker"
```

#### 5.3 Test Replication
```bash
# Test CLI with job ID
./chorctl repl add --job-id <job-id> --dry-run

# Test gRPC API
grpcurl -plaintext localhost:9670 chorus.ChorusService/AddBucketReplication \
  -d '{"job_id": "<job-id>", "dry_run": true}'
```

## Environment-Specific Configurations

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
    failureThreshold: 5
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
    failureThreshold: 10
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
    failureThreshold: 5
    operationTimeout: "10s"
```

## Security Considerations

### 1. Database Security
- Use strong passwords
- Enable SSL/TLS
- Restrict network access
- Regular security updates
- Backup encryption

### 2. Kubernetes Security
- Use RBAC
- Network policies
- Pod security policies
- Secret management
- Image scanning

### 3. Application Security
- Input validation
- Error handling
- Logging and monitoring
- Rate limiting
- Authentication/authorization

## Troubleshooting

### Common Issues

#### 1. Database Connection Failed
```bash
# Check database status
kubectl exec -it <worker-pod> -- nc -zv <db-host> 5432

# Check database logs
kubectl logs <postgres-pod> --tail=100

# Verify credentials
kubectl exec -it <worker-pod> -- env | grep DB_
```

#### 2. Circuit Breaker Open
```bash
# Check circuit breaker state
kubectl logs -l app=worker | grep "circuit breaker"

# Restart worker pods
kubectl rollout restart deployment/worker

# Monitor recovery
kubectl logs -f deployment/worker
```

#### 3. High Memory Usage
```bash
# Check resource usage
kubectl top pods -l app=worker

# Check memory limits
kubectl describe pod <worker-pod>

# Adjust resource limits
kubectl patch deployment worker -p '{"spec":{"template":{"spec":{"containers":[{"name":"worker","resources":{"limits":{"memory":"2Gi"}}}]}}}}'
```

## Maintenance

### Regular Tasks
- Monitor database performance
- Check circuit breaker state
- Review error logs
- Update dependencies
- Backup database

### Updates
- Test in staging first
- Use rolling updates
- Monitor during deployment
- Have rollback plan ready

## Support

### Documentation
- [Rollout Strategy](rollout-strategy.md)
- [Runbooks](runbooks/)
- [API Documentation](api.md)
- [Configuration Reference](config-reference.md)

### Contacts
- **Development Team**: dev-team@example.com
- **Operations Team**: ops-team@example.com
- **On-call**: oncall@example.com

### Resources
- [GitHub Repository](https://github.com/example/chorus)
- [Issue Tracker](https://github.com/example/chorus/issues)
- [Wiki](https://github.com/example/chorus/wiki)
