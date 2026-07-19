package sbom_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawnkit-cli/pkg/sbom"
)

func TestGenerateDeterministicFormats(t *testing.T) {
	lock := &lockfile.Lock{Packages: []lockfile.Package{
		{Name: "z/package", Commit: "1234567", Source: lockfile.PackageSource{Type: "git", URL: "https://example.test/z"}, Kind: lockfile.KindDependency},
		{Name: "a/package", Version: "1.0.0", Commit: "7654321", Source: lockfile.PackageSource{Type: "git", URL: "https://example.test/a"}, Kind: lockfile.KindDependency},
	}}
	for _, format := range []string{"cyclonedx", "spdx"} {
		first, err := sbom.Generate(format, lock)
		if err != nil {
			t.Fatal(err)
		}
		second, _ := sbom.Generate(format, lock)
		firstJSON, _ := json.Marshal(first)
		secondJSON, _ := json.Marshal(second)
		if string(firstJSON) != string(secondJSON) {
			t.Fatalf("%s output is not deterministic", format)
		}
	}
}

func TestGenerateRedactsSourceCredentials(t *testing.T) {
	lock := &lockfile.Lock{Packages: []lockfile.Package{{
		Name: "owner/package", Commit: "1234567", Kind: lockfile.KindDependency,
		Source: lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: "https://user:secret@example.test/repo"},
	}}}
	for _, format := range []string{"cyclonedx", "spdx"} {
		document, err := sbom.Generate(format, lock)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "user") || strings.Contains(string(encoded), "secret") {
			t.Fatalf("%s output leaked credentials: %s", format, encoded)
		}
	}
}
