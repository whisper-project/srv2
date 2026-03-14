/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/whisper-project/srv2/api/console"
	"github.com/whisper-project/srv2/lifecycle"
	"github.com/whisper-project/srv2/platform"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the whisper server",
	Long:  `Runs the whisper server until it's killed by signal.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		address, _ := cmd.Flags().GetString("address")
		port, _ := cmd.Flags().GetString("port")
		err := platform.SetConfig(env)
		if err != nil {
			panic(fmt.Sprintf("Can't load configuration: %v", err))
		}
		serve(address, port)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Args = cobra.NoArgs
	serveCmd.Flags().StringP("env", "e", "development", "The environment to run in")
	serveCmd.Flags().StringP("address", "a", "127.0.0.1", "The IP address to listen on")
	serveCmd.Flags().StringP("port", "p", "8080", "The port to listen on")
}

func serve(address, port string) {
	r, err := lifecycle.CreateEngine()
	if err != nil {
		panic(err)
	}
	consoleClient := r.Group("/api/console/v0")
	console.AddRoutes(consoleClient)
	lifecycle.Startup(r, fmt.Sprintf("%s:%s", address, port))
}
