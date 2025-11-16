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
	"github.com/google/uuid"

	"github.com/clyso/chorus/pkg/dom"
)

type Storage struct {
	ID                    uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	Name                  string          `gorm:"size:255;not null" json:"name"`
	Address               string          `gorm:"size:1024;not null" json:"address"`
	Provider              string          `gorm:"size:64;not null" json:"provider"`
	Type                  dom.StorageType `gorm:"size:64;not null;default:'both'" json:"type"`
	IsSecure              bool            `json:"is_secure"`
	DefaultRegion         string          `gorm:"size:128" json:"default_region"`
	HealthCheckIntervalMs int64           `json:"health_check_interval_ms"`
	HealthCheckEnabled    bool            `json:"health_check_enabled"`
	HttpTimeoutMs         int64           `json:"http_timeout_ms"`
	RateLimitEnabled      bool            `json:"rate_limit_enabled"`
	RateLimitRPM          int             `json:"rate_limit_rpm"`
	ProjectID             uuid.UUID       `gorm:"type:uuid;index;not null" json:"project_id"`
	AccessKeyID           string          `gorm:"size:255;not null" json:"access_key_id"`
	SecretAccessKey       string          `gorm:"size:255;not null" json:"secret_access_key"`
}

func (Storage) TableName() string { return "storage" }

// ReplicateJob represents a replication job persisted in DB.
// Table name is "replicate_job".
type ReplicateJob struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ProjectID    uuid.UUID `gorm:"type:uuid;index;not null" json:"project_id"`
	Bucket       string    `gorm:"size:255;index;not null" json:"bucket"`
	FromID       uuid.UUID `gorm:"type:uuid;not null;constraint:OnDelete:CASCADE" json:"from_id"`
	ToID         uuid.UUID `gorm:"type:uuid;not null;constraint:OnDelete:CASCADE" json:"to_id"`
	ToBucket     string    `gorm:"size:255" json:"to_bucket"`
	Status       string    `gorm:"size:64;default:'pending'" json:"status"`
	StatusReason string    `gorm:"size:255" json:"status_reason"`

	// Relations
	FromStorage *Storage `gorm:"foreignKey:FromID;references:ID" json:"from_storage,omitempty"`
	ToStorage   *Storage `gorm:"foreignKey:ToID;references:ID" json:"to_storage,omitempty"`
}

func (ReplicateJob) TableName() string { return "replicate_job" }
