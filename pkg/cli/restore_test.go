package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawnkit-core/hash"
)

func TestRestoreLockedResources(t *testing.T) {
	root := t.TempDir()
	content := []byte("plugin")
	resource := lockfile.ResolvedResource{
		Package: "owner/plugin", Resource: "plugin", Target: "linux-amd64",
		URL: "https://example.com/plugin", Size: int64(len(content)),
		Checksum: hash.Content(content), Archive: "file",
		Files: []lockfile.ResolvedResourceFile{{
			Source: "plugin.so", Destination: "plugins/plugin.so",
			Size: int64(len(content)), Checksum: hash.Content(content),
		}},
	}

	results, err := restoreLockedResources(
		context.Background(),
		root,
		"linux-amd64",
		&lockfile.Lock{Resources: []lockfile.ResolvedResource{resource}},
		staticResourceDownloader{content: content},
	)
	if err != nil {
		t.Fatalf("restoreLockedResources: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	installed, err := os.ReadFile(filepath.Join(root, "plugins", "plugin.so"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(installed, content) {
		t.Fatalf("installed = %q, want %q", installed, content)
	}
}

func TestRestoreLockedResourcesSkipsEmptyExtension(t *testing.T) {
	results, err := restoreLockedResources(
		context.Background(),
		t.TempDir(),
		"linux-amd64",
		&lockfile.Lock{},
		nil,
	)
	if err != nil || results != nil {
		t.Fatalf("results = %v, error = %v", results, err)
	}
}

type staticResourceDownloader struct {
	content []byte
}

func (d staticResourceDownloader) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(d.content)), nil
}
