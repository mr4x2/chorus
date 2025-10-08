# DB Test Utility for Worker (throwaway)

This directory contains a small, throwaway utility to validate that the worker can:
- read database config from env/config
- connect to PostgreSQL via GORM
- use repositories to access `storage` and `replicate_job`
- build a runtime configuration from a `job_id` using the ConfigLoader and cache

You can delete this utility after manual verification.

## Prerequisites
- PostgreSQL running locally (or accessible)
- Schema created with foreign keys
- GORM dependencies installed in this repo

Create schema (recommended):
```bash
psql -h localhost -U chorus -d chorus -f ~/projects/chorus/scripts/setup-db.sql
```

## Configure database (env)
You can use a full DSN or individual parts. Env overrides config values.

Full DSN:
```bash
export DB_DSN='host=localhost port=5432 user=chorus password=secret dbname=chorus sslmode=disable TimeZone=UTC'
```

Or parts:
```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=chorus
export DB_PASSWORD=secret
export DB_NAME=chorus
export DB_SSLMODE=disable
# Optional tuning
export DB_MAX_OPEN_CONNS=20
export DB_MAX_IDLE_CONNS=10
export DB_CONN_MAX_LIFETIME=1h
export DB_CONN_MAX_IDLE_TIME=15m
export DB_LOG_LEVEL=warn
```

## Run the test utility
```bash
cd ~/projects/chorus
go run ./tools/dbtest
```
Expected output (example):
```
DB connected
Created job <uuid>
Repo OK: job <uuid> bucket test-bucket from main-storage-test to follower-storage-test
Loader OK: job <uuid> bucket test-bucket -> test-bucket-copy from main-storage-test to follower-storage-test
Status update OK: pending -> running
All checks passed
```

## What it does
1) Opens DB using env-driven config (pkg/db)
2) Inserts two `storage` rows and one `replicate_job` row
3) Loads the job via repository with `Preload("FromStorage").Preload("ToStorage")`
4) Resolves a `RuntimeConfig` from `job_id` via `ConfigLoader`, with caching
5) Updates job `status` and confirms cache invalidation

## Repositories overview
- `pkg/repository/db/models.go`
  - `Storage`: connection and credential details; now scoped by `ProjectID` (UUID)
  - `ReplicateJob`: links to storages via `FromID` and `ToID` (UUID FKs), has `ProjectID`, `Bucket`, `ToBucket`, `Status`
- `pkg/repository/db/repositories.go`
  - `StorageRepository`:
    - `GetByID(ctx, id)`
    - `GetByName(ctx, name)` (compat & testing)
    - `GetByProjectID(ctx, projectID)`
  - `ReplicateJobRepository`:
    - `GetByID(ctx, id)` (preloads `FromStorage`, `ToStorage`)
    - `GetByProjectID(ctx, projectID)` (preloads)
    - `UpdateStatus(ctx, id, status)`
    - `Create(ctx, job)`

## ConfigLoader and cache
- `pkg/repository/db/loader.go`
  - `LoadConfig(ctx, jobID)` → returns `RuntimeConfig` with both storages resolved
  - `UpdateJobStatus(ctx, jobID, status)` → updates DB and invalidates cache
  - `GetJobByProjectID(ctx, projectID)` → bulk fetch
- `pkg/repository/db/cache.go`
  - TTL-based in-memory cache with background cleanup
  - Invalidate per job or by project

## Troubleshooting
- Missing GORM deps: install and tidy in repo root
```bash
cd ~/projects/chorus
go get gorm.io/gorm@v1.25.10 gorm.io/driver/postgres@v1.5.7
go mod tidy
```
- Connection errors: verify `DB_*` env vars and that the DB is reachable
- Schema mismatch: re-run `scripts/setup-db.sql`

## Cleanup
This tool is for manual validation and can be removed after testing:
```bash
git rm -r tools/dbtest
```
