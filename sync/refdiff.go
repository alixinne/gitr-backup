package sync

import "gitr-backup/vcs/repository"

type RefdiffResult struct {
	ChangedRefs []repository.Ref
	DeletedRefs []repository.Ref
}

func refmapFromList(refs []repository.Ref) map[string]repository.Ref {
	result := map[string]repository.Ref{}
	for _, val := range refs {
		result[val.RefName] = val
	}
	return result
}

func listFromRefmap(refs map[string]repository.Ref) []repository.Ref {
	result := []repository.Ref{}
	for _, val := range refs {
		result = append(result, val)
	}
	return result
}

func FullRefdiff(refs []repository.Ref) RefdiffResult {
	return RefdiffResult{
		ChangedRefs: refs,
		DeletedRefs: []repository.Ref{},
	}
}

func Refdiff(sourceRefs, destRefs []repository.Ref) RefdiffResult {
	// Create lookup tables for the refs
	srefs := refmapFromList(sourceRefs)
	drefs := refmapFromList(destRefs)

	changedRefs := map[string]repository.Ref{}
	deletedRefs := map[string]repository.Ref{}

	for _, dref := range drefs {
		if _, ok := srefs[dref.RefName]; !ok {
			deletedRefs[dref.RefName] = dref
		}
	}

	for _, sref := range srefs {
		dref, ok := drefs[sref.RefName]
		if !ok || dref.Sha != sref.Sha {
			changedRefs[sref.RefName] = sref
		}
	}

	return RefdiffResult{
		ChangedRefs: listFromRefmap(changedRefs),
		DeletedRefs: listFromRefmap(deletedRefs),
	}
}

func (result RefdiffResult) Len() int {
	return len(result.ChangedRefs) + len(result.DeletedRefs)
}
