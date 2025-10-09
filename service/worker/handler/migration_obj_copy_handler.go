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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/clyso/chorus/pkg/dom"
	"github.com/clyso/chorus/pkg/entity"
	"github.com/clyso/chorus/pkg/log"
	"github.com/clyso/chorus/pkg/meta"
	"github.com/clyso/chorus/pkg/rclone"
	"github.com/clyso/chorus/pkg/s3client"
	"github.com/clyso/chorus/pkg/tasks"
)

func (s *svc) HandleMigrationObjCopy(ctx context.Context, t *asynq.Task) (err error) {
	var p tasks.MigrateObjCopyPayload
	if err = json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("MigrateObjCopyPayload Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}
	ctx = log.WithBucket(ctx, p.Bucket)
	ctx = log.WithObjName(ctx, p.Obj.Name)
	logger := zerolog.Ctx(ctx)
	fromBucket, toBucket := p.ID.FromToBuckets(p.Bucket)

	if err = s.limit.StorReq(ctx, p.ID.FromStorage()); err != nil {
		logger.Debug().Err(err).Str(log.Storage, p.ID.FromStorage()).Msg("rate limit error")
		return err
	}
	if err = s.limit.StorReq(ctx, p.ID.ToStorage()); err != nil {
		logger.Debug().Err(err).Str(log.Storage, p.ID.ToStorage()).Msg("rate limit error")
		return err
	}

	domObj := dom.Object{
		Bucket:  p.Bucket,
		Name:    p.Obj.Name,
		Version: p.Obj.VersionID,
	}

	objectLockID := entity.NewVersionedObjectLockID(p.ID.ToStorage(), toBucket, p.Obj.Name, p.Obj.VersionID)
	lock, err := s.objectLocker.Lock(ctx, objectLockID)
	if err != nil {
		return err
	}
	defer lock.Release(ctx)
	objMeta, err := s.versionSvc.GetObj(ctx, domObj)
	if err != nil {
		return fmt.Errorf("migration obj copy: unable to get obj meta: %w", err)
	}
	destVersionKey := meta.ToDest(p.ID.ToStorage(), toBucket)
	fromVer, toVer := objMeta[meta.ToDest(p.ID.FromStorage(), "")], objMeta[destVersionKey]

	if fromVer != 0 && fromVer <= toVer {
		logger.Info().Int64("from_ver", fromVer).Int64("to_ver", toVer).Msg("migration obj copy: identical from/to obj version: skip copy")
		return nil
	}

	// 1. sync obj meta and content
    // Ensure rclone has runtime creds for project_id alias
    // We use clients resolved via configSource to extract runtime creds
	var fromClient, toClient s3client.Client
	fromClient, toClient, err = s.getClients(ctx, p.ID.User(), p.ID.FromStorage(), p.ID.ToStorage(), p.JobID)
    if err != nil {
        return fmt.Errorf("migration obj copy: unable to get clients: %w", err)
    }
    if err = s.rc.EnsureRuntimeUser(p.ID.FromStorage(), p.ID.User(), fromClient.Config().Address, fromClient.Config().Provider, fromClient.Config().Credentials[p.ID.User()].AccessKeyID, fromClient.Config().Credentials[p.ID.User()].SecretAccessKey); err != nil {
        return fmt.Errorf("migration obj copy: ensure runtime user (from) failed: %w", err)
    }
    if err = s.rc.EnsureRuntimeUser(p.ID.ToStorage(), p.ID.User(), toClient.Config().Address, toClient.Config().Provider, toClient.Config().Credentials[p.ID.User()].AccessKeyID, toClient.Config().Credentials[p.ID.User()].SecretAccessKey); err != nil {
        return fmt.Errorf("migration obj copy: ensure runtime user (to) failed: %w", err)
    }

    err = lock.Do(ctx, time.Second*2, func() error {
        return s.rc.CopyTo(ctx, p.ID.User(), rclone.File{
            Storage: p.ID.FromStorage(),
            Bucket:  fromBucket,
            Name:    p.Obj.Name,
        }, rclone.File{
            Storage: p.ID.ToStorage(),
            Bucket:  toBucket,
            Name:    p.Obj.Name,
        }, p.Obj.Size)
    })
	if err != nil {
		if errors.Is(err, dom.ErrNotFound) {
			logger.Warn().Msg("migration obj copy: skip object sync: object missing in source")
			return nil
		}
		return fmt.Errorf("migration obj copy: unable to copy with rclone: %w", err)
	}

    // clients already fetched above

	// 2. sync obj ACL
	err = s.syncObjectACL(ctx, fromClient, toClient, fromBucket, p.Obj.Name, toBucket)
	if err != nil {
		return err
	}

	// 3. sync obj tags
	err = s.syncObjectTagging(ctx, fromClient, toClient, fromBucket, p.Obj.Name, toBucket)
	if err != nil {
		return err
	}

	if fromVer != 0 {
		err = s.versionSvc.UpdateIfGreater(ctx, domObj, destVersionKey, fromVer)
		if err != nil {
			return fmt.Errorf("migration obj copy: unable to update obj meta: %w", err)
		}
	}
	logger.Info().Msg("migration obj copy: done")

	return nil
}
