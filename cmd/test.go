/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"context"
	"log"

	"github.com/whisper-project/server.golang/platform"

	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Manage test data",
	Long: `This utility manages the data in the test database.
Use flags to indicate what you want to do.
Data is always loaded last.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		err := platform.PushConfig(env)
		if err != nil {
			panic(err)
		}
		log.Printf("Operating in the %s environment.", platform.GetConfig().Name)
		var cnt int
		if cnt, _ = cmd.Flags().GetCount("clear"); cnt > 0 {
			clearDb()
		}
		if cnt, _ = cmd.Flags().GetCount("clean"); cnt > 0 {
			log.Printf("Deleting all keys created by tests...")
		}
		if cnt, _ = cmd.Flags().GetCount("load"); cnt > 0 {
			loadKnownTestData()
		}
		log.Printf("Done.")
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.Args = cobra.NoArgs
	testCmd.Flags().StringP("env", "e", "test", "db environment to use")
	testCmd.Flags().Count("load", "load the known test values")
	testCmd.Flags().Count("clear", "remove all values")
	testCmd.Flags().Count("clean", "remove all test-created values")
	testCmd.MarkFlagsMutuallyExclusive("clear", "clean")
	testCmd.MarkFlagsOneRequired("load", "clear", "clean")
}

func clearDb() {
	ctx := context.Background()
	db, prefix := platform.GetDb()
	log.Printf("Deleting all keys that start with %q...", prefix)
	iter := db.Scan(ctx, 0, prefix+"*", 20).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		_ = db.Del(ctx, key)
	}
}

func loadKnownTestData() {
	log.Printf("Loading test data...")
}
