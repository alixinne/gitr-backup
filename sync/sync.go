package sync

import (
	"context"
	"errors"
	"gitr-backup/config"
	"gitr-backup/vcs"
	"gitr-backup/vcs/repository"
	"net/url"

	"github.com/rs/zerolog/log"
)

func processDestination(ctx context.Context, clients []vcs.Vcs, destination vcs.Vcs, sourcesByPrefix *map[string]*vcs.Vcs) error {
	logger := vcs.GetLogger(destination)

	logger.Info().Msg("Analyzing state of destination")

	repos, err := destination.GetRepositories(ctx)
	if err != nil {
		return err
	}

	errCount := 0
	sourceMapping := make(map[string]repository.Repository)

	// For each known destination repository, try to update it from the source
	for _, destRepo := range repos {
		logger := logger.With().Str("repository", destRepo.GetName()).Logger()

		err := processRepo(ctx, logger, destRepo, sourcesByPrefix, &sourceMapping)
		if err != nil {
			logger.Error().Err(err).Send()
			errCount += 1
		}
	}

	// For each source repository, upload it to the destination if there is no matching destination repository
	for _, source := range clients {
		config := source.GetConfig()
		if config.Usage != "source" {
			continue
		}

		logger := logger.With().Str("source", config.Name).Logger()

		// Get source repositories
		sourceRepos, err := source.GetRepositories(ctx)
		if err != nil {
			logger.Error().Err(err).Msg("Could not fetch source repositories")
			errCount += 1
			continue
		}

		for _, sourceRepo := range sourceRepos {
			logger := logger.With().Str("repository", sourceRepo.GetName()).Logger()

			url := sourceRepo.GetUrl()
			_, found := sourceMapping[url]

			if !found {
				logger.Info().Msg("Not found in backup, creating")
				err := backupNewRepo(ctx, logger, destination, sourceRepo)
				if err != nil {
					logger.Error().Err(err).Msg("Could not backup repository")
					errCount += 1
					continue
				}
			}
		}
	}

	if errCount > 0 {
		return errors.New("Some repositories failed")
	}

	return nil
}

func SyncHosts(ctx context.Context, config *config.Config) error {
	log.Info().Msgf("%d hosts configured", len(config.Hosts))

	clients, err := vcs.LoadClients(ctx, config)
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	// Build the prefix lookup map
	prefixClients := make(map[string]*vcs.Vcs)
	for i := range clients {
		source := &clients[i]
		cnf := (*source).GetConfig()
		if cnf.Usage != "source" {
			continue
		}

		// Extract the hostname part of the url
		parsed, err := url.Parse(cnf.BaseUrl)
		if err != nil {
			return err
		}

		// Clear the path
		parsed.Path = ""

		// Convert back to string
		prefixStr := parsed.String()
		prefixClients[prefixStr] = source
	}

	// For each backup destination, check the source repositories
	errCount := 0
	for _, destination := range clients {
		if destination.GetConfig().Usage != "backup" {
			continue
		}

		err := processDestination(ctx, clients, destination, &prefixClients)
		if err != nil {
			log.Error().Err(err).Str("host", destination.GetConfig().Name).Msg("Failed processing destination")
			errCount += 1
		}
	}

	if errCount > 0 {
		return errors.New("Some destinations have failed")
	}

	return nil
}
