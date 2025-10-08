package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	appdb "github.com/clyso/chorus/pkg/db"
	repodb "github.com/clyso/chorus/pkg/repository/db"
)

// This is a throwaway test utility to validate DB config, repositories, and loader.
// Build and run locally: go run ./tools/dbtest
func main() {
    ctx := context.Background()

    // Open DB from config+env
    cfg := appdb.FromEnv()
    gdb, err := appdb.Open(cfg)
    if err != nil {
        log.Fatalf("open db: %v", err)
    }
    defer appdb.Close(gdb)

    // Ping
    pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    if err := appdb.Ping(pingCtx, gdb); err != nil {
        log.Fatalf("ping db: %v", err)
    }
    fmt.Println("DB connected")

    // Prepare repositories
    _ = repodb.NewStorageRepository(gdb) // not used directly below; validated via loader
    jobRepo := repodb.NewReplicateJobRepository(gdb)

    // Create sample data (idempotent-ish)
    projectID := uuid.New()
    fromID := uuid.New()
    toID := uuid.New()
    jobID := uuid.New()

    // Insert storages
    fromStorage := &repodb.Storage{
        ID: fromID,
        Name: "main-storage-test",
        Address: "http://localhost:9000",
        Provider: "minio",
        IsMain: true,
        IsSecure: false,
        DefaultRegion: "us-east-1",
        ProjectID: projectID,
        AccessKeyID: "minioadmin",
        SecretAccessKey: "minioadmin",
    }
    toStorage := &repodb.Storage{
        ID: toID,
        Name: "follower-storage-test",
        Address: "http://localhost:9001",
        Provider: "minio",
        IsMain: false,
        IsSecure: false,
        DefaultRegion: "us-east-1",
        ProjectID: projectID,
        AccessKeyID: "minioadmin",
        SecretAccessKey: "minioadmin",
    }
    // Upsert storages using GORM
    if err := gdb.WithContext(ctx).Save(fromStorage).Error; err != nil {
        log.Fatalf("save from storage: %v", err)
    }
    if err := gdb.WithContext(ctx).Save(toStorage).Error; err != nil {
        log.Fatalf("save to storage: %v", err)
    }

    // Create replicate job
    job := &repodb.ReplicateJob{
        ID: jobID,
        ProjectID: projectID,
        Bucket: "test-bucket",
        FromID: fromID,
        ToID: toID,
        ToBucket: "test-bucket-copy",
        Status: "pending",
    }
    if err := jobRepo.Create(ctx, job); err != nil {
        log.Fatalf("create job: %v", err)
    }
    fmt.Printf("Created job %s\n", jobID)

    // Load with repo (preloads storages)
    loaded, err := jobRepo.GetByID(ctx, jobID)
    if err != nil {
        log.Fatalf("load job: %v", err)
    }
    if loaded == nil || loaded.FromStorage == nil || loaded.ToStorage == nil {
        log.Fatalf("loaded job missing storages: %+v", loaded)
    }
    fmt.Printf("Repo OK: job %s bucket %s from %s to %s\n", loaded.ID, loaded.Bucket, loaded.FromStorage.Name, loaded.ToStorage.Name)

    // Load via ConfigLoader with cache
    loader := repodb.NewConfigLoader(gdb, 2*time.Minute)
    rc, err := loader.LoadConfig(ctx, jobID)
    if err != nil {
        log.Fatalf("loader load: %v", err)
    }
    fmt.Printf("Loader OK: job %s bucket %s -> %s from %s to %s\n", rc.JobID, rc.Bucket, rc.ToBucket, rc.FromStorage.Name, rc.ToStorage.Name)

    // Update status and test cache invalidation
    if err := loader.UpdateJobStatus(ctx, jobID, "running"); err != nil {
        log.Fatalf("update status: %v", err)
    }
    rc2, err := loader.LoadConfig(ctx, jobID)
    if err != nil {
        log.Fatalf("loader reload: %v", err)
    }
    fmt.Printf("Status update OK: %s -> %s\n", rc.Status, rc2.Status)

    fmt.Println("All checks passed")
}


