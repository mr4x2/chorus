// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/clyso/chorus/pkg/dom"
)

// ConfigLoader builds runtime configuration from a replicate job ID
type ConfigLoader struct {
	db          *gorm.DB
	storageRepo *StorageRepository
	jobRepo     *ReplicateJobRepository
	cache       *ConfigCache
}

// RuntimeConfig represents the resolved configuration for job execution
type RuntimeConfig struct {
	JobID     uuid.UUID `json:"job_id"`
	ProjectID uuid.UUID `json:"project_id"`
	Bucket    string    `json:"bucket"`
	ToBucket  string    `json:"to_bucket"`
	Status    string    `json:"status"`

	// Source storage configuration
	FromStorage *StorageConfig `json:"from_storage"`

	// Destination storage configuration
	ToStorage *StorageConfig `json:"to_storage"`
}

// StorageConfig represents storage connection details for runtime
type StorageConfig struct {
	ID                    uuid.UUID       `json:"id"`
	Name                  string          `json:"name"`
	Address               string          `json:"address"`
	Provider              string          `json:"provider"`
	Type                  dom.StorageType `json:"type"`
	IsSecure              bool            `json:"is_secure"`
	DefaultRegion         string          `json:"default_region"`
	HealthCheckIntervalMs int64           `json:"health_check_interval_ms"`
	HealthCheckEnabled    bool            `json:"health_check_enabled"`
	HttpTimeoutMs         int64           `json:"http_timeout_ms"`
	RateLimitEnabled      bool            `json:"rate_limit_enabled"`
	RateLimitRPM          int             `json:"rate_limit_rpm"`
	AccessKeyID           string          `json:"access_key_id"`
	SecretAccessKey       string          `json:"secret_access_key"`
}

// NewConfigLoader creates a new config loader with caching
func NewConfigLoader(db *gorm.DB, cacheTTL time.Duration, config ResilienceConfig, logger zerolog.Logger) *ConfigLoader {
	storageRepo := NewStorageRepository(db, config, logger)
	jobRepo := NewReplicateJobRepository(db, config, logger)

	return &ConfigLoader{
		db:          db,
		storageRepo: storageRepo,
		jobRepo:     jobRepo,
		cache:       NewConfigCache(cacheTTL),
	}
}

// LoadConfig loads runtime configuration for a job ID
func (l *ConfigLoader) LoadConfig(ctx context.Context, jobID uuid.UUID) (*RuntimeConfig, error) {
	// Check cache first
	if cached := l.cache.Get(jobID); cached != nil {
		return cached, nil
	}

	// Load from database
	job, err := l.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to load job %s: %w", jobID, err)
	}
	if job == nil {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	// Validate job status
	if job.Status != "pending" && job.Status != "running" {
		return nil, fmt.Errorf("job %s is not in executable state (status: %s)", jobID, job.Status)
	}

	// Build runtime config
	config := &RuntimeConfig{
		JobID:     job.ID,
		ProjectID: job.ProjectID,
		Bucket:    job.Bucket,
		ToBucket:  job.ToBucket,
		Status:    job.Status,
	}

	// Set default ToBucket if empty
	if config.ToBucket == "" {
		config.ToBucket = config.Bucket
	}

	// Load storage configurations
	if job.FromStorage == nil || job.ToStorage == nil {
		return nil, fmt.Errorf("job %s missing storage references", jobID)
	}

	config.FromStorage = l.storageToConfig(job.FromStorage)
	config.ToStorage = l.storageToConfig(job.ToStorage)

	// Cache the result
	l.cache.Set(jobID, config)

	return config, nil
}

// UpdateJobStatus updates job status and invalidates cache
func (l *ConfigLoader) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	err := l.jobRepo.UpdateStatus(ctx, jobID, status)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Invalidate cache
	l.cache.Invalidate(jobID)

	return nil
}

// UpdateJobStatusWithReason updates job status and reason and invalidates cache
func (l *ConfigLoader) UpdateJobStatusWithReason(ctx context.Context, jobID uuid.UUID, status, reason string) error {
	err := l.jobRepo.UpdateStatusWithReason(ctx, jobID, status, reason)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	l.cache.Invalidate(jobID)
	return nil
}

// DeleteJob deletes the replicate job by ID from the database and invalidates cache
func (l *ConfigLoader) DeleteJob(ctx context.Context, jobID uuid.UUID) error {
	err := l.jobRepo.DeleteByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to delete job %s: %w", jobID, err)
	}
	// Invalidate any cache for this jobID
	l.cache.Invalidate(jobID)
	return nil
}

// storageToConfig converts Storage model to StorageConfig
func (l *ConfigLoader) storageToConfig(storage *Storage) *StorageConfig {
	return &StorageConfig{
		ID:                    storage.ID,
		Name:                  storage.Name,
		Address:               storage.Address,
		Provider:              storage.Provider,
		Type:                  storage.Type,
		IsSecure:              storage.IsSecure,
		DefaultRegion:         storage.DefaultRegion,
		HealthCheckIntervalMs: storage.HealthCheckIntervalMs,
		HealthCheckEnabled:    storage.HealthCheckEnabled,
		HttpTimeoutMs:         storage.HttpTimeoutMs,
		RateLimitEnabled:      storage.RateLimitEnabled,
		RateLimitRPM:          storage.RateLimitRPM,
		AccessKeyID:           storage.AccessKeyID,
		SecretAccessKey:       storage.SecretAccessKey,
	}
}

// GetJobByProjectID loads all jobs for a project
func (l *ConfigLoader) GetJobByProjectID(ctx context.Context, projectID uuid.UUID) ([]*RuntimeConfig, error) {
	jobs, err := l.jobRepo.GetByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load jobs for project %s: %w", projectID, err)
	}

	configs := make([]*RuntimeConfig, 0, len(jobs))
	for _, job := range jobs {
		config := &RuntimeConfig{
			JobID:     job.ID,
			ProjectID: job.ProjectID,
			Bucket:    job.Bucket,
			ToBucket:  job.ToBucket,
			Status:    job.Status,
		}

		if config.ToBucket == "" {
			config.ToBucket = config.Bucket
		}

		if job.FromStorage != nil {
			config.FromStorage = l.storageToConfig(job.FromStorage)
		}
		if job.ToStorage != nil {
			config.ToStorage = l.storageToConfig(job.ToStorage)
		}

		configs = append(configs, config)
	}

	return configs, nil
}
