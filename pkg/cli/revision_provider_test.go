package cli

import (
	"context"
	"testing"

	"github.com/pawnkit/pawn-project/dependency"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
)

type namedRevisionProvider string

func (p namedRevisionProvider) Resolve(
	context.Context,
	manifest.Dependency,
	*lockfile.Package,
) (dependency.Revision, error) {
	return dependency.Revision{Resolved: string(p)}, nil
}

func TestRevisionProviderRouter(t *testing.T) {
	router := revisionProviderRouter{
		github: namedRevisionProvider("github"),
		git:    namedRevisionProvider("git"),
	}
	for _, test := range []struct {
		reference string
		want      string
	}{
		{reference: "owner/package", want: "github"},
		{reference: "https://github.com/owner/package", want: "github"},
		{reference: "https://gitlab.com/owner/package", want: "git"},
	} {
		dep, err := manifest.ParseDependency(test.reference)
		if err != nil {
			t.Fatal(err)
		}
		got, err := router.Resolve(context.Background(), dep, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.Resolved != test.want {
			t.Errorf("Resolve(%q) used %q, want %q", test.reference, got.Resolved, test.want)
		}
	}
}
