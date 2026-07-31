package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/dependency"
	"github.com/pawnkit/pawn-project/lockfile"
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

type noopSourceInstaller struct{}

func (noopSourceInstaller) Install(
	context.Context,
	lockfile.Package,
	string,
) (dependency.Status, error) {
	return dependency.StatusInstalled, nil
}

type fixedReleaseProvider struct {
	assets []dependency.ReleaseAsset
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
