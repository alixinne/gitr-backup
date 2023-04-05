package sync

import (
	"context"
	"errors"
	"fmt"
	"gitr-backup/constants"
	"gitr-backup/vcs"
	"gitr-backup/vcs/repository"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/maps"

	"github.com/r3labs/diff/v3"

	git "github.com/libgit2/git2go/v34"
)

type repositorySource struct {
	host   vcs.Vcs
	source string
}

type repositoryState struct {
	isBackup bool
	ignore   bool
}

func createMirrorRemote(repo *git.Repository, name, url string) (*git.Remote, error) {
	remote, err := repo.Remotes.CreateWithOptions(url, &git.RemoteCreateOptions{
		Name:      name,
		FetchSpec: "+refs/*:refs/*",
	})
	if err != nil {
		return nil, err
	}

	return remote, nil
}

func safeUrl(rawUrl string) string {
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return rawUrl
	}

	parsed.User = nil
	return parsed.String()
}

func (this *syncContext) findRepositorySource(logger zerolog.Logger, repository repository.Repository) (*repositorySource, repositoryState, error) {
	// Ensure labels are set correctly
	desc := repository.GetDescription()

	state := repositoryState{
		isBackup: strings.Contains(desc, constants.BACKUP_PREFIX),
		ignore:   strings.Contains(desc, constants.IGNORE_PREFIX),
	}

	if state.isBackup {
		sourceUrl := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(desc, constants.BACKUP_PREFIX, ""), constants.IGNORE_PREFIX, ""))

		sourceLogger := logger.With().Str("source", sourceUrl).Logger()
		if sourceUrl != "" {
			// Try parsing url
			parsedUrl, err := url.Parse(sourceUrl)
			if err != nil {
				sourceLogger.Warn().Err(err).Msg("Failed parsing URL for repository")
			} else {
				parsedUrl.Path = ""
				urlStr := parsedUrl.String()

				host, found := this.sourcesByPrefix[urlStr]
				if found {
					sourceLogger.Info().Str("source_host", host.GetConfig().Name).Msg("Found backup repository")

					this.mtx.Lock()
					// Record it in the source
					this.sourceMapping[sourceUrl] = repository
					this.mtx.Unlock()

					// We identified the source for this repository
					return &repositorySource{
						host:   host,
						source: sourceUrl,
					}, state, nil
				} else {
					sourceLogger.Warn().Str("source_host", "").Msg("Orphaned backup repository")

					// We have a source url, but no host
					return &repositorySource{
						source: sourceUrl,
					}, state, nil
				}
			}
		} else {
			sourceLogger.Warn().Msg("No source for repository")
		}
	}

	// By default, repositories are not backups
	return nil, state, nil
}

func (state *syncContext) ensureLabel(logger zerolog.Logger, repository repository.Repository, isBackup bool) error {
	// Ensure labels are set correctly
	desc := repository.GetDescription()
	if strings.HasPrefix(desc, constants.BACKUP_PREFIX) {
		err := repository.AddLabel(state.ctx, constants.BACKUP_LABEL)
		if err != nil {
			return err
		}

		err = repository.RemoveLabel(state.ctx, constants.PRIVATE_LABEL)
		if err != nil {
			return err
		}

	} else {
		err := repository.AddLabel(state.ctx, constants.PRIVATE_LABEL)
		if err != nil {
			return err
		}

		err = repository.RemoveLabel(state.ctx, constants.BACKUP_LABEL)
		if err != nil {
			return err
		}
	}

	return nil
}

