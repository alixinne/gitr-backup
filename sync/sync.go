package sync

import (
	"context"
	"errors"
	"gitr-backup/config"
	"gitr-backup/vcs"
	"gitr-backup/vcs/repository"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"
)

type syncContext struct {
	ctx             context.Context
	clients         []vcs.Vcs
	sourcesByPrefix map[string]vcs.Vcs
	sourceMapping   map[string]repository.Repository
	mtx             sync.Mutex
}

func newSyncContext(ctx context.Context, clients []vcs.Vcs) (*syncContext, error) {
	// Build the prefix lookup map
	prefixClients := make(map[string]vcs.Vcs)
	for _, source := range clients {
		cnf := source.GetConfig()
		if cnf.Usage != "source" {
			continue
		}

		// Extract the hostname part of the url
		parsed, err := url.Parse(cnf.BaseUrl)
		if err != nil {
			return nil, err
		}

		// Clear the path
		parsed.Path = ""

		// Convert back to string
		prefixStr := parsed.String()
		prefixClients[prefixStr] = source
	}

	return &syncContext{
		ctx:             ctx,
		clients:         clients,
		sourcesByPrefix: prefixClients,
		sourceMapping:   map[string]repository.Repository{},
		mtx:             sync.Mutex{},
	}, nil
}

func (state *syncContext) processDestination(destination vcs.Vcs) error {
	logger := vcs.GetLogger(destination)

	logger.Info().Msg("Analyzing state of destination")

	repos, err := destination.GetRepositories(state.ctx)
	if err != nil {
		return err
	}

	var errCount int32 = 0

	// TODO: Make this configurable
	sem := semaphore.NewWeighted(1)
	wg := sync.WaitGroup{}
	wg.Add(len(repos))

	// For each known destination repository, try to update it from the source
	for _, destRepo := range repos {
		go func(destRepo repository.Repository) {
			sem.Acquire(state.ctx, 1)
			defer sem.Release(1)
			defer wg.Done()

			logger := logger.With().Str("repository", destRepo.GetName()).Logger()

			err := state.processRepo(logger, destRepo)
			if err != nil {
				logger.Error().Err(err).Send()
				atomic.AddInt32(&errCount, 1)
			}
		}(destRepo)
	}

	wg.Wait()

	// For each source repository, upload it to the destination if there is no matching destination repository
	for _, source := range state.clients {
		config := source.GetConfig()
		if config.Usage != "source" {
			continue
		}

		logger := logger.With().Str("source", config.Name).Logger()

		// Get source repositories
		sourceRepos, err := source.GetRepositories(state.ctx)
		if err != nil {
			logger.Error().Err(err).Msg("Could not fetch source repositories")
			atomic.AddInt32(&errCount, 1)
			continue
		}

		wg.Add(len(sourceRepos))

		for _, sourceRepo := range sourceRepos {
			go func(sourceRepo repository.Repository) {
				sem.Acquire(state.ctx, 1)
				defer sem.Release(1)
				defer wg.Done()

				logger := logger.With().Str("repository", sourceRepo.GetName()).Logger()

				url := sourceRepo.GetUrl()
				_, found := state.sourceMapping[url]

				if !found {
					logger.Info().Msg("Not found in backup, creating")
					err := state.backupNewRepo(logger, destination, sourceRepo)
					if err != nil {
						logger.Error().Err(err).Msg("Could not backup repository")
						atomic.AddInt32(&errCount, 1)
					}
				}
			}(sourceRepo)
		}
	}

	wg.Wait()

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

	// Create the sync context
	state, err := newSyncContext(ctx, clients)
	if err != nil {
		return err
	}

	// For each backup destination, check the source repositories
	errCount := 0
	for _, destination := range clients {
		if destination.GetConfig().Usage != "backup" {
			continue
		}

		err := state.processDestination(destination)
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
