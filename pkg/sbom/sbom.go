// Package sbom creates deterministic dependency inventories.
package sbom

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/pawnkit/pawn-project/lockfile"
)

func Generate(format string, lock *lockfile.Lock) (any, error) {
	if lock == nil {
		return nil, errors.New("sbom: pawn.lock is required")
	}
	packages := append([]lockfile.Package(nil), lock.Packages...)
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })
	switch format {
	case "cyclonedx":
		return cyclonedx(packages), nil
	case "spdx":
		return spdx(lock, packages), nil
	default:
		return nil, errors.New("sbom: format must be cyclonedx or spdx")
	}
}

type cdxDocument struct {
	BomFormat   string         `json:"bomFormat"`
	SpecVersion string         `json:"specVersion"`
	Version     int            `json:"version"`
	Components  []cdxComponent `json:"components"`
}

type cdxComponent struct {
	Type       string    `json:"type"`
	Reference  string    `json:"bom-ref"`
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	Hashes     []cdxHash `json:"hashes,omitempty"`
	Properties []cdxProp `json:"properties,omitempty"`
}

type cdxHash struct {
	Algorithm string `json:"alg"`
	Content   string `json:"content"`
}
type cdxProp struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func cyclonedx(packages []lockfile.Package) cdxDocument {
	document := cdxDocument{BomFormat: "CycloneDX", SpecVersion: "1.5", Version: 1}
	for _, item := range packages {
		component := cdxComponent{Type: "library", Reference: item.Name + "@" + item.Commit, Name: item.Name, Version: packageVersion(item)}
		if checksum := checksumValue(item.Checksum); checksum != "" {
			component.Hashes = []cdxHash{{Algorithm: "SHA-256", Content: checksum}}
		}
		component.Properties = []cdxProp{{Name: "pawnkit:source", Value: publicSourceURL(item.Source.URL)}, {Name: "pawnkit:kind", Value: item.Kind}}
		document.Components = append(document.Components, component)
	}
	return document
}

type spdxDocument struct {
	Version      string        `json:"spdxVersion"`
	DataLicense  string        `json:"dataLicense"`
	ID           string        `json:"SPDXID"`
	Name         string        `json:"name"`
	Namespace    string        `json:"documentNamespace"`
	CreationInfo spdxCreation  `json:"creationInfo"`
	Packages     []spdxPackage `json:"packages"`
}

type spdxCreation struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}
type spdxPackage struct {
	Name             string         `json:"name"`
	ID               string         `json:"SPDXID"`
	Version          string         `json:"versionInfo"`
	DownloadLocation string         `json:"downloadLocation"`
	LicenseConcluded string         `json:"licenseConcluded"`
	LicenseDeclared  string         `json:"licenseDeclared"`
	Checksums        []spdxChecksum `json:"checksums,omitempty"`
}
type spdxChecksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"checksumValue"`
}

func spdx(lock *lockfile.Lock, packages []lockfile.Package) spdxDocument {
	encoded, _ := json.Marshal(lock)
	sum := sha256.Sum256(encoded)
	created := lock.GeneratedAt
	if created == "" {
		created = "1970-01-01T00:00:00Z"
	}
	document := spdxDocument{
		Version: "SPDX-2.3", DataLicense: "CC0-1.0", ID: "SPDXRef-DOCUMENT", Name: "Pawn project dependencies",
		Namespace:    "https://pawnkit.dev/spdx/" + hex.EncodeToString(sum[:]),
		CreationInfo: spdxCreation{Created: created, Creators: []string{"Tool: pawn"}},
	}
	for index, item := range packages {
		entry := spdxPackage{
			Name: item.Name, ID: fmt.Sprintf("SPDXRef-Package-%d", index+1), Version: packageVersion(item),
			DownloadLocation: publicSourceURL(item.Source.URL), LicenseConcluded: "NOASSERTION", LicenseDeclared: "NOASSERTION",
		}
		if entry.DownloadLocation == "" {
			entry.DownloadLocation = "NOASSERTION"
		}
		if checksum := checksumValue(item.Checksum); checksum != "" {
			entry.Checksums = []spdxChecksum{{Algorithm: "SHA256", Value: checksum}}
		}
		document.Packages = append(document.Packages, entry)
	}
	return document
}

func packageVersion(item lockfile.Package) string {
	if item.Version != "" {
		return item.Version
	}
	return item.Commit
}

func checksumValue(value string) string {
	return strings.TrimPrefix(value, "sha256:")
}

func publicSourceURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil {
		return value
	}
	parsed.User = nil
	return parsed.String()
}
