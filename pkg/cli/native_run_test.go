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
	version, options, err := nativeRuntimeSelection(project)
	if err != nil {
		t.Fatal(err)
	}
	if version != defaultRuntimeVersion || options.Port != 0 {
		t.Fatalf("runtime = %s:%d", version, options.Port)
	}
}

func TestNativeRuntimeSelectionMapsOpenMPSettings(t *testing.T) {
	project := loadNativeProject(t, `{
		"entry":"main.pwn",
		"output":"main.amx",
		"preset":"openmp",
		"runtime":{
			"version":"1.5.8.3079",
			"hostname":"Test server",
			"port":7788,
			"announce":false,
			"query":false,
			"rcon_password":"secret",
			"maxplayers":100,
			"sleep":"4.5",
			"gamemodetext":"Test mode"
		}
	}`)
	version, options, err := nativeRuntimeSelection(project)
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.5.8.3079" || options.Name != "Test server" || options.Port != 7788 ||
		options.Announce == nil || *options.Announce || options.EnableQuery == nil || *options.EnableQuery ||
		options.RCONPassword != "secret" || options.MaxPlayers != 100 || options.Sleep != 4.5 ||
		options.GameMode != "Test mode" {
		t.Fatalf("runtime = %s %+v", version, options)
	}
}

func TestRuntimeSleepRejectsInvalidValue(t *testing.T) {
	if _, err := runtimeSleep("soon"); err == nil {
		t.Fatal("invalid sleep accepted")
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
