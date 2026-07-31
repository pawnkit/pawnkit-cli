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
			_, _ = fmt.Fprintf(writer, `{"encoding":"base64","content":%q,"html_url":"https://github.com/canonical/package/blob/%s/pawn.json"}`, content, revisionCommit)
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
		revision.CanonicalName != "canonical/package" ||
		revision.SourceURL != "https://github.com/canonical/package" ||
		len(revision.Manifest.Dependencies) != 1 {
		t.Fatalf("revision = %+v", revision)
	}
}

func TestGitHubRevisionProviderSelectsTagRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/owner/package/tags":
			_, _ = fmt.Fprint(writer, `[{"name":"v1.2.0"},{"name":"v1.4.0"},{"name":"v2.0.0"}]`)
		case "/repos/owner/package/commits/v1.4.0":
			_, _ = fmt.Fprintf(writer, `{"sha":%q}`, revisionCommit)
		case "/repos/owner/package/contents/pawn.json":
			content := base64.StdEncoding.EncodeToString([]byte(`{"dependencies":[]}`))
			_, _ = fmt.Fprintf(writer, `{"encoding":"base64","content":%q}`, content)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	dep, err := manifest.ParseDependency("owner/package:^1.2.0")
	if err != nil {
		t.Fatal(err)
	}

	revision, err := (githubRevisionProvider{
		client: server.Client(), baseURL: server.URL,
	}).Resolve(context.Background(), dep, nil)
	if err != nil {
		t.Fatal(err)
	}
	if revision.Resolved != "v1.4.0" {
		t.Fatalf("resolved = %q", revision.Resolved)
	}
}

func TestGitHubRevisionProviderAllowsLeafPackage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/repos/owner/package/commits/HEAD" {
			_, _ = fmt.Fprintf(writer, `{"sha":%q}`, revisionCommit)
			return
		}
		http.NotFound(writer, request)
	}))
	t.Cleanup(server.Close)
	dep, err := manifest.ParseDependency("owner/package")
	if err != nil {
		t.Fatal(err)
	}

	revision, err := (githubRevisionProvider{
		client: server.Client(), baseURL: server.URL,
	}).Resolve(context.Background(), dep, nil)
	if err != nil {
		t.Fatal(err)
	}
	if revision.CanonicalName != "owner/package" || len(revision.Manifest.Dependencies) != 0 {
		t.Fatalf("revision = %+v", revision)
	}
}

func TestCanonicalGitHubName(t *testing.T) {
	tests := map[string]string{
		"https://github.com/owner/package/blob/main/pawn.json": "owner/package",
		"https://example.com/owner/package":                    "",
		"not a URL":                                            "",
	}
	for raw, want := range tests {
		if got := canonicalGitHubName(raw); got != want {
			t.Errorf("canonicalGitHubName(%q) = %q, want %q", raw, got, want)
		}
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

	revision, err := (githubRevisionProvider{
		client: server.Client(), baseURL: server.URL,
	}).Resolve(context.Background(), dep, &lockfile.Package{Commit: revisionCommit, Resolved: "v1.0.0"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if revision.Resolved != "v1.0.0" {
		t.Fatalf("resolved = %q", revision.Resolved)
	}
}
