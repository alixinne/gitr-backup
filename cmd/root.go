/*
Copyright © 2023 Alixinne <alixinne@pm.me>
*/
package cmd

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitr-backup/config"
	"gitr-backup/constants"
	"gitr-backup/sync"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gitr-backup",
	Short: "Backup solution for Git hosts",
	Run:   run,
}

var dryRun bool
var debugMode bool

func run(cmd *cobra.Command, args []string) {
	ctx := context.WithValue(context.Background(), constants.DRY_RUN, dryRun)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	if !debugMode {
		log.Logger = log.Logger.Level(zerolog.InfoLevel)
	}

	config, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	err = sync.SyncHosts(ctx, config, args)
	if err != nil {
		log.Fatal().Err(err).Send()
	}
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Dry-run mode")
	rootCmd.PersistentFlags().BoolVarP(&debugMode, "debug", "D", false, "Debug mode")
}
