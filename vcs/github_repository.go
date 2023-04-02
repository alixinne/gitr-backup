package vcs

import (
	"context"
	"errors"
	"gitr-backup/vcs/repository"
	"net/url"

	"github.com/google/go-github/v50/github"
)

type githubRepository struct {
	host *GitHub
	repo *github.Repository
}

func (repo *githubRepository) GetName() string {
	return repo.repo.GetName()
}

func (repo *githubRepository) GetDescription() string {
	return repo.repo.GetDescription()
}

func (repo *githubRepository) AddLabel(ctx context.Context, label string) error {
	return errors.New("not implemented")
}

func (repo *githubRepository) RemoveLabel(ctx context.Context, label string) error {
	return errors.New("not implemented")
}

func (repo *githubRepository) ListBranches(ctx context.Context) ([]repository.Branch, error) {
	allBranches := []repository.Branch{}
	options := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 50,
		},
	}

	for {
		branches, resp, err := repo.host.client.Repositories.ListBranches(ctx, repo.repo.GetOwner().GetLogin(), repo.repo.GetName(), options)
		if err != nil {
			return nil, err
		}

		for _, branch := range branches {
			allBranches = append(allBranches, repository.Branch{
				Name: branch.GetName(),
				Sha: branch.GetCommit().GetSHA(),
			})
		}

		if resp.NextPage == 0 {
			break
		}

		options.ListOptions.Page = resp.NextPage
	}

	return allBranches, nil
}

func (repo *githubRepository) GetHttpsCloneUrl() (string, error) {
	parsed, err := url.Parse(repo.repo.GetCloneURL())
	if err != nil {
		return "", err
	}

	parsed.User = url.UserPassword(repo.host.username, repo.host.config.Token)

	return parsed.String(), nil
}
