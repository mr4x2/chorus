-- PostgreSQL setup script for chorus worker database
-- Creates required tables for storage and replicate_job with UUID FKs

-- Enable UUID extension if not already enabled
CREATE
EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create storage table
CREATE TABLE IF NOT EXISTS storage
(
    id
    UUID
    PRIMARY
    KEY
    DEFAULT
    uuid_generate_v4
(
),
    name VARCHAR
(
    255
) NOT NULL UNIQUE,
    address VARCHAR
(
    1024
) NOT NULL,
    provider VARCHAR
(
    64
) NOT NULL,
    type VARCHAR
(
    64
) NOT NULL DEFAULT 'both',
    is_secure BOOLEAN DEFAULT FALSE,
    default_region VARCHAR
(
    128
),
    health_check_interval_ms BIGINT DEFAULT 0,
    http_timeout_ms BIGINT DEFAULT 0,
    rate_limit_enabled BOOLEAN DEFAULT FALSE,
    rate_limit_rpm INTEGER DEFAULT 0,
    project_id UUID NOT NULL,
    access_key_id VARCHAR
(
    255
) NOT NULL,
    secret_access_key VARCHAR
(
    255
) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW
(
),
    updated_at TIMESTAMPTZ DEFAULT NOW
(
)
    );

-- Create replicate_job table with foreign keys to storage
CREATE TABLE IF NOT EXISTS replicate_job
(
    id
    UUID
    PRIMARY
    KEY
    DEFAULT
    uuid_generate_v4
(
),
    project_id UUID NOT NULL,
    bucket VARCHAR
(
    255
) NOT NULL,
    from_id UUID NOT NULL,
    to_id UUID NOT NULL,
    to_bucket VARCHAR
(
    255
),
    status VARCHAR
(
    64
) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW
(
),
    updated_at TIMESTAMPTZ DEFAULT NOW
(
),
    CONSTRAINT fk_replicate_job_from_storage FOREIGN KEY
(
    from_id
) REFERENCES storage
(
    id
) ON DELETE CASCADE,
    CONSTRAINT fk_replicate_job_to_storage FOREIGN KEY
(
    to_id
) REFERENCES storage
(
    id
)
  ON DELETE CASCADE
    );

-- Indexes
CREATE INDEX IF NOT EXISTS idx_storage_project_id ON storage(project_id);
CREATE INDEX IF NOT EXISTS idx_repl_job_project_id ON replicate_job(project_id);
CREATE INDEX IF NOT EXISTS idx_repl_job_bucket ON replicate_job(bucket);
CREATE INDEX IF NOT EXISTS idx_repl_job_from_id ON replicate_job(from_id);
CREATE INDEX IF NOT EXISTS idx_repl_job_to_id ON replicate_job(to_id);
CREATE INDEX IF NOT EXISTS idx_repl_job_status ON replicate_job(status);

-- Seed sample data for testing (idempotent best-effort)
DO
$$
DECLARE
sample_project_id UUID := uuid_generate_v4();
    main_storage_id
UUID := uuid_generate_v4();
    follower_storage_id
UUID := uuid_generate_v4();
BEGIN
    -- Insert storages if names don't exist
    IF
NOT EXISTS (SELECT 1 FROM storage WHERE name = 'follower-storage') THEN
        INSERT INTO storage (id, name, address, provider, type, is_secure, default_region, health_check_interval_ms, http_timeout_ms,
                             rate_limit_enabled, rate_limit_rpm, project_id, access_key_id, secret_access_key)
        VALUES (main_storage_id, 'follower-storage', 'http://localhost:9000', 'minio', 'destination', true, 'us-east-1', 10, 0,
                false, 0, sample_project_id, 'minioadmin', 'minioadmin');
ELSE
SELECT id, project_id
INTO main_storage_id, sample_project_id
FROM storage
WHERE name = 'main-storage' LIMIT 1;
END IF;

    IF
NOT EXISTS (SELECT 1 FROM storage WHERE name = 'main-storage') THEN
        INSERT INTO storage (id, name, address, provider, type, is_secure, default_region, health_check_interval_ms, http_timeout_ms,
                             rate_limit_enabled, rate_limit_rpm, project_id, access_key_id, secret_access_key)
        VALUES (follower_storage_id, 'main-storage', 'http://localhost:900', 'minio', 'destination', true, 'us-east-1', 10, 0,
                false, 0, sample_project_id, 'minioadmin', 'minioadmin');
ELSE
SELECT id
INTO follower_storage_id
FROM storage
WHERE name = 'follower-storage' LIMIT 1;
END IF;

    -- Insert a sample replicate job if none exists for these storages
    IF
NOT EXISTS (
        SELECT 1 FROM replicate_job WHERE from_id = main_storage_id AND to_id = follower_storage_id AND bucket = 'test-bucket'
    ) THEN
        INSERT INTO replicate_job (id, project_id, bucket, from_id, to_id, to_bucket, status)
        VALUES (uuid_generate_v4(), sample_project_id, 'bucket-repl-10',follower_storage_id, main_storage_id, 'test-migrate', 'pending');
END IF;
END $$;

-- Show summary
\dt
SELECT 'Storage records:' AS info;
SELECT id, name, address, provider, type, project_id
FROM storage
ORDER BY name;

SELECT 'Replicate job records (joined):' AS info;
SELECT rj.id,
       rj.project_id,
       rj.bucket,
       rj.status,
       fs.name AS from_storage,
       ts.name AS to_storage,
       rj.to_bucket
FROM replicate_job rj
         LEFT JOIN storage fs ON fs.id = rj.from_id
         LEFT JOIN storage ts ON ts.id = rj.to_id
ORDER BY rj.created_at DESC;