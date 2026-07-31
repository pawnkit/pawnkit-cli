package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pawnkit/pawn-project/dependency"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
)

func TestInstallResolvesLocksAndInstallsResources(t *testing.T) {
	root := t.TempDir()
	writeInstallTestFile(t, root, "pawn.json", `{"entry":"main.pwn"}`)
	writeInstallTestFile(t, root, "main.pwn", "main() {}\n")
	writeInstallTestFile(t, root, "vendor/plugin/pawn.json", `{
		"resources":[{
			"name":"^plugin.so$",
			"platform":"linux"
		}]
	}`)
	writeInstallTestFile(t, root, "pawn.lock", `{
		"version":1,
		"generated":"2026-07-31T00:00:00Z",
		"sampctl_version":"1.14.1",
		"dependencies":{
			"plugin://owner/plugin":{
				"constraint":"plugin://owner/plugin:v1",
				"resolved":"v1",
				"commit":"abcdef0",
				"user":"owner",
				"repo":"plugin",
				"scheme":"plugin",
				"transitive":true,
				"local":"vendor/plugin"
			}
		}
	}`)
	content := []byte("plugin")
	asset := dependency.ReleaseAsset{
		Name: "plugin.so", URL: "https://example.com/plugin.so",
		Size: int64(len(content)),
	}
	downloader := &installCountingDownloader{content: content}
	var stdout, stderr bytes.Buffer
	code := runInstallWith(
		context.Background(),
		[]string{
			"--project", root,
			"--target", "linux-amd64",
			"--format", "json",
		},
		&stdout,
		&stderr,
		installServices{
			sourceInstaller: noopSourceInstaller{},
			downloader:      downloader,
			provider:        fixedReleaseProvider{assets: []dependency.ReleaseAsset{asset}},
			writeLock:       replaceLockfile,
		},
	)
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	installed, err := os.ReadFile(filepath.Join(root, "plugins", "plugin.so"))
	if err != nil {
		t.Fatalf("ReadFile plugin: %v", err)
	}
	if string(installed) != "plugin" {
		t.Fatalf("plugin = %q", installed)
	}
	lockContent, err := os.ReadFile(filepath.Join(root, "pawn.lock"))
	if err != nil {
		t.Fatalf("ReadFile lock: %v", err)
	}
	if !bytes.Contains(lockContent, []byte(`"schema_version": 1`)) ||
		!bytes.Contains(lockContent, []byte(`"linux-amd64"`)) {
		t.Fatalf("lockfile = %s", lockContent)
	}
	if !strings.Contains(stdout.String(), `"resources"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if downloader.calls != 1 {
		t.Fatalf("download calls = %d, want 1", downloader.calls)
	}

	stdout.Reset()
	stderr.Reset()
	code = runInstallWith(
		context.Background(),
		[]string{"--project", root, "--target", "linux-amd64"},
		&stdout,
		&stderr,
		installServices{
			sourceInstaller: noopSourceInstaller{},
			downloader:      staticResourceDownloader{content: content},
			provider:        failingReleaseProvider{},
			writeLock: func(string, []byte) error {
				return errors.New("unexpected lock write")
			},
		},
	)
	if code != ExitOK {
		t.Fatalf("repeat code = %d, stderr = %s", code, stderr.String())
	}
}

func TestReplaceLockfilePreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pawn.lock")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := replaceLockfile(path, []byte("new")); err != nil {
		t.Fatalf("replaceLockfile: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("content = %q", content)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o640 {
			t.Fatalf("mode = %o", info.Mode().Perm())
		}
	}
}

func TestInstallCreatesMissingLock(t *testing.T) {
	root := t.TempDir()
	writeInstallTestFile(t, root, "pawn.json", `{
		"entry":"main.pwn",
		"dependencies":["owner/package:v1"]
	}`)
	writeInstallTestFile(t, root, "main.pwn", "main() {}\n")
	var stdout, stderr bytes.Buffer
	code := runInstallWith(
		context.Background(),
		[]string{"--project", root, "--target", "linux-amd64"},
		&stdout,
		&stderr,
		installServices{
			sourceInstaller:  installingSourceInstaller{},
			downloader:       staticResourceDownloader{content: []byte("unused")},
			provider:         fixedReleaseProvider{},
			revisionProvider: fixedRevisionProvider{},
			writeLock:        replaceLockfile,
			now: func() time.Time {
				return time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
			},
		},
	)
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	content, err := os.ReadFile(filepath.Join(root, "pawn.lock"))
	if err != nil {
		t.Fatalf("ReadFile lock: %v", err)
	}
	if !bytes.Contains(content, []byte(`"github.com/owner/package"`)) ||
		!bytes.Contains(content, []byte(`"sampctl_version": "1.14.1"`)) {
		t.Fatalf("lockfile = %s", content)
	}
}

func TestInstallReconcilesChangedLock(t *testing.T) {
	root := t.TempDir()
	writeInstallTestFile(t, root, "pawn.json", `{
		"entry":"main.pwn",
		"dependencies":["owner/package:v2"]
	}`)
	writeInstallTestFile(t, root, "main.pwn", "main() {}\n")
	writeInstallTestFile(t, root, "pawn.lock", `{
		"version":1,
		"generated":"2026-07-31T00:00:00Z",
		"sampctl_version":"1.14.1",
		"dependencies":{"github.com/owner/package":{
			"constraint":":v1","resolved":"v1","commit":"1111111111111111111111111111111111111111",
			"user":"owner","repo":"package"
		}}
	}`)

	var stdout, stderr bytes.Buffer
	code := runInstallWith(
		context.Background(),
		[]string{"--project", root, "--target", "linux-amd64"},
		&stdout,
		&stderr,
		installServices{
			sourceInstaller:  installingSourceInstaller{},
			downloader:       staticResourceDownloader{content: []byte("unused")},
			provider:         fixedReleaseProvider{},
			revisionProvider: fixedRevisionProvider{},
			writeLock:        replaceLockfile,
		},
	)
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	content, err := os.ReadFile(filepath.Join(root, "pawn.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte(`"constraint": ":v2"`)) ||
		bytes.Contains(content, []byte(`"constraint": ":v1"`)) {
		t.Fatalf("lockfile = %s", content)
	}
}

func TestInstallReusesMatchingLockOffline(t *testing.T) {
	root := t.TempDir()
	writeInstallTestFile(t, root, "pawn.json", `{
		"entry":"main.pwn",
		"dependencies":["owner/package:v1"]
	}`)
	writeInstallTestFile(t, root, "main.pwn", "main() {}\n")
	writeInstallTestFile(t, root, "pawn.lock", `{
		"version":1,
		"generated":"2026-07-31T00:00:00Z",
		"sampctl_version":"1.14.1",
		"dependencies":{"github.com/owner/package":{
			"constraint":":v1","resolved":"v1","commit":"1111111111111111111111111111111111111111",
			"user":"owner","repo":"package"
		}}
	}`)

	var stdout, stderr bytes.Buffer
	code := runInstallWith(
		context.Background(),
		[]string{"--project", root, "--target", "linux-amd64"},
		&stdout,
		&stderr,
		installServices{
			sourceInstaller: installingSourceInstaller{},
			downloader:      staticResourceDownloader{content: []byte("unused")},
			provider:        fixedReleaseProvider{},
			writeLock: func(string, []byte) error {
				return errors.New("unexpected lock write")
			},
		},
	)
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
}

func TestInstallUpdateRefreshesMatchingLock(t *testing.T) {
	root := t.TempDir()
	writeInstallTestFile(t, root, "pawn.json", `{
		"entry":"main.pwn",
		"dependencies":["owner/package@main"]
	}`)
	writeInstallTestFile(t, root, "main.pwn", "main() {}\n")
	writeInstallTestFile(t, root, "pawn.lock", `{
		"version":1,
		"generated":"2026-07-31T00:00:00Z",
		"sampctl_version":"1.14.1",
		"dependencies":{"github.com/owner/package":{
			"constraint":"@main","resolved":"main","commit":"1111111111111111111111111111111111111111",
			"user":"owner","repo":"package","branch":"main"
		}}
	}`)
	provider := &recordingRevisionProvider{}

	var stdout, stderr bytes.Buffer
	code := runInstallWith(
		context.Background(),
		[]string{"--project", root, "--update", "--target", "linux-amd64"},
		&stdout,
		&stderr,
		installServices{
			sourceInstaller:  installingSourceInstaller{},
			downloader:       staticResourceDownloader{content: []byte("unused")},
			provider:         fixedReleaseProvider{},
			revisionProvider: provider,
			writeLock:        replaceLockfile,
		},
	)
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if provider.receivedLocked {
		t.Fatal("update reused the locked revision")
	}
	content, err := os.ReadFile(filepath.Join(root, "pawn.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("2222222222222222222222222222222222222222")) {
		t.Fatalf("lockfile = %s", content)
	}
}

func TestReplaceLockfileCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pawn.lock")
	if err := replaceLockfile(path, []byte("new")); err != nil {
		t.Fatalf("replaceLockfile: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "new" {
		t.Fatalf("content = %q, err = %v", content, err)
	}
}

type noopSourceInstaller struct{}

func (noopSourceInstaller) Install(
	context.Context,
	lockfile.Package,
	string,
) (dependency.Status, error) {
	return dependency.StatusInstalled, nil
}

type installingSourceInstaller struct{}

func (installingSourceInstaller) Install(
	_ context.Context,
	_ lockfile.Package,
	target string,
) (dependency.Status, error) {
	if err := os.MkdirAll(target, 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(target, "pawn.json"), []byte(`{}`), 0o600); err != nil {
		return "", err
	}
	return dependency.StatusInstalled, nil
}

type fixedRevisionProvider struct{}

func (fixedRevisionProvider) Resolve(
	context.Context,
	manifest.Dependency,
	*lockfile.Package,
) (dependency.Revision, error) {
	return dependency.Revision{
		Commit:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Resolved: "v1",
	}, nil
}

type recordingRevisionProvider struct {
	receivedLocked bool
}

func (p *recordingRevisionProvider) Resolve(
	_ context.Context,
	_ manifest.Dependency,
	locked *lockfile.Package,
) (dependency.Revision, error) {
	p.receivedLocked = locked != nil
	return dependency.Revision{
		Commit:   "2222222222222222222222222222222222222222",
		Resolved: "main",
	}, nil
}

type fixedReleaseProvider struct {
	assets []dependency.ReleaseAsset
}

type failingReleaseProvider struct{}

func (failingReleaseProvider) Assets(
	context.Context,
	lockfile.Package,
) ([]dependency.ReleaseAsset, error) {
	return nil, errors.New("release lookup should not run")
}

func (p fixedReleaseProvider) Assets(
	context.Context,
	lockfile.Package,
) ([]dependency.ReleaseAsset, error) {
	return p.assets, nil
}

type installCountingDownloader struct {
	content []byte
	calls   int
}

func (d *installCountingDownloader) Download(context.Context, string) (io.ReadCloser, error) {
	d.calls++
	return io.NopCloser(bytes.NewReader(d.content)), nil
}

func writeInstallTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
