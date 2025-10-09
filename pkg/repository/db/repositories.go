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
	"gorm.io/gorm"
)

type StorageRepository struct{ db *gorm.DB }

func NewStorageRepository(db *gorm.DB) *StorageRepository { return &StorageRepository{db: db} }

func (r *StorageRepository) GetByID(ctx context.Context, id uuid.UUID) (*Storage, error) {
	var s Storage
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *StorageRepository) GetByName(ctx context.Context, name string) (*Storage, error) {
	var s Storage
	if err := r.db.WithContext(ctx).First(&s, "name = ?", name).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *StorageRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*Storage, error) {
	var storages []*Storage
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&storages).Error; err != nil {
		return nil, err
	}
	return storages, nil
}

type ReplicateJobRepository struct{ db *gorm.DB }

func NewReplicateJobRepository(db *gorm.DB) *ReplicateJobRepository {
	return &ReplicateJobRepository{db: db}
}

func (r *ReplicateJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*ReplicateJob, error) {
	var j ReplicateJob
	if err := r.db.WithContext(ctx).Preload("FromStorage").Preload("ToStorage").First(&j, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &j, nil
}

func (r *ReplicateJobRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*ReplicateJob, error) {
	var jobs []*ReplicateJob
	if err := r.db.WithContext(ctx).Preload("FromStorage").Preload("ToStorage").Where("project_id = ?", projectID).Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *ReplicateJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.db.WithContext(ctx).Model(&ReplicateJob{}).Where("id = ?", id).Update("status", status).Error
}

func (r *ReplicateJobRepository) UpdateStatusWithReason(ctx context.Context, id uuid.UUID, status, reason string) error {
	return r.db.WithContext(ctx).Model(&ReplicateJob{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"status_reason": reason,
	}).Error
}

func (r *ReplicateJobRepository) Create(ctx context.Context, job *ReplicateJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}
