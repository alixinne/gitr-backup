package vcs

import (
	"context"
	"fmt"
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

func (repo *giteaRepository) ListRefs(ctx context.Context) ([]repository.Ref, error) {
	allRefs := []repository.Ref{}

	// List branches
	options := gitea.ListRepoBranchesOptions{
		ListOptions: gitea.ListOptions{
			Page: 1,
			PageSize: 50,
		},
	}

	for {
		refs, resp, err := repo.host.client.ListRepoBranches(repo.repo.Owner.UserName, repo.repo.Name, options)
		if err != nil {
			return nil, err
		}

		for _, ref := range refs {
			allRefs = append(allRefs, repository.Ref{
				Name: ref.Name,
				Sha: ref.Commit.ID,
				RefName: fmt.Sprintf("refs/heads/%s", ref.Name),
			})
		}

		totalCount, err := strconv.Atoi(resp.Header.Get("X-Total-Count"))
		if err != nil {
			return nil, err
		}

		if totalCount <= len(allRefs) {
			break
		}

		options.ListOptions.Page += 1
	}

	// List tags
	tagOptions := gitea.ListRepoTagsOptions{
		ListOptions: gitea.ListOptions{
			Page: 1,
			PageSize: 50,
		},
	}

	baseCount := len(allRefs)

	for {
		refs, resp, err := repo.host.client.ListRepoTags(repo.repo.Owner.UserName, repo.repo.Name, tagOptions)
		if err != nil {
			return nil, err
		}

		for _, ref := range refs {
			allRefs = append(allRefs, repository.Ref{
				Name: ref.Name,
				Sha: ref.Commit.SHA,
				RefName: fmt.Sprintf("refs/tags/%s", ref.Name),
			})
		}

		totalCount, err := strconv.Atoi(resp.Header.Get("X-Total-Count"))
		if err != nil {
			return nil, err
		}

		if totalCount <= len(allRefs) - baseCount {
			break
		}

		tagOptions.ListOptions.Page += 1
	}


	return allRefs, nil
}

func (repo *giteaRepository) GetHttpsCloneUrl() (string, error) {
	parsed, err := url.Parse(repo.repo.CloneURL)
	if err != nil {
		return "", err
	}

	parsed.User = url.UserPassword(repo.host.username, repo.host.config.Token)

	return parsed.String(), nil
}

func (repo *giteaRepository) GetUrl() string {
	return repo.repo.HTMLURL
}
