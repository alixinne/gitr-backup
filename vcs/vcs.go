package vcs

import (
	"context"
	"errors"
	"fmt"
	"gitr-backup/config"
	"gitr-backup/vcs/repository"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Vcs interface {
	GetConfig() *config.Host
	GetRepositories(ctx context.Context) ([]repository.Repository, error)
	GetRepositoryByUrl(ctx context.Context, url string) (*repository.Repository, error)
}

func LoadClients(ctx context.Context, config *config.Config) ([]Vcs, error) {
	result := []Vcs{}

	for _, host := range config.Hosts {
		var err error
		var client Vcs

		if host.Type == "gitea" {
			client, err = NewGiteaClient(ctx, host)
		} else if host.Type == "github" {
			client, err = NewGitHubClient(ctx, host)
		} else {
			err = errors.New(fmt.Sprintf("Unsupported host type: %s", host.Type))
		}

		if err != nil {
			return nil, err
		}

		result = append(result, client)
	}

	return result, nil
}

func GetLogger(vcs Vcs) zerolog.Logger {
	return log.With().Str("host", vcs.GetConfig().Name).Logger()
}
