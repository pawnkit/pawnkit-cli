package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawnkit-core/source"
)

func TestNativeRuntimeSelectionDefaultsOpenMP(t *testing.T) {
	project := loadNativeProject(t, `{"entry":"main.pwn","output":"main.amx","preset":"openmp"}`)
	version, port, err := nativeRuntimeSelection(project)
	if err != nil {
		t.Fatal(err)
	}
	if version != defaultRuntimeVersion || port != 0 {
		t.Fatalf("runtime = %s:%d", version, port)
	}
}

func TestNativeRuntimeSelectionRejectsPlugins(t *testing.T) {
	project := loadNativeProject(t, `{
		"entry":"main.pwn",
		"output":"main.amx",
		"preset":"openmp",
		"runtime":{"version":"1.5.8.3079","plugins":["streamer"]}
	}`)
	if _, _, err := nativeRuntimeSelection(project); err == nil {
		t.Fatal("runtime plugins accepted")
	}
}

func TestNativeRuntimeSelectionRejectsSAMP(t *testing.T) {
	project := loadNativeProject(t, `{"entry":"main.pwn","output":"main.amx","preset":"samp"}`)
	if _, _, err := nativeRuntimeSelection(project); err == nil {
		t.Fatal("SA-MP native run accepted")
	}
}

func loadNativeProject(t *testing.T, manifest string) *projectmodel.Project {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pawn.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.pwn"), []byte("main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, root, projectmodel.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return project
}
