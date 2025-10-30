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
	"go.opentelemetry.io/otel/trace"

	"github.com/clyso/chorus/pkg/metrics"
	"github.com/clyso/chorus/pkg/s3client"
)

// YAMLSource implements ConfigSource using YAML-based configuration
// This is the legacy implementation that uses storage names instead of job IDs
type YAMLSource struct {
	clients    s3client.Service
	metricsSvc metrics.S3Service
	tp         trace.TracerProvider
}

// NewYAMLSource creates a new YAML-based config source
func NewYAMLSource(clients s3client.Service, metricsSvc metrics.S3Service, tp trace.TracerProvider) *YAMLSource {
	return &YAMLSource{
		clients:    clients,
		metricsSvc: metricsSvc,
		tp:         tp,
	}
}

// LoadConfig is not supported for YAML source as it doesn't have job IDs
func (y *YAMLSource) LoadConfig(ctx context.Context, jobID uuid.UUID) (*RuntimeConfig, error) {
	return nil, fmt.Errorf("YAML source does not support job ID-based configuration loading")
}

// UpdateJobStatus is not supported for YAML source
func (y *YAMLSource) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status string) error {
	return fmt.Errorf("YAML source does not support job status updates")
}

// UpdateJobStatusWithReason is not supported for YAML source
func (y *YAMLSource) UpdateJobStatusWithReason(ctx context.Context, jobID uuid.UUID, status, reason string) error {
	return fmt.Errorf("YAML source does not support job status updates")
}

// DeleteJob is not supported for YAMLSource
func (y *YAMLSource) DeleteJob(ctx context.Context, jobID uuid.UUID) error {
	return fmt.Errorf("YAML source does not support job deletion")
}

// GetClients returns S3 clients using storage names (legacy approach)
func (y *YAMLSource) GetClients(ctx context.Context, jobID uuid.UUID, user string) (fromClient s3client.Client, toClient s3client.Client, err error) {
	return nil, nil, fmt.Errorf("YAML source requires storage names, not job ID")
}

// GetClientsByName returns S3 clients using storage names (the actual YAML method)
func (y *YAMLSource) GetClientsByName(ctx context.Context, user, fromStorage, toStorage string) (fromClient s3client.Client, toClient s3client.Client, err error) {
	fromClient, err = y.clients.GetByName(ctx, user, fromStorage)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get %q s3 client: %w", fromStorage, err)
	}

	toClient, err = y.clients.GetByName(ctx, user, toStorage)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get %q s3 client: %w", toStorage, err)
	}

	return fromClient, toClient, nil
}
