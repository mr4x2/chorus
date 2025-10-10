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
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type StorageRepository struct {
	db *ResilientDB
}

func NewStorageRepository(db *gorm.DB, config ResilienceConfig, logger zerolog.Logger) *StorageRepository {
	return &StorageRepository{db: NewResilientDB(db, config, logger)}
}

func (r *StorageRepository) GetByID(ctx context.Context, id uuid.UUID) (*Storage, error) {
	var s Storage
	var result *Storage

	err := r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		if err := r.db.GetDB().WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				result = nil
				return nil
			}
			return err
		}
		result = &s
		return nil
	})

	return result, err
}

func (r *StorageRepository) GetByName(ctx context.Context, name string) (*Storage, error) {
	var s Storage
	var result *Storage

	err := r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		if err := r.db.GetDB().WithContext(ctx).First(&s, "name = ?", name).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				result = nil
				return nil
			}
			return err
		}
		result = &s
		return nil
	})

	return result, err
}

func (r *StorageRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*Storage, error) {
	var storages []*Storage
	var result []*Storage

	err := r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		if err := r.db.GetDB().WithContext(ctx).Where("project_id = ?", projectID).Find(&storages).Error; err != nil {
			return err
		}
		result = storages
		return nil
	})

	return result, err
}

type ReplicateJobRepository struct {
	db *ResilientDB
}

func NewReplicateJobRepository(db *gorm.DB, config ResilienceConfig, logger zerolog.Logger) *ReplicateJobRepository {
	return &ReplicateJobRepository{db: NewResilientDB(db, config, logger)}
}

func (r *ReplicateJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*ReplicateJob, error) {
	var j ReplicateJob
	var result *ReplicateJob

	err := r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		if err := r.db.GetDB().WithContext(ctx).Preload("FromStorage").Preload("ToStorage").First(&j, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				result = nil
				return nil
			}
			return err
		}
		result = &j
		return nil
	})

	return result, err
}

func (r *ReplicateJobRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*ReplicateJob, error) {
	var jobs []*ReplicateJob
	var result []*ReplicateJob

	err := r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		if err := r.db.GetDB().WithContext(ctx).Preload("FromStorage").Preload("ToStorage").Where("project_id = ?", projectID).Find(&jobs).Error; err != nil {
			return err
		}
		result = jobs
		return nil
	})

	return result, err
}

func (r *ReplicateJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		return r.db.GetDB().WithContext(ctx).Model(&ReplicateJob{}).Where("id = ?", id).Update("status", status).Error
	})
}

func (r *ReplicateJobRepository) UpdateStatusWithReason(ctx context.Context, id uuid.UUID, status, reason string) error {
	return r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		return r.db.GetDB().WithContext(ctx).Model(&ReplicateJob{}).Where("id = ?", id).Updates(map[string]any{
			"status":        status,
			"status_reason": reason,
		}).Error
	})
}

func (r *ReplicateJobRepository) Create(ctx context.Context, job *ReplicateJob) error {
	return r.db.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		return r.db.GetDB().WithContext(ctx).Create(job).Error
	})
}
