package cli

import (
	"fmt"
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

func TestNativeRuntimeSelectionMapsPluginsAndFilterscripts(t *testing.T) {
	project := loadNativeProject(t, `{
		"entry":"main.pwn",
		"output":"main.amx",
		"preset":"openmp",
		"runtime":{"version":"1.5.8.3079","plugins":["plugins/streamer.so"],"filterscripts":["admin.amx"]}
	}`)
	_, options, err := nativeRuntimeSelection(project)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(options.LegacyPlugins) != "[streamer]" || fmt.Sprint(options.SideScripts) != "[admin 1]" {
		t.Fatalf("runtime resources = %+v", options)
	}
}

func TestNativeSessionResourcesSelectsCurrentTarget(t *testing.T) {
	project := loadNativeProject(t, `{"entry":"main.pwn","output":"main.amx","preset":"openmp"}`)
	resources := map[string]string{
		"plugins/streamer.so":     "plugin",
		"components/commands.so":  "component",
		"filterscripts/admin.amx": "filterscript",
		"includes/resource.inc":   "include",
	}
	for name, contents := range resources {
		path := filepath.Join(project.Root(), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lock := fmt.Sprintf(`{
		"version":1,
		"dependencies":{"plugin://owner/package":{"constraint":"1.0.0","resolved":"1.0.0","commit":"abcdef1","user":"owner","repo":"package","scheme":"plugin"}},
		"pawnkit":{"schema_version":1,"resources":[
			{"package":"plugin://owner/package","resource":"server","target":"linux-amd64","url":"https://example.invalid/a.zip","size":1,"checksum":"%[1]s","archive":"zip","files":[
				{"source":"streamer.so","destination":"plugins/streamer.so","size":6,"checksum":"%[2]s"},
				{"source":"commands.so","destination":"components/commands.so","size":9,"checksum":"%[3]s"},
				{"source":"admin.amx","destination":"filterscripts/admin.amx","size":12,"checksum":"%[4]s"},
				{"source":"resource.inc","destination":"includes/resource.inc","size":7,"checksum":"%[5]s"}
			]},
			{"package":"plugin://owner/package","resource":"server","target":"windows-amd64","url":"https://example.invalid/a.zip","size":1,"checksum":"%[1]s","archive":"zip","files":[
				{"source":"streamer.dll","destination":"plugins/streamer.dll","size":6,"checksum":"%[2]s"}
			]}
		]}
	}`, checksum([]byte("archive")), checksum([]byte("plugin")), checksum([]byte("component")), checksum([]byte("filterscript")), checksum([]byte("include")))
	if err := os.WriteFile(filepath.Join(project.Root(), "pawn.lock"), []byte(lock), 0o600); err != nil {
		t.Fatal(err)
	}
	project = loadNativeProjectAt(t, project.Root())
	files, plugins, scripts, err := nativeSessionResources(project, "linux-amd64")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || fmt.Sprint(plugins) != "[streamer]" || fmt.Sprint(scripts) != "[admin 1]" {
		t.Fatalf("resources = %+v, %v, %v", files, plugins, scripts)
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
	return loadNativeProjectAt(t, root)
}

func loadNativeProjectAt(t *testing.T, root string) *projectmodel.Project {
	t.Helper()
	project, err := projectmodel.Load(source.NewRegistry(), fsx.OS{}, root, projectmodel.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return project
}
