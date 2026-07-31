package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
)

const revisionCommit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestGitHubRevisionProviderResolvesManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/owner/package/commits/v1":
			_, _ = fmt.Fprintf(writer, `{"sha":%q}`, revisionCommit)
		case "/repos/owner/package/contents/pawn.json":
			content := base64.StdEncoding.EncodeToString([]byte(`{
				"dependencies":["owner/child:v2"]
			}`))
			_, _ = fmt.Fprintf(writer, `{"encoding":"base64","content":%q}`, content)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	dep, err := manifest.ParseDependency("owner/package:v1")
	if err != nil {
		t.Fatal(err)
	}

	revision, err := (githubRevisionProvider{
		client: server.Client(), baseURL: server.URL,
	}).Resolve(context.Background(), dep, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if revision.Commit != revisionCommit || revision.Resolved != "v1" ||
		len(revision.Manifest.Dependencies) != 1 {
		t.Fatalf("revision = %+v", revision)
	}
}

func TestGitHubRevisionProviderReusesLockedCommit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/repos/owner/package/contents/pawn.json" &&
			request.URL.Query().Get("ref") == revisionCommit {
			content := base64.StdEncoding.EncodeToString([]byte(`{"dependencies":[]}`))
			_, _ = fmt.Fprintf(writer, `{"encoding":"base64","content":%q}`, content)
			return
		}
		t.Fatalf("unexpected request: %s", request.URL)
	}))
	t.Cleanup(server.Close)
	dep, err := manifest.ParseDependency("owner/package:v1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = (githubRevisionProvider{
		client: server.Client(), baseURL: server.URL,
	}).Resolve(context.Background(), dep, &lockfile.Package{Commit: revisionCommit})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
}
