-- PostgreSQL setup script for chorus worker database
-- Run this script to create the required tables for storage and replicate_job

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create storage table
CREATE TABLE IF NOT EXISTS storage (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    address VARCHAR(1024) NOT NULL,
    provider VARCHAR(64) NOT NULL,
    is_main BOOLEAN DEFAULT FALSE,
    is_secure BOOLEAN DEFAULT FALSE,
    default_region VARCHAR(128),
    health_check_interval_ms BIGINT DEFAULT 0,
    http_timeout_ms BIGINT DEFAULT 0,
    rate_limit_enabled BOOLEAN DEFAULT FALSE,
    rate_limit_rpm INTEGER DEFAULT 0,
    "user" VARCHAR(255) NOT NULL,
    access_key_id VARCHAR(255) NOT NULL,
    secret_access_key VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create replicate_job table
CREATE TABLE IF NOT EXISTS replicate_job (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    "user" VARCHAR(255) NOT NULL,
    bucket VARCHAR(255) NOT NULL,
    "from" VARCHAR(255) NOT NULL,
    "to" VARCHAR(255) NOT NULL,
    to_bucket VARCHAR(255),
    status VARCHAR(64) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_replicate_job_user ON replicate_job("user");
CREATE INDEX IF NOT EXISTS idx_replicate_job_bucket ON replicate_job(bucket);
CREATE INDEX IF NOT EXISTS idx_replicate_job_from ON replicate_job("from");
CREATE INDEX IF NOT EXISTS idx_replicate_job_to ON replicate_job("to");
CREATE INDEX IF NOT EXISTS idx_replicate_job_status ON replicate_job(status);

-- Insert sample data for testing
INSERT INTO storage (id, name, address, provider, is_main, is_secure, "user", access_key_id, secret_access_key) VALUES
    (uuid_generate_v4(), 'main-storage', 'http://localhost:9000', 'minio', true, false, 'testuser', 'minioadmin', 'minioadmin'),
    (uuid_generate_v4(), 'follower-storage', 'http://localhost:9001', 'minio', false, false, 'testuser', 'minioadmin', 'minioadmin')
ON CONFLICT (name) DO NOTHING;

-- Insert sample replicate job
INSERT INTO replicate_job (id, "user", bucket, "from", "to", to_bucket, status) VALUES
    (uuid_generate_v4(), 'testuser', 'test-bucket', 'main-storage', 'follower-storage', 'test-bucket-copy', 'pending')
ON CONFLICT DO NOTHING;

-- Show created tables
\dt

-- Show sample data
SELECT 'Storage records:' as info;
SELECT id, name, address, provider, is_main FROM storage;

SELECT 'Replicate job records:' as info;
SELECT id, "user", bucket, "from", "to", to_bucket, status FROM replicate_job;