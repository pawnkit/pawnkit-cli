package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pawnkit/pawn-project/dependency"
	"github.com/pawnkit/pawn-project/lockfile"
)

const maxGitHubReleaseResponse = 2 << 20

func githubToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GH_TOKEN")
}

type githubReleaseProvider struct {
	client  *http.Client
	baseURL string
	token   string
}

func (p githubReleaseProvider) Assets(
	ctx context.Context,
	pkg lockfile.Package,
) ([]dependency.ReleaseAsset, error) {
	owner, repo, err := githubRepository(pkg.Source.URL)
	if err != nil {
		return nil, err
	}
	if pkg.Resolved == "" || pkg.Resolved == "HEAD" {
		return nil, errors.New("GitHub release resolution requires a locked tag")
	}
	baseURL := p.baseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	endpoint := strings.TrimSuffix(baseURL, "/") +
		"/repos/" + url.PathEscape(owner) +
		"/" + url.PathEscape(repo) +
		"/releases/tags/" + url.PathEscape(pkg.Resolved)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating GitHub release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "pawnkit-cli")
	if p.token != "" {
		request.Header.Set("Authorization", "Bearer "+p.token)
	}

	client := p.client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("requesting GitHub release: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == http.StatusForbidden ||
			response.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf(
				"GitHub release lookup returned %s; set GITHUB_TOKEN or wait for the rate limit to reset",
				response.Status,
			)
		}
		return nil, fmt.Errorf("GitHub release lookup returned %s", response.Status)
	}

	var release struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxGitHubReleaseResponse+1))
	if err != nil {
		return nil, fmt.Errorf("reading GitHub release: %w", err)
	}
	if len(content) > maxGitHubReleaseResponse {
		return nil, errors.New("GitHub release response exceeds 2 MiB")
	}
	if err := json.Unmarshal(content, &release); err != nil {
		return nil, fmt.Errorf("decoding GitHub release: %w", err)
	}
	assets := make([]dependency.ReleaseAsset, len(release.Assets))
	for i, asset := range release.Assets {
		assets[i] = dependency.ReleaseAsset{
			Name: asset.Name,
			URL:  asset.URL,
			Size: asset.Size,
		}
	}
	return assets, nil
}

func githubRepository(rawURL string) (string, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil ||
		parsed.Scheme != "https" ||
		!strings.EqualFold(parsed.Host, "github.com") ||
		parsed.User != nil {
		return "", "", fmt.Errorf("unsupported GitHub package URL %q", rawURL)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unsupported GitHub package URL %q", rawURL)
	}
	repo := strings.TrimSuffix(parts[1], ".git")
	if repo == "" {
		return "", "", fmt.Errorf("unsupported GitHub package URL %q", rawURL)
	}
	return parts[0], repo, nil
}
