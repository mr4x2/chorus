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
	rpJobID    string
	rpFrom     string
	rpTo       string
	rpUser     string
	rpBucket   string
	rpToBucket string
)

// pauseCmd represents the pause command
var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "pauses bucket replication rule",
	Long:  `Examples:\nchorctl repl pause --job-id=<job-id>\nchorctl repl pause -f main -t follower -u admin -b bucket1`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		conn, err := api.Connect(ctx, address)
		if err != nil {
			logrus.WithError(err).WithField("address", address).Fatal("unable to connect to api")
		}
		defer conn.Close()
		client := pb.NewChorusClient(conn)

		jobIDUsed := rpJobID != ""
		legacyUsed := rpFrom != "" || rpTo != "" || rpUser != "" || rpBucket != ""
		if jobIDUsed && legacyUsed {
			logrus.Fatal("Use either --job-id OR --from/--to/--user/--bucket, not both")
		}
		if jobIDUsed {
			// Use job-id based request (other fields are empty)
			req := &pb.ReplicationRequest{JobId: &rpJobID}
			_, err = client.PauseReplication(ctx, req)
			if err != nil {
				logrus.WithError(err).Fatal("unable to pause replication by job-id")
			}
			return
		}
		// Legacy path
		req := &pb.ReplicationRequest{
			User:     rpUser,
			Bucket:   rpBucket,
			From:     rpFrom,
			To:       rpTo,
			ToBucket: rpToBucket,
		}
		if rpToBucket == "" {
			req.ToBucket = rpBucket
		}
		_, err = client.PauseReplication(ctx, req)
		if err != nil {
			logrus.WithError(err).Fatal("unable to pause replication")
		}
	},
}

func init() {
	replCmd.AddCommand(pauseCmd)
	pauseCmd.Flags().StringVar(&rpJobID, "job-id", "", "replicate job UUID (for database-backed replication)")
	pauseCmd.Flags().StringVarP(&rpFrom, "from", "f", "", "from storage")
	pauseCmd.Flags().StringVarP(&rpTo, "to", "t", "", "to storage")
	pauseCmd.Flags().StringVarP(&rpUser, "user", "u", "", "storage user")
	pauseCmd.Flags().StringVarP(&rpBucket, "bucket", "b", "", "bucket name")
	pauseCmd.Flags().StringVar(&rpToBucket, "to-bucket", "", "custom destinatin bucket name. Set if destination bucket should have different name from source bucket")
	// Don't mark required for legacy flags, now handled conditionally
}