func mirrorRefs(ctx context.Context, logger zerolog.Logger, sourceRepo, destRepo repository.Repository, changedRefs, deletedRefs []string) error {
	// Clone the remote repository
	dir, err := os.MkdirTemp("", "gitr-backup")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	// https://github.com/libgit2/pygit2/blob/acb4abbcb2ac7d59961ede6c6be2c43782f22f63/docs/recipes/git-clone-mirror.rst
	options := &git.CloneOptions{
		Bare:                 true,
		RemoteCreateCallback: createMirrorRemote,
	}

	sourceCloneUrl, err := sourceRepo.GetHttpsCloneUrl()
	if err != nil {
		return err
	}

	logger.Info().Str("path", dir).Str("clone_url", safeUrl(sourceCloneUrl)).Msg("Cloning source repository")
	cloned, err := git.Clone(sourceCloneUrl, dir, options)
	if err != nil {
		return err
	}

	// Switch to the destination remote
	destCloneUrl, err := destRepo.GetHttpsCloneUrl()
	if err != nil {
		return err
	}

	// Compute refspec to push
	refspecs := []string{}

	for _, k := range changedRefs {
		refspecs = append(refspecs, fmt.Sprintf("+%s:%s", k, k))
	}

	for _, k := range deletedRefs {
		refspecs = append(refspecs, fmt.Sprintf("+:%s", k))
	}

	logger.Info().
		Str("clone_url", safeUrl(destCloneUrl)).
		Any("refspecs", refspecs).
		Msg("Pushing to destination remote")

	remote, err := cloned.Remotes.Create("backup", destCloneUrl)
	if err != nil {
		return err
	}

	err = remote.Push(refspecs, &git.PushOptions{
		RemoteCallbacks: git.RemoteCallbacks{
			PushUpdateReferenceCallback: func(refname, status string) error {
				logger.Info().Str("refname", refname).Msgf("Updated ref")
				return nil
			},
			PushTransferProgressCallback: func(current, total uint32, bytes uint) error {
				logger.Info().Msgf("Progress: %d/%d", current, total)
				return nil
			},
		},
	})
	if err != nil {
		return err

	expected := sourceRepo.GetDefaultBranch()
	actual := destRepo.GetDefaultBranch()
	if actual != expected{
		logger.Info().Str("from", actual).Str("to", expected).Msg("Updating default branch")

		err := destRepo.SetDefaultBranch(ctx, expected)
		if err != nil {
			return err
		}
	}

	return nil
}

func (state *syncContext) backupNewRepo(logger zerolog.Logger, dest vcs.Vcs, sourceRepo repository.Repository) error {
	// Check the dry-run flag
	dryRun := state.ctx.Value(constants.DRY_RUN).(bool)
	if dryRun {
		logger.Info().Msg("Would create the destination repository, but dry-run mode is enabled")
		return nil
	}

	// Create the target repository
	destRepo, err := dest.CreateRepository(state.ctx, &vcs.CreateRepositoryOptions{
		Name:        sourceRepo.GetName(),
		Description: fmt.Sprintf("%s %s", constants.BACKUP_PREFIX, sourceRepo.GetUrl()),
	})
	if err != nil {
		return fmt.Errorf("Failed creating repository: %w", err)
	}

	// Add tags to the repository
	err = state.ensureLabel(logger, destRepo, true)
	if err != nil {
		return fmt.Errorf("Error ensuring labels: %w", err)
	}

	// Get all the refs in the source repo
	sourceRefs, err := sourceRepo.ListRefs(state.ctx)
	if err != nil {
		return fmt.Errorf("Failed getting source refs: %w", err)
	}

	allRefs := []string{}
	for _, ref := range sourceRefs {
		allRefs = append(allRefs, ref.RefName)
	}

	// Clone the source to the destination
	return mirrorRefs(state.ctx, logger, sourceRepo, destRepo, allRefs, nil)
}

func (this *syncContext) processRepo(logger zerolog.Logger, destRepo repository.Repository) error {
	// Identify the repository source
	source, state, err := this.findRepositorySource(logger, destRepo)
	if err != nil {
		return err
	}

	// Ensure it's labeled correctly in the destination
	err = this.ensureLabel(logger, destRepo, state.isBackup)
	if err != nil {
		return err
	}

	// If we have no source or source host, just ignore it
	if source == nil || source.host == nil {
		return nil
	}

	// If this repository is explicitely ignored, ignore it
	if state.ignore {
		return nil
	}

	// Try getting the source repository from the host
	sourceRepo, err := source.host.GetRepositoryByUrl(this.ctx, source.source)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed getting repository from source host: %v", err))
	}

	changedRefSet := map[string]struct{}{}
	deletedRefSet := map[string]struct{}{}

	// Get the refs for the source repository
	sourceRefs, err := (*sourceRepo).ListRefs(this.ctx)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed getting source repository refs: %v", err))
	}

	// Get the refs for the destination repository
	destRefs, err := destRepo.ListRefs(this.ctx)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed getting destination repository refs: %v", err))
	}

	changelog, err := diff.Diff(destRefs, sourceRefs, diff.DisableStructValues())
	if err != nil {
		return errors.New(fmt.Sprintf("Comparison error: %v", err))
	}

	if len(changelog) > 0 {
		for _, change := range changelog {
			if change.Type == "delete" {
				// In case of a deletion, the from field contains the entire ref
				name := change.From.(repository.Ref).RefName

				_, found := changedRefSet[name]
				// Only mark a ref as deleted if it wasn't changed already
				if !found {
					deletedRefSet[name] = struct{}{}
				}
			} else if change.Type == "update" {
				// The SHA of a ref changed, so we need to fetch the ref from its path
				// since the change entry only contains the SHA change and not the full entry
				i, _ := strconv.Atoi(change.Path[0])
				name := destRefs[i].RefName

				changedRefSet[name] = struct{}{}
				// A ref that is changed was in fact, not deleted
				delete(deletedRefSet, name)
			} else if change.Type == "create" {
				// On creation, the ref information is in the To field
				name := change.To.(repository.Ref).RefName

				changedRefSet[name] = struct{}{}
				// A ref that is changed was in fact, not deleted
				delete(deletedRefSet, name)
			} else {
				log.Fatal().Any("change", change).Msg("Unknown change type")
			}
		}

		logger.Info().
			Any("changelog", changelog).
			Any("changed", changedRefSet).
			Any("deleted", deletedRefSet).
			Msg("Differences found")
	} else {
		logger.Debug().Msg("No changes found in refs")
	}

	if len(changedRefSet) == 0 && len(deletedRefSet) == 0 {
		return nil
	}

	// Check the dry-run flag
	dryRun := this.ctx.Value(constants.DRY_RUN).(bool)
	if dryRun {
		logger.Info().Msg("Would synchronize the repositories, but dry-run mode is enabled")
		return nil
	}

	return mirrorRefs(this.ctx, logger, *sourceRepo, destRepo, maps.Keys(changedRefSet), maps.Keys(deletedRefSet))
}
