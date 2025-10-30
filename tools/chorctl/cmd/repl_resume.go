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

	"github.com/sirupsen/logrus"

	pb "github.com/clyso/chorus/proto/gen/go/chorus"
	"github.com/clyso/chorus/tools/chorctl/internal/api"

	"github.com/spf13/cobra"
)

var (
	rrJobID    string
	rrFrom     string
	rrTo       string
	rrUser     string
	rrBucket   string
	rrToBucket string
)

// resumeCmd represents the pause command
var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "resumes bucket replication rule",
	Long:  `Examples:\nchorctl repl resume --job-id=<job-id>\nchorctl repl resume -f main -t follower -u admin -b bucket1`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		conn, err := api.Connect(ctx, address)
		if err != nil {
			logrus.WithError(err).WithField("address", address).Fatal("unable to connect to api")
		}
		defer conn.Close()
		client := pb.NewChorusClient(conn)

		jobIDUsed := rrJobID != ""
		legacyUsed := rrFrom != "" || rrTo != "" || rrUser != "" || rrBucket != ""
		if jobIDUsed && legacyUsed {
			logrus.Fatal("Use either --job-id OR --from/--to/--user/--bucket, not both")
		}
		if jobIDUsed {
			req := &pb.ReplicationRequest{JobId: &rrJobID}
			_, err = client.ResumeReplication(ctx, req)
			if err != nil {
				logrus.WithError(err).Fatal("unable to resume replication by job-id")
			}
			return
		}
		req := &pb.ReplicationRequest{
			User:     rrUser,
			Bucket:   rrBucket,
			From:     rrFrom,
			To:       rrTo,
			ToBucket: rrToBucket,
		}
		if rrToBucket == "" {
			req.ToBucket = rrBucket
		}
		_, err = client.ResumeReplication(ctx, req)
		if err != nil {
			logrus.WithError(err).Fatal("unable to resume replication")
		}
	},
}

func init() {
	replCmd.AddCommand(resumeCmd)
	resumeCmd.Flags().StringVar(&rrJobID, "job-id", "", "replicate job UUID (for database-backed replication)")
	resumeCmd.Flags().StringVarP(&rrFrom, "from", "f", "", "from storage")
	resumeCmd.Flags().StringVarP(&rrTo, "to", "t", "", "to storage")
	resumeCmd.Flags().StringVarP(&rrUser, "user", "u", "", "storage user")
	resumeCmd.Flags().StringVarP(&rrBucket, "bucket", "b", "", "bucket name")
	resumeCmd.Flags().StringVar(&rrToBucket, "to-bucket", "", "custom destinatin bucket name. Set if destination bucket should have different name from source bucket")
	// Don't mark required for legacy flags, let command error if wrong usage
}
