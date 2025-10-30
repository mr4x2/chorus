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
	rdJobID    string // job-id flag for delete
	rdFrom     string
	rdTo       string
	rdUser     string
	rdBucket   string
	rdToBucket string
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "deletes bucket replication rule",
	Long:  `Examples:\nchorctl repl delete --job-id=<job-id>\nchorctl repl delete -f main -t follower -u admin -b bucket1`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		conn, err := api.Connect(ctx, address)
		if err != nil {
			logrus.WithError(err).WithField("address", address).Fatal("unable to connect to api")
		}
		defer conn.Close()
		client := pb.NewChorusClient(conn)

		jobIDUsed := rdJobID != ""
		legacyUsed := rdFrom != "" || rdTo != "" || rdUser != "" || rdBucket != ""
		if jobIDUsed && legacyUsed {
			logrus.Fatal("Use either --job-id OR --from/--to/--user/--bucket, not both")
		}
		if jobIDUsed {
			req := &pb.ReplicationRequest{JobId: &rdJobID}
			_, err = client.DeleteReplication(ctx, req)
			if err != nil {
				logrus.WithError(err).Fatal("unable to delete replication by job-id")
			}
			return
		}
		req := &pb.ReplicationRequest{
			User:     rdUser,
			Bucket:   rdBucket,
			From:     rdFrom,
			To:       rdTo,
			ToBucket: rdToBucket,
		}
		if rdToBucket == "" {
			req.ToBucket = rdBucket
		}
		_, err = client.DeleteReplication(ctx, req)
		if err != nil {
			logrus.WithError(err).Fatal("unable to delete replication")
		}
	},
}

func init() {
	replCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&rdJobID, "job-id", "", "replicate job UUID (for database-backed replication)")
	deleteCmd.Flags().StringVarP(&rdFrom, "from", "f", "", "from storage")
	deleteCmd.Flags().StringVarP(&rdTo, "to", "t", "", "to storage")
	deleteCmd.Flags().StringVarP(&rdUser, "user", "u", "", "storage user")
	deleteCmd.Flags().StringVarP(&rdBucket, "bucket", "b", "", "bucket name")
	deleteCmd.Flags().StringVar(&rdToBucket, "to-bucket", "", "custom destinatin bucket name. Set if destination bucket should have different name from source bucket")
	// Don't mark required for legacy flags, let command error if wrong usage
}
