package vcs

import (
	"context"
	"errors"
	"gitr-backup/config"
	"gitr-backup/vcs/repository"
	"strconv"
	"sync"

	"code.gitea.io/sdk/gitea"
	"github.com/rs/zerolog/log"
)

type Gitea struct {
	config         *config.Host
	client         *gitea.Client
	mutex          *sync.Mutex
	username       string
	initialContext context.Context
}

func NewGiteaClient(ctx context.Context, config config.Host) (*Gitea, error) {
	logger := log.With().Str("host", config.Name).Logger()

	logger.Info().Msg("Initializing client")

	client, err := gitea.NewClient(config.BaseUrl, gitea.SetToken(config.Token), gitea.SetContext(ctx))
	if err != nil {
		return nil, err
	}

	user, _, err := client.GetMyUserInfo()
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	username := user.UserName
	logger.Info().Msgf("Logged in as %s", user.FullName)

	return &Gitea{config: &config, client: client, mutex: &sync.Mutex{}, username: username, initialContext: ctx}, nil
}

func (this *Gitea) withContext(ctx context.Context, cb func(client *gitea.Client) error) error {
	// We need the mutex to protect against setting the default context for the current request
	this.mutex.Lock()

	this.client.SetContext(ctx)
	err := cb(this.client)
	this.client.SetContext(this.initialContext)

	this.mutex.Unlock()

	return err
}

func (this *Gitea) GetConfig() *config.Host {
	return this.config
}

func (this *Gitea) GetRepositories(ctx context.Context) ([]repository.Repository, error) {
	logger := log.With().Str("host", this.config.Name).Logger()

	allRepos := []repository.Repository{}
	options := gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{
			Page:     1,
			PageSize: 50,
		},
	}

	for {
		var repos []*gitea.Repository
		var resp *gitea.Response

		err := this.withContext(ctx, func(client *gitea.Client) error {
			var err error
			repos, resp, err = client.ListMyRepos(options)
			return err
		})

		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			logger.Debug().Msgf("Found repository: %s (%s)", repo.Name, repo.Description)
			allRepos = append(allRepos, &giteaRepository{
				host: this,
				repo: repo,
			})
		}

		totalCount, err := strconv.Atoi(resp.Header.Get("X-Total-Count"))
		if err != nil {
			return nil, err
		}

		if totalCount <= len(allRepos) {
			break
		}

		options.ListOptions.Page += 1
	}

	return allRepos, nil
}

func (this *Gitea) GetRepositoryByUrl(ctx context.Context, url string) (*repository.Repository, error) {
	return nil, errors.New("not implemented")
}
