package cli

import (
	"context"
	"strings"

	"github.com/pawnkit/pawn-project/dependency"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
)

type revisionProviderRouter struct {
	github dependency.RevisionProvider
	git    dependency.RevisionProvider
}

func (p revisionProviderRouter) Resolve(
	ctx context.Context,
	dep manifest.Dependency,
	locked *lockfile.Package,
) (dependency.Revision, error) {
	if dep.Site == "" || strings.EqualFold(dep.Site, "github.com") {
		return p.github.Resolve(ctx, dep, locked)
	}
	return p.git.Resolve(ctx, dep, locked)
}
