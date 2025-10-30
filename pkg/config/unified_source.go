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

	"github.com/google/uuid"

	"github.com/clyso/chorus/pkg/s3client"
)

// UnifiedSource implements ConfigSource by delegating to either YAML or DB source
// based on whether a job ID is provided and the useDB flag
type UnifiedSource struct {
	yamlSource *YAMLSource
	dbSource   *DBSource
	useDB      bool
}

// NewUnifiedSource creates a new unified config source
func NewUnifiedSource(yamlSource *YAMLSource, dbSource *DBSource, useDB bool) *UnifiedSource {
	return &UnifiedSource{
		yamlSource: yamlSource,
		dbSource:   dbSource,
		useDB:      useDB,
	}
}

// LoadConfig loads configuration using the appropriate source
func (u *UnifiedSource) LoadConfig(ctx context.Context, jobID uuid.UUID) (*RuntimeConfig, error) {
	if u.useDB && u.dbSource != nil {
		return u.dbSource.LoadConfig(ctx, jobID)
	}
	return u.yamlSource.LoadConfig(ctx, jobID)
}

// UpdateJobStatus updates job status using the appropriate source
func (u *UnifiedSource) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	if u.useDB && u.dbSource != nil {
		return u.dbSource.UpdateJobStatus(ctx, jobID, status)
	}
	return u.yamlSource.UpdateJobStatus(ctx, jobID, status)
}

// UpdateJobStatusWithReason updates job status with reason using the appropriate source
func (u *UnifiedSource) UpdateJobStatusWithReason(ctx context.Context, jobID uuid.UUID, status, reason string) error {
	if u.useDB && u.dbSource != nil {
		return u.dbSource.UpdateJobStatusWithReason(ctx, jobID, status, reason)
	}
	return u.yamlSource.UpdateJobStatusWithReason(ctx, jobID, status, reason)
}

// GetClients returns S3 clients using the appropriate source
func (u *UnifiedSource) GetClients(ctx context.Context, jobID uuid.UUID, user string) (fromClient s3client.Client, toClient s3client.Client, err error) {
	if u.useDB && u.dbSource != nil {
		return u.dbSource.GetClients(ctx, jobID, user)
	}
	return u.yamlSource.GetClients(ctx, jobID, user)
}

// GetClientsByName returns S3 clients using storage names (YAML source only)
// This method is provided for backward compatibility with existing code
func (u *UnifiedSource) GetClientsByName(ctx context.Context, user, fromStorage, toStorage string) (fromClient s3client.Client, toClient s3client.Client, err error) {
	if u.yamlSource == nil {
		return nil, nil, fmt.Errorf("YAML source not available")
	}
	return u.yamlSource.GetClientsByName(ctx, user, fromStorage, toStorage)
}

// GetClientsByJobIDOrName returns S3 clients using either job ID or storage names
// This is the main method that should be used by handlers
func (u *UnifiedSource) GetClientsByJobIDOrName(ctx context.Context, jobID *uuid.UUID, user, fromStorage, toStorage string) (fromClient s3client.Client, toClient s3client.Client, err error) {
	// If job ID is provided and DB is enabled, use DB source
	if jobID != nil && u.useDB && u.dbSource != nil {
		return u.dbSource.GetClients(ctx, *jobID, user)
	}

	// Otherwise, use YAML source with storage names
	if u.yamlSource == nil {
		return nil, nil, fmt.Errorf("YAML source not available")
	}
	return u.yamlSource.GetClientsByName(ctx, user, fromStorage, toStorage)
}

// DeleteJob deletes a job by ID using the appropriate config source
func (u *UnifiedSource) DeleteJob(ctx context.Context, jobID uuid.UUID) error {
	if u.useDB && u.dbSource != nil {
		return u.dbSource.DeleteJob(ctx, jobID)
	}
	if u.yamlSource != nil {
		return u.yamlSource.DeleteJob(ctx, jobID)
	}
	return fmt.Errorf("no config source available for DeleteJob")
}
