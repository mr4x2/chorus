/*
 * Copyright © 2023 Clyso GmbH
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

package cmd

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	pb "github.com/clyso/chorus/proto/gen/go/chorus"
	"github.com/clyso/chorus/tools/chorctl/internal/api"
)

var (
	raFrom     string
	raTo       string
	raUser     string
	raAgentURL string
	raBucket   string
	raToBucket string
	raJobID    string
	raDryRun   bool
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "adds new bucket replication rule",
	Long: `Example:
chorctl repl add -f main -t follower -u admin -b bucket1
  - will replicate bucket "bucket1" from storage "main" to storage "follower"

chorctl repl add -f main -t follower -u admin -b src-bucket --to-bucket=dest-bucket
  - will replicate bucket "src-bucket" from storage "main" to bucket "dest-bucket" in storage "follower"

chorctl repl add --job-id=123e4567-e89b-12d3-a456-426614174000
  - will replicate using database job ID (DB-backed replication)
  - bucket names and storage config are read from database

chorctl repl add -f main -t follower -u admin -b bucket1 --dry-run
  - will validate configuration without creating replication`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		conn, err := api.Connect(ctx, address)
		if err != nil {
			logrus.WithError(err).WithField("address", address).Fatal("unable to connect to api")
		}
		defer conn.Close()
		client := pb.NewChorusClient(conn)

		// For job-id mode, provide defaults for required fields
		user := raUser
		bucket := raBucket
		if raJobID != "" {
			if user == "" {
				user = "default" // Default user for job-id mode
			}
			if bucket == "" {
				bucket = "default" // Default bucket for job-id mode (will be overridden by database)
			}
		}

		req := &pb.AddBucketReplicationRequest{
			User:        user,
			FromStorage: raFrom,
			ToStorage:   raTo,
			FromBucket:  bucket,
			ToBucket:    raToBucket,
			DryRun:      raDryRun,
		}
		if raAgentURL != "" {
			req.AgentUrl = &raAgentURL
		}
		if raJobID != "" {
			req.JobId = &raJobID
		}
		if raToBucket == "" && bucket != "" {
			req.ToBucket = bucket
		}

		_, err = client.AddBucketReplication(ctx, req)
		if err != nil {
			logrus.WithError(err).WithField("address", address).Fatal("unable to add replication")
		}

		if raDryRun {
			logrus.Info("Dry run completed successfully - configuration is valid")
		} else {
			logrus.Info("Replication added successfully")
		}
	},
}

func init() {
	replCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&raFrom, "from", "f", "", "from storage")
	addCmd.Flags().StringVarP(&raTo, "to", "t", "", "to storage")
	addCmd.Flags().StringVarP(&raUser, "user", "u", "", "storage user (required for YAML-based replication, optional for job-id)")
	addCmd.Flags().StringVar(&raAgentURL, "agent-url", "", "notifications agent url")
	addCmd.Flags().StringVarP(&raBucket, "bucket", "b", "", "bucket name to replicate (required for YAML-based replication, optional for job-id)")
	addCmd.Flags().StringVar(&raToBucket, "to-bucket", "", "custom destination bucket name. Set if destination bucket should have different name from source bucket")
	addCmd.Flags().StringVar(&raJobID, "job-id", "", "database job ID for DB-backed replication (optional)")
	addCmd.Flags().BoolVar(&raDryRun, "dry-run", false, "validate configuration without creating replication")

	// Add validation: either job-id OR (from + to + user + bucket) must be provided
	addCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if raJobID == "" {
			// Traditional YAML-based replication requires all fields
			if raFrom == "" || raTo == "" || raUser == "" || raBucket == "" {
				return fmt.Errorf("for YAML-based replication, --from, --to, --user, and --bucket are required")
			}
		} else {
			// Job ID-based replication - validate no conflicting flags
			if raFrom != "" || raTo != "" {
				return fmt.Errorf("cannot specify both --job-id and --from/--to flags")
			}
			// User and bucket are optional for job-id mode (will be read from database)
			// But we still need them for the gRPC request structure
		}
		return nil
	}

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// addCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// addCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
