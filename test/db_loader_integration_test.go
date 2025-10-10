package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	appdb "github.com/clyso/chorus/pkg/db"
	"github.com/clyso/chorus/pkg/log"
	repodb "github.com/clyso/chorus/pkg/repository/db"
)

// cleanupTestData removes any existing test data to avoid conflicts
func cleanupTestData(ctx context.Context, gdb *gorm.DB) {
	// Delete test jobs first (due to foreign key constraints)
	gdb.WithContext(ctx).Where("bucket LIKE ?", "it-bucket%").Delete(&repodb.ReplicateJob{})
	// Delete test storages
	gdb.WithContext(ctx).Where("name IN ?", []string{"it-from-storage", "it-to-storage"}).Delete(&repodb.Storage{})
}

// Integration test for DB loader and repositories.
// Skips if DB env is not configured.
func TestDBLoader_Integration(t *testing.T) {
	if os.Getenv("DB_DSN") == "" && os.Getenv("DB_HOST") == "" {
		t.Skip("DB env not configured; set DB_DSN or DB_HOST to run integration test")
	}

	ctx := context.Background()
	gdb, err := appdb.Open(appdb.FromEnv())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer appdb.Close(gdb)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := appdb.Ping(pingCtx, gdb); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	// Clean up any existing test data
	cleanupTestData(ctx, gdb)

	_ = repodb.NewStorageRepository(gdb, repodb.DefaultResilienceConfig(), log.GetLogger(&log.Config{
		Level: "warn",
	}, "test", "test")) // covered implicitly via loader preloads
	jobRepo := repodb.NewReplicateJobRepository(gdb, repodb.DefaultResilienceConfig(), log.GetLogger(&log.Config{
		Level: "warn",
	}, "test", "test"))

	// Seed minimal data
	projectID := uuid.New()
	fromID := uuid.New()
	toID := uuid.New()
	jobID := uuid.New()

	fromStorage := &repodb.Storage{
		ID:              fromID,
		Name:            "it-from-storage",
		Address:         "http://localhost:9000",
		Provider:        "minio",
		IsMain:          true,
		IsSecure:        false,
		DefaultRegion:   "us-east-1",
		ProjectID:       projectID,
		AccessKeyID:     "minioadmin1",
		SecretAccessKey: "minioadmin1",
	}
	toStorage := &repodb.Storage{
		ID:              toID,
		Name:            "it-to-storage",
		Address:         "http://localhost:9001",
		Provider:        "minio",
		IsMain:          false,
		IsSecure:        false,
		DefaultRegion:   "us-east-1",
		ProjectID:       projectID,
		AccessKeyID:     "minioadmin2",
		SecretAccessKey: "minioadmin2",
	}
	if err := gdb.WithContext(ctx).Save(fromStorage).Error; err != nil {
		t.Fatalf("save from storage: %v", err)
	}
	if err := gdb.WithContext(ctx).Save(toStorage).Error; err != nil {
		t.Fatalf("save to storage: %v", err)
	}

	job := &repodb.ReplicateJob{
		ID:        jobID,
		ProjectID: projectID,
		Bucket:    "it-bucket",
		FromID:    fromID,
		ToID:      toID,
		ToBucket:  "it-bucket-copy",
		Status:    "pending",
	}
	if err := jobRepo.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	// Loader
	loader := repodb.NewConfigLoader(gdb, time.Minute, repodb.DefaultResilienceConfig(), log.GetLogger(&log.Config{
		Level: "warn",
	}, "test", "test"))
	cfg, err := loader.LoadConfig(ctx, jobID)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.FromStorage == nil || cfg.ToStorage == nil {
		t.Fatalf("storages not resolved: %+v", cfg)
	}
	if cfg.Bucket != "it-bucket" || cfg.ToBucket != "it-bucket-copy" {
		t.Fatalf("unexpected buckets: %+v", cfg)
	}

	// Status updates
	if err := loader.UpdateJobStatus(ctx, jobID, "running"); err != nil {
		t.Fatalf("update status running: %v", err)
	}
	if err := loader.UpdateJobStatusWithReason(ctx, jobID, "failed", "test failure"); err != nil {
		t.Fatalf("update status failed: %v", err)
	}
}
