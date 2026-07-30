package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pawnkit/pawnserver/runtimeartifact"
)

func TestRuntimeInstallUsesVerifiedIndexAndCache(t *testing.T) {
	archive := []byte("reviewed archive")
	executableSum := "sha256:" + strings.Repeat("1", 64)
	index := []byte(`{
		"schemaVersion":1,
		"id":"test",
		"generatedAt":"2026-07-30T00:00:00Z",
		"artifacts":[{
			"vendor":"openmultiplayer",
			"version":"1.5.8.3079",
			"profile":"openmp",
			"target":"linux-amd64",
			"source":{"repository":"https://github.com/openmultiplayer/open.mp","tag":"v1.5.8.3079","commit":"c6759bd8d265171ae3d86598895a23d5a8d92a3b"},
			"archive":{"url":"https://example.test/runtime.tar.gz","format":"tar.gz","size":16,"checksum":"` + checksum(archive) + `"},
			"root":"Server",
			"executable":{"path":"Server/omp-server","architecture":"386","checksum":"` + executableSum + `"}
		}]
	}`)
	cache := filepath.Join(t.TempDir(), "cache")
	var installed string
	var stdout, stderr bytes.Buffer
	code := runRuntimeWith(context.Background(), []string{
		"install", "--target", "linux-amd64",
	}, &stdout, &stderr, runtimeInstaller{
		indexURL:      "https://example.test/index.json",
		indexChecksum: checksum(index),
		downloader: compilerDownloads{
			"https://example.test/index.json":     index,
			"https://example.test/runtime.tar.gz": archive,
		},
		cacheDir: func() (string, error) { return cache, nil },
		install: func(_ runtimeartifact.Artifact, reader io.Reader, destination string) error {
			if got, err := io.ReadAll(reader); err != nil || !bytes.Equal(got, archive) {
				t.Fatalf("archive = %q, %v", got, err)
			}
			installed = destination
			return nil
		},
	})
	if code != ExitOK {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	wantDir := filepath.Join(cache, "openmultiplayer", "1.5.8.3079", "linux-amd64")
	if installed != wantDir {
		t.Fatalf("destination = %q, want %q", installed, wantDir)
	}
	if want := filepath.Join(wantDir, "omp-server") + "\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRuntimeRequiresInstallSubcommand(t *testing.T) {
	var stderr bytes.Buffer
	if code := runRuntimeWith(context.Background(), nil, io.Discard, &stderr, runtimeInstaller{}); code != ExitUsage {
		t.Fatalf("code = %d", code)
	}
}

func checksum(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}
