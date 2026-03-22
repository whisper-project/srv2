/*
 * Copyright 2024-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/whisper-project/whisper.server2/platform"

	"github.com/whisper-project/whisper.server2/api/console"
	"github.com/whisper-project/whisper.server2/lifecycle"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the whisper server",
	Long:  `Runs the whisper server until it's killed by signal.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := platform.SetConfig(environment); err != nil {
			fmt.Fprintf(os.Stderr, "Can't run in environment %q: %v\n", environment, err)
			os.Exit(1)
		}
		address, _ := cmd.Flags().GetString("address")
		port, _ := cmd.Flags().GetString("port")
		log.SetFlags(0)
		serve(address, port)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Args = cobra.NoArgs
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
