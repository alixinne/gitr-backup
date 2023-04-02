package repository

import (
	"context"
	"fmt"
)

type Branch struct {
	Name string `diff:"name, identifier"`
	Sha string `diff:"sha"`
}

func (branch Branch) RefName() string {
	return fmt.Sprintf("refs/heads/%s", branch.Name)
}

type Repository interface {
	GetName() string
	GetDescription() string
	AddLabel(ctx context.Context, label string) error
	RemoveLabel(ctx context.Context, label string) error
	ListBranches(ctx context.Context) ([]Branch, error)
	GetHttpsCloneUrl() (string, error)
}
