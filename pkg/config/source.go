/*
 * Copyright © 2024 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"context"

	"github.com/google/uuid"

	"github.com/clyso/chorus/pkg/s3client"
)

// RuntimeConfig represents the runtime configuration for a replication job
type RuntimeConfig struct {
	JobID        uuid.UUID `json:"job_id"`
	ProjectID    uuid.UUID `json:"project_id"`
	Bucket       string    `json:"bucket"`
	ToBucket     string    `json:"to_bucket"`
	Status       string    `json:"status"`
	StatusReason string    `json:"status_reason,omitempty"`
	FromStorage  *Storage  `json:"from_storage"`
	ToStorage    *Storage  `json:"to_storage"`
}

// Storage represents storage configuration
type Storage struct {
	Name                  string `json:"name"`
	Address               string `json:"address"`
	Provider              string `json:"provider"`
	Type                  string `json:"type"`
	IsSecure              bool   `json:"is_secure"`
	DefaultRegion         string `json:"default_region"`
	HealthCheckIntervalMs int64  `json:"health_check_interval_ms"`
	HealthCheckEnabled    bool   `json:"health_check_enabled"`
	HttpTimeoutMs         int64  `json:"http_timeout_ms"`
	RateLimitEnabled      bool   `json:"rate_limit_enabled"`
	RateLimitRPM          int    `json:"rate_limit_rpm"`
	AccessKeyID           string `json:"access_key_id"`
	SecretAccessKey       string `json:"secret_access_key"`
}

// ConfigSource defines the interface for loading replication job configurations
type ConfigSource interface {
	// LoadConfig loads runtime configuration for a replication job by ID
	LoadConfig(ctx context.Context, jobID uuid.UUID) (*RuntimeConfig, error)

	// UpdateJobStatus updates the status of a replication job
	UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error

	// UpdateJobStatusWithReason updates the status of a replication job with a reason
	UpdateJobStatusWithReason(ctx context.Context, jobID uuid.UUID, status, reason string) error

	// GetClients returns S3 clients for a replication job
	GetClients(ctx context.Context, jobID uuid.UUID, user string) (fromClient s3client.Client, toClient s3client.Client, err error)

	// DeleteJob deletes a replication job by ID (no-op if not supported)
	DeleteJob(ctx context.Context, jobID uuid.UUID) error
}

// JobStatusUpdater defines the interface for updating job status
type JobStatusUpdater interface {
	UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error
	UpdateJobStatusWithReason(ctx context.Context, jobID uuid.UUID, status, reason string) error

	DeleteJob(ctx context.Context, jobID uuid.UUID) error
}
