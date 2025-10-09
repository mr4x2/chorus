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
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/clyso/chorus/pkg/metrics"
	ldb "github.com/clyso/chorus/pkg/repository/db"
	"github.com/clyso/chorus/pkg/s3"
	"github.com/clyso/chorus/pkg/s3client"
)

// DBSource implements ConfigSource using database-backed configuration
type DBSource struct {
	loader     *ldb.ConfigLoader
	metricsSvc metrics.S3Service
	tp         trace.TracerProvider
}

// NewDBSource creates a new database-based config source
func NewDBSource(loader *ldb.ConfigLoader, metricsSvc metrics.S3Service, tp trace.TracerProvider) *DBSource {
	return &DBSource{
		loader:     loader,
		metricsSvc: metricsSvc,
		tp:         tp,
	}
}

// LoadConfig loads runtime configuration from the database
func (d *DBSource) LoadConfig(ctx context.Context, jobID uuid.UUID) (*RuntimeConfig, error) {
	if d.loader == nil {
		return nil, fmt.Errorf("config loader is not initialized")
	}

	dbConfig, err := d.loader.LoadConfig(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from database: %w", err)
	}

	// Convert from database format to our unified format
	config := &RuntimeConfig{
		JobID:     dbConfig.JobID,
		ProjectID: dbConfig.ProjectID,
		Bucket:    dbConfig.Bucket,
		ToBucket:  dbConfig.ToBucket,
		Status:    dbConfig.Status,
		// Note: StatusReason is not available in the current database RuntimeConfig
		FromStorage: convertStorageConfig(dbConfig.FromStorage),
		ToStorage:   convertStorageConfig(dbConfig.ToStorage),
	}

	return config, nil
}

// UpdateJobStatus updates the job status in the database
func (d *DBSource) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	if d.loader == nil {
		return fmt.Errorf("config loader is not initialized")
	}
	return d.loader.UpdateJobStatus(ctx, jobID, status)
}

// UpdateJobStatusWithReason updates the job status with reason in the database
func (d *DBSource) UpdateJobStatusWithReason(ctx context.Context, jobID uuid.UUID, status, reason string) error {
	if d.loader == nil {
		return fmt.Errorf("config loader is not initialized")
	}
	return d.loader.UpdateJobStatusWithReason(ctx, jobID, status, reason)
}

// GetClients returns S3 clients built from database configuration
func (d *DBSource) GetClients(ctx context.Context, jobID uuid.UUID, user string) (fromClient s3client.Client, toClient s3client.Client, err error) {
	config, err := d.LoadConfig(ctx, jobID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Build a transient s3client.Service from runtime config
	userAlias := config.ProjectID.String() // scoped user key for credentials map
	storages := map[string]s3.Storage{
		"from_runtime": {
			Address:             config.FromStorage.Address,
			Provider:            config.FromStorage.Provider,
			IsMain:              config.FromStorage.IsMain,
			DefaultRegion:       config.FromStorage.DefaultRegion,
			HealthCheckInterval: time.Duration(config.FromStorage.HealthCheckIntervalMs) * time.Millisecond,
			HealthCheckEnabled:  config.FromStorage.HealthCheckEnabled,
			HttpTimeout:         time.Duration(config.FromStorage.HttpTimeoutMs) * time.Millisecond,
			IsSecure:            config.FromStorage.IsSecure,
			RateLimit: s3.RateLimit{
				Enabled: config.FromStorage.RateLimitEnabled,
				RPM:     config.FromStorage.RateLimitRPM,
			},
			Credentials: map[string]s3.CredentialsV4{
				userAlias: {
					AccessKeyID:     config.FromStorage.AccessKeyID,
					SecretAccessKey: config.FromStorage.SecretAccessKey,
				},
			},
		},
		"to_runtime": {
			Address:             config.ToStorage.Address,
			Provider:            config.ToStorage.Provider,
			IsMain:              config.ToStorage.IsMain,
			DefaultRegion:       config.ToStorage.DefaultRegion,
			HealthCheckInterval: time.Duration(config.ToStorage.HealthCheckIntervalMs) * time.Millisecond,
			HealthCheckEnabled:  config.ToStorage.HealthCheckEnabled,
			HttpTimeout:         time.Duration(config.ToStorage.HttpTimeoutMs) * time.Millisecond,
			IsSecure:            config.ToStorage.IsSecure,
			RateLimit: s3.RateLimit{
				Enabled: config.ToStorage.RateLimitEnabled,
				RPM:     config.ToStorage.RateLimitRPM,
			},
			Credentials: map[string]s3.CredentialsV4{
				userAlias: {
					AccessKeyID:     config.ToStorage.AccessKeyID,
					SecretAccessKey: config.ToStorage.SecretAccessKey,
				},
			},
		},
	}

	svc, err := s3client.New(ctx, &s3.StorageConfig{Storages: storages}, d.metricsSvc, d.tp)
	if err != nil {
		return nil, nil, fmt.Errorf("build runtime s3 clients failed: %w", err)
	}

	fromClient, err = svc.GetByName(ctx, userAlias, "from_runtime")
	if err != nil {
		return nil, nil, fmt.Errorf("get from client failed: %w", err)
	}

	toClient, err = svc.GetByName(ctx, userAlias, "to_runtime")
	if err != nil {
		return nil, nil, fmt.Errorf("get to client failed: %w", err)
	}

	return fromClient, toClient, nil
}

// convertStorageConfig converts from database storage config format to our unified format
func convertStorageConfig(dbStorage *ldb.StorageConfig) *Storage {
	if dbStorage == nil {
		return nil
	}

	return &Storage{
		Name:                  dbStorage.Name,
		Address:               dbStorage.Address,
		Provider:              dbStorage.Provider,
		IsMain:                dbStorage.IsMain,
		IsSecure:              dbStorage.IsSecure,
		DefaultRegion:         dbStorage.DefaultRegion,
		HealthCheckIntervalMs: dbStorage.HealthCheckIntervalMs,
		HealthCheckEnabled:    dbStorage.HealthCheckEnabled,
		HttpTimeoutMs:         dbStorage.HttpTimeoutMs,
		RateLimitEnabled:      dbStorage.RateLimitEnabled,
		RateLimitRPM:          dbStorage.RateLimitRPM,
		AccessKeyID:           dbStorage.AccessKeyID,
		SecretAccessKey:       dbStorage.SecretAccessKey,
	}
}
