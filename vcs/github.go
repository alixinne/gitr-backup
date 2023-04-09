package vcs

import (
	"context"
	"errors"
	"gitr-backup/config"
	"gitr-backup/vcs/repository"
	"strings"

	"github.com/google/go-github/v50/github"
	"github.com/rs/zerolog/log"
)

type GitHub struct {
	config   *config.Host
	client   *github.Client
	username string
}

func NewGitHubClient(ctx context.Context, config config.Host) (*GitHub, error) {
	logger := log.With().Str("host", config.Name).Logger()

	logger.Info().Msg("Initializing client")

	var client *github.Client

	if config.BaseUrl != "" && !strings.HasPrefix(config.BaseUrl, "https://github.com") {
		return nil, errors.New("GHES not supported yet")
	} else {
		client = github.NewTokenClient(ctx, config.Token)
	}

	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	username := user.GetLogin()
	logger.Info().Msgf("Logged in as %s", username)

	return &GitHub{config: &config, client: client, username: username}, nil
}

func (this *GitHub) GetConfig() *config.Host {
	return this.config
}

func (this *GitHub) GetRepositories(ctx context.Context) ([]repository.Repository, error) {
	logger := log.With().Str("host", this.config.Name).Logger()

	allRepos := []repository.Repository{}
	options := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{
			PerPage: 50,
		},
	}

	for {
		repos, resp, err := this.client.Repositories.List(ctx, this.username, options)
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			logger.Debug().Msgf("Found repository: %s (%s)", repo.GetName(), repo.GetDescription())
			allRepos = append(allRepos, &githubRepository{
				host: this,
				repo: repo,
			})
		}

		if resp.NextPage == 0 {
			break
		}

		options.ListOptions.Page = resp.NextPage
	}

	return allRepos, nil
}

func (this *GitHub) GetRepositoryByUrl(ctx context.Context, url string) (*repository.Repository, error) {
	repositoryParts := strings.SplitN(strings.TrimLeft(strings.TrimPrefix(url, this.config.BaseUrl), "/"), "/", 3)
	if len(repositoryParts) != 2 {
		return nil, errors.New("Invalid repository url for this host")
	}

	repo, _, err := this.client.Repositories.Get(ctx, repositoryParts[0], repositoryParts[1])
	if err != nil {
		return nil, err
	}

	var ghRepo repository.Repository = &githubRepository{
		host: this,
		repo: repo,
	}

	return &ghRepo, nil
}

func (this *GitHub) CreateRepository(ctx context.Context, options *CreateRepositoryOptions) (repository.Repository, error) {
	repo, _, err := this.client.Repositories.Create(ctx, "", &github.Repository{
		Name:        &options.Name,
		Description: &options.Description,
	})

	if err != nil {
		return nil, err
	}

	return &githubRepository{
		host: this,
		repo: repo,
	}, nil
}
