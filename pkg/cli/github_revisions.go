package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pawnkit/pawn-project/dependency"
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"
)

const maxGitHubManifestResponse = 12 << 20

type githubRevisionProvider struct {
	client  *http.Client
	baseURL string
	token   string
}

func (p githubRevisionProvider) Resolve(
	ctx context.Context,
	dep manifest.Dependency,
	locked *lockfile.Package,
) (dependency.Revision, error) {
	commit := ""
	if locked != nil {
		commit = locked.Commit
	}
	if commit == "" {
		ref := dep.Ref
		if ref == "" {
			ref = "HEAD"
		}
		var response struct {
			SHA string `json:"sha"`
		}
		endpoint := p.endpoint(dep, "/commits/"+url.PathEscape(ref))
		if err := p.getJSON(ctx, endpoint, &response); err != nil {
			return dependency.Revision{}, err
		}
		commit = response.SHA
	}

	packageManifest, err := p.manifest(ctx, dep, commit)
	if err != nil {
		return dependency.Revision{}, err
	}
	resolved := "HEAD"
	if dep.Ref != "" {
		resolved = dep.Ref
		if dep.RefKind == manifest.RefCommit && len(commit) >= 8 {
			resolved = commit[:8]
		}
	}
	return dependency.Revision{
		Commit: commit, Resolved: resolved,
		SourceURL: "https://github.com/" + dep.Name(),
		Manifest:  *packageManifest,
	}, nil
}

func (p githubRevisionProvider) manifest(
	ctx context.Context,
	dep manifest.Dependency,
	commit string,
) (*manifest.Manifest, error) {
	for _, name := range []string{"pawn.json", "pawn.yaml", "pawn.yml"} {
		var response struct {
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		endpoint := p.endpoint(dep, "/contents/"+name) + "?ref=" + url.QueryEscape(commit)
		err := p.getJSON(ctx, endpoint, &response)
		if errors.Is(err, errGitHubNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if response.Encoding != "base64" {
			return nil, fmt.Errorf("GitHub manifest %s uses unsupported encoding %q", name, response.Encoding)
		}
		content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(response.Content, "\n", ""))
		if err != nil {
			return nil, fmt.Errorf("decoding GitHub manifest %s: %w", name, err)
		}
		return loadRemoteManifest(name, content)
	}
	return nil, errors.New("GitHub package has no pawn.json, pawn.yaml, or pawn.yml")
}

var errGitHubNotFound = errors.New("GitHub object not found")

func (p githubRevisionProvider) endpoint(dep manifest.Dependency, suffix string) string {
	base := p.baseURL
	if base == "" {
		base = "https://api.github.com"
	}
	return strings.TrimSuffix(base, "/") + "/repos/" +
		url.PathEscape(dep.User) + "/" + url.PathEscape(dep.Repo) + suffix
}

func (p githubRevisionProvider) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("creating GitHub request: %w", err)
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
		return fmt.Errorf("requesting GitHub object: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return errGitHubNotFound
	}
	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("GitHub lookup returned %s; set GITHUB_TOKEN or GH_TOKEN", response.Status)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GitHub lookup returned %s", response.Status)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxGitHubManifestResponse+1))
	if err != nil {
		return fmt.Errorf("reading GitHub response: %w", err)
	}
	if len(content) > maxGitHubManifestResponse {
		return errors.New("GitHub response exceeds 12 MiB")
	}
	if err := json.Unmarshal(content, target); err != nil {
		return fmt.Errorf("decoding GitHub response: %w", err)
	}
	return nil
}

func loadRemoteManifest(name string, content []byte) (*manifest.Manifest, error) {
	mem := fsx.NewMem()
	path := "/package/" + name
	mem.AddFile(path, content)
	result, err := manifest.Load(source.NewRegistry(), mem, path)
	if err != nil {
		return nil, err
	}
	var messages []string
	for _, item := range result.Diagnostics {
		if item.Severity == diagnostic.SeverityError {
			messages = append(messages, item.Message)
		}
	}
	if len(messages) != 0 || result.Manifest == nil {
		return nil, fmt.Errorf("invalid package manifest: %s", strings.Join(messages, "; "))
	}
	return result.Manifest, nil
}
