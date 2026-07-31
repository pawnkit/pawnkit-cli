package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/lockfile"
)

func TestGitHubReleaseProviderListsLockedTagAssets(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/owner/repo/releases/tags/v1.2.3" {
			t.Errorf("path = %q", request.URL.Path)
		}
		authorization = request.Header.Get("Authorization")
		_, _ = writer.Write([]byte(`{"assets":[{
			"name":"plugin.zip",
			"browser_download_url":"https://example.com/plugin.zip",
			"size":42
		}]}`))
	}))
	defer server.Close()
	provider := githubReleaseProvider{
		client: server.Client(), baseURL: server.URL, token: "token",
	}
	pkg := lockfile.Package{
		Resolved: "v1.2.3",
		Source: lockfile.PackageSource{
			URL: "https://github.com/owner/repo.git",
		},
	}

	assets, err := provider.Assets(context.Background(), pkg)
	if err != nil {
		t.Fatalf("Assets: %v", err)
	}
	if len(assets) != 1 || assets[0].Name != "plugin.zip" || assets[0].Size != 42 {
		t.Fatalf("assets = %+v", assets)
	}
	if authorization != "Bearer token" {
		t.Fatalf("Authorization = %q", authorization)
	}
}

func TestGitHubReleaseProviderReportsRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	provider := githubReleaseProvider{client: server.Client(), baseURL: server.URL}
	pkg := lockfile.Package{
		Resolved: "v1",
		Source:   lockfile.PackageSource{URL: "https://github.com/owner/repo"},
	}

	_, err := provider.Assets(context.Background(), pkg)
	if err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("Assets error = %v", err)
	}
}

func TestGitHubRepositoryRejectsOtherHosts(t *testing.T) {
	if _, _, err := githubRepository("https://gitlab.com/owner/repo"); err == nil {
		t.Fatal("githubRepository succeeded")
	}
}
