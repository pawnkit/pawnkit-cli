package audit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawnkit-cli/pkg/audit"
	"github.com/pawnkit/pawnkit-core/source"
)

func TestInspectRedactsSourceCredentials(t *testing.T) {
	filesystem := fsx.NewMem()
	filesystem.AddFile("/project/pawn.json", []byte(`{"entry":"main.pwn","experimental":{"build_file":false}}`))
	filesystem.AddFile("/project/main.pwn", nil)
	filesystem.AddFile("/project/pawn.lock", []byte(`{"schemaVersion":1,"packages":[{"name":"owner/package","resolved":"v1","commit":"1234567","source":{"type":"git","url":"https://user:password@example.test/repo"},"kind":"dependency"}]}`))
	project, err := projectmodel.Load(source.NewRegistry(), filesystem, "/project", projectmodel.Options{})
	if err != nil {
		t.Fatal(err)
	}
	report := audit.Inspect(project, audit.Options{Offline: true, Source: "pawn.lock"})
	if len(report.Findings) != 3 || !strings.Contains(report.Disclaimer, "does not mean") {
		t.Fatalf("report = %+v", report)
	}
	for _, finding := range report.Findings {
		if strings.Contains(finding.Message, "password") {
			t.Fatalf("credential leaked: %+v", finding)
		}
	}
}

func TestInspectRejectsArtifactOutsideProject(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "plugin.so")
	if err := os.WriteFile(outside, []byte("not an artifact"), 0o644); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(dir, outside)
	if err != nil {
		t.Fatal(err)
	}
	filesystem := fsx.NewMem()
	filesystem.AddFile(filepath.Join(dir, "pawn.json"), []byte(`{"entry":"main.pwn","experimental":{"build_file":false}}`))
	filesystem.AddFile(filepath.Join(dir, "main.pwn"), nil)
	filesystem.AddFile(filepath.Join(dir, "pawn.lock"), []byte(`{"schemaVersion":1,"packages":[{"name":"owner/package","resolved":"v1","commit":"1234567","source":{"type":"git","url":"https://example.test/repo"},"kind":"plugin","platformArtifacts":[{"platform":"linux-x86_64","path":"`+filepath.ToSlash(relative)+`"}]}]}`))
	project, err := projectmodel.Load(source.NewRegistry(), filesystem, dir, projectmodel.Options{})
	if err != nil {
		t.Fatal(err)
	}
	report := audit.Inspect(project, audit.Options{Offline: true, Source: "pawn.lock"})
	found := false
	for _, finding := range report.Findings {
		if strings.HasPrefix(finding.ID, "artifact-path-invalid/") {
			found = true
		}
	}
	if !found {
		t.Fatalf("report = %+v", report)
	}
}
