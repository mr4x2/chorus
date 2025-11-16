-- Migration script to convert YAML configurations to database records
-- This script should be run after exporting YAML configurations

-- Create temporary tables for migration data
CREATE TEMP TABLE IF NOT EXISTS temp_storages (
    name VARCHAR(255),
    address VARCHAR(1024),
    provider VARCHAR(64),
    type VARCHAR(64),
    is_secure BOOLEAN,
    default_region VARCHAR(128),
    health_check_interval_ms BIGINT,
    health_check_enabled BOOLEAN,
    http_timeout_ms BIGINT,
    rate_limit_enabled BOOLEAN,
    rate_limit_rpm INTEGER,
    access_key_id VARCHAR(255),
    secret_access_key VARCHAR(255),
    project_id UUID
);

CREATE TEMP TABLE IF NOT EXISTS temp_jobs (
    bucket VARCHAR(255),
    to_bucket VARCHAR(255),
    from_storage_name VARCHAR(255),
    to_storage_name VARCHAR(255),
    project_id UUID
);

-- Function to insert storage from YAML data
CREATE OR REPLACE FUNCTION insert_storage_from_yaml(
    p_name VARCHAR(255),
    p_address VARCHAR(1024),
    p_provider VARCHAR(64),
    p_type VARCHAR(64),
    p_is_secure BOOLEAN,
    p_default_region VARCHAR(128),
    p_health_check_interval_ms BIGINT DEFAULT 10000,
    p_health_check_enabled BOOLEAN DEFAULT false,
    p_http_timeout_ms BIGINT DEFAULT 60000,
    p_rate_limit_enabled BOOLEAN DEFAULT false,
    p_rate_limit_rpm INTEGER DEFAULT 60,
    p_access_key_id VARCHAR(255),
    p_secret_access_key VARCHAR(255),
    p_project_id UUID
) RETURNS UUID AS $$
DECLARE
    storage_id UUID;
BEGIN
    -- Generate new UUID for storage
    storage_id := gen_random_uuid();
    
    -- Insert storage
    INSERT INTO storage (
        id, name, address, provider, type, is_secure, default_region,
        health_check_interval_ms, health_check_enabled, http_timeout_ms,
        rate_limit_enabled, rate_limit_rpm, project_id, access_key_id, secret_access_key
    ) VALUES (
        storage_id, p_name, p_address, p_provider, COALESCE(NULLIF(TRIM(p_type), ''), 'both'), p_is_secure, p_default_region,
        p_health_check_interval_ms, p_health_check_enabled, p_http_timeout_ms,
        p_rate_limit_enabled, p_rate_limit_rpm, p_project_id, p_access_key_id, p_secret_access_key
    );
    
    RETURN storage_id;
END;
$$ LANGUAGE plpgsql;

-- Function to insert replication job from YAML data
CREATE OR REPLACE FUNCTION insert_job_from_yaml(
    p_bucket VARCHAR(255),
    p_to_bucket VARCHAR(255),
    p_from_storage_name VARCHAR(255),
    p_to_storage_name VARCHAR(255),
    p_project_id UUID
) RETURNS UUID AS $$
DECLARE
    job_id UUID;
    from_storage_id UUID;
    to_storage_id UUID;
BEGIN
    -- Get storage IDs by name
    SELECT id INTO from_storage_id FROM storage WHERE name = p_from_storage_name AND project_id = p_project_id;
    SELECT id INTO to_storage_id FROM storage WHERE name = p_to_storage_name AND project_id = p_project_id;
    
    -- Check if storages exist
    IF from_storage_id IS NULL THEN
        RAISE EXCEPTION 'From storage % not found for project %', p_from_storage_name, p_project_id;
    END IF;
    
    IF to_storage_id IS NULL THEN
        RAISE EXCEPTION 'To storage % not found for project %', p_to_storage_name, p_project_id;
    END IF;
    
    -- Generate new UUID for job
    job_id := gen_random_uuid();
    
    -- Insert job
    INSERT INTO replicate_job (
        id, project_id, bucket, to_bucket, from_id, to_id, status
    ) VALUES (
        job_id, p_project_id, p_bucket, p_to_bucket, from_storage_id, to_storage_id, 'pending'
    );
    
    RETURN job_id;
END;
$$ LANGUAGE plpgsql;

-- Example usage:
-- 
-- 1. Insert storages:
-- SELECT insert_storage_from_yaml(
--     'main-storage',
--     'http://localhost:9000',
--     'minio',
--     'source',
--     false,
--     'us-east-1',
--     10000,
--     false,
--     60000,
--     false,
--     60,
--     'minioadmin',
--     'minioadmin',
--     'your-project-id-here'
-- );
--
-- 2. Insert jobs:
-- SELECT insert_job_from_yaml(
--     'test-bucket',
--     'test-bucket-copy',
--     'main-storage',
--     'follower-storage',
--     'your-project-id-here'
-- );

-- Validation queries
CREATE OR REPLACE FUNCTION validate_migration() RETURNS TABLE (
    table_name TEXT,
    record_count BIGINT,
    status TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        'storage'::TEXT,
        COUNT(*)::BIGINT,
        CASE WHEN COUNT(*) > 0 THEN 'OK' ELSE 'EMPTY' END::TEXT
    FROM storage
    UNION ALL
    SELECT 
        'replicate_job'::TEXT,
        COUNT(*)::BIGINT,
        CASE WHEN COUNT(*) > 0 THEN 'OK' ELSE 'EMPTY' END::TEXT
    FROM replicate_job;
END;
$$ LANGUAGE plpgsql;

-- Rollback function
CREATE OR REPLACE FUNCTION rollback_migration() RETURNS VOID AS $$
BEGIN
    -- Delete all replicate_job records
    DELETE FROM replicate_job;
    
    -- Delete all storage records
    DELETE FROM storage;
    
    RAISE NOTICE 'Migration rolled back successfully';
END;
$$ LANGUAGE plpgsql;

-- Display migration status
SELECT 'Migration functions created successfully' AS status;
