package vcs

import (
	"context"
	"errors"
	"fmt"
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

func (repo *githubRepository) ListRefs(ctx context.Context) ([]repository.Ref, error) {
	allRefs := []repository.Ref{}

	options := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 50,
		},
	}

	for {
		refs, resp, err := repo.host.client.Repositories.ListBranches(ctx, repo.repo.GetOwner().GetLogin(), repo.repo.GetName(), options)
		if err != nil {
			return nil, err
		}

		for _, ref := range refs {
			allRefs = append(allRefs, repository.Ref{
				Name:    ref.GetName(),
				Sha:     ref.GetCommit().GetSHA(),
				RefName: fmt.Sprintf("refs/heads/%s", ref.GetName()),
			})
		}

		if resp.NextPage == 0 {
			break
		}

		options.ListOptions.Page = resp.NextPage
	}

	tagOptions := &github.ListOptions{
		PerPage: 50,
	}

	for {
		refs, resp, err := repo.host.client.Repositories.ListTags(ctx, repo.repo.GetOwner().GetLogin(), repo.repo.GetName(), tagOptions)
		if err != nil {
			return nil, err
		}

		for _, ref := range refs {
			allRefs = append(allRefs, repository.Ref{
				Name:    ref.GetName(),
				Sha:     ref.GetCommit().GetSHA(),
				RefName: fmt.Sprintf("refs/tags/%s", ref.GetName()),
			})
		}

		if resp.NextPage == 0 {
			break
		}

		tagOptions.Page = resp.NextPage
	}

	return allRefs, nil
}

func (repo *githubRepository) GetHttpsCloneUrl() (string, error) {
	parsed, err := url.Parse(repo.repo.GetCloneURL())
	if err != nil {
		return "", err
	}

	parsed.User = url.UserPassword(repo.host.username, repo.host.config.Token)

	return parsed.String(), nil
}
