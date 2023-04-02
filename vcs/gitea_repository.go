package vcs

import (
	"context"
	"gitr-backup/constants"
	"gitr-backup/vcs/repository"
	"net/url"
	"strconv"

	"code.gitea.io/sdk/gitea"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)


type giteaRepository struct {
	host              *Gitea
	repo              *gitea.Repository
	topics            map[string]struct{}
	topicsInitialized bool
}

func (repo *giteaRepository) getLogger() zerolog.Logger {
	return log.With().Str("host", repo.host.config.Name).Str("repository", repo.repo.Name).Logger()
}

func (repo *giteaRepository) ensureTopics(ctx context.Context) error {
	logger := repo.getLogger()

	user := repo.host.username
	name := repo.repo.Name

	if !repo.topicsInitialized {
		logger.Debug().Msg("Fetching topics for repository")

		var topics []string
		err := repo.host.withContext(ctx, func(client *gitea.Client) error {
			var err error
			topics, _, err = client.ListRepoTopics(user, name, gitea.ListRepoTopicsOptions{
				ListOptions: gitea.ListOptions{PageSize: 50},
			})

			return err
		})

		if err != nil {
			return nil
		}

		repo.topics = make(map[string]struct{})

		for _, topic := range topics {
			repo.topics[topic] = struct{}{}
		}

		repo.topicsInitialized = true
	}
	
	return nil
}

func (repo *giteaRepository) GetName() string {
	return repo.repo.Name
}

func (repo *giteaRepository) GetDescription() string {
	return repo.repo.Description
}

func (repo *giteaRepository) AddLabel(ctx context.Context, label string) error {
	logger := repo.getLogger()

	user := repo.repo.Owner.UserName
	name := repo.repo.Name

	err := repo.ensureTopics(ctx)
	if err != nil {
		return nil
	}

	_, found := repo.topics[label]
	if !found {
		logger.Info().Msgf("Adding topic %s to repository", label)
	} else {
		return nil
	}

	if !ctx.Value(constants.DRY_RUN).(bool) {
		err = repo.host.withContext(ctx, func(client *gitea.Client) error {
			_, err := client.AddRepoTopic(user, name, label)
			return err
		})
	}

	if err == nil {
		repo.topics[label] = struct{}{}
	}

	return err
}

func (repo *giteaRepository) RemoveLabel(ctx context.Context, label string) error {
	logger := repo.getLogger()

	user := repo.repo.Owner.UserName
	name := repo.repo.Name

	err := repo.ensureTopics(ctx)
	if err != nil {
		return nil
	}

	_, found := repo.topics[label]
	if found {
		logger.Info().Msgf("Removing topic %s from repository", label)
	} else {
		return nil
	}

	if !ctx.Value(constants.DRY_RUN).(bool) {
		err = repo.host.withContext(ctx, func(client *gitea.Client) error {
			_, err := client.DeleteRepoTopic(user, name, label)
			return err
		})
	}

	if err == nil {
		delete(repo.topics, label)
	}

	return err
}

func (repo *giteaRepository) ListBranches(ctx context.Context) ([]repository.Branch, error) {
	allBranches := []repository.Branch{}
	options := gitea.ListRepoBranchesOptions{
		ListOptions: gitea.ListOptions{
			Page: 1,
			PageSize: 50,
		},
	}

	for {
		branches, resp, err := repo.host.client.ListRepoBranches(repo.repo.Owner.UserName, repo.repo.Name, options)
		if err != nil {
			return nil, err
		}

		for _, branch := range branches {
			allBranches = append(allBranches, repository.Branch{
				Name: branch.Name,
				Sha: branch.Commit.ID,
			})
		}

		totalCount, err := strconv.Atoi(resp.Header.Get("X-Total-Count"))
		if err != nil {
			return nil, err
		}

		if totalCount <= len(allBranches) {
			break
		}

		options.ListOptions.Page += 1
	}

	return allBranches, nil
}

func (repo *giteaRepository) GetHttpsCloneUrl() (string, error) {
	parsed, err := url.Parse(repo.repo.CloneURL)
	if err != nil {
		return "", err
	}

	parsed.User = url.UserPassword(repo.host.username, repo.host.config.Token)

	return parsed.String(), nil
}
