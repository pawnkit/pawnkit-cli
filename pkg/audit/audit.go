// Package audit inspects resolved project supply-chain metadata.
package audit

import (
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pawnkit/pawn-project/lockfile"
	projectmodel "github.com/pawnkit/pawn-project/project"
)

type Finding struct {
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	Certainty string `json:"certainty"`
	Message   string `json:"message"`
	Package   string `json:"package,omitempty"`
}

type Metadata struct {
	Offline   bool   `json:"offline"`
	Source    string `json:"source"`
	Timestamp string `json:"timestamp,omitempty"`
}

type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	Metadata      Metadata  `json:"metadata"`
	Disclaimer    string    `json:"disclaimer"`
	Findings      []Finding `json:"findings"`
}

type Options struct {
	Offline   bool
	Source    string
	Timestamp time.Time
}

func Inspect(project *projectmodel.Project, opts Options) Report {
	metadata := Metadata{Offline: opts.Offline, Source: opts.Source}
	if !opts.Timestamp.IsZero() {
		metadata.Timestamp = opts.Timestamp.UTC().Format(time.RFC3339)
	}
	report := Report{
		SchemaVersion: 1, Metadata: metadata,
		Disclaimer: "No advisory data was checked. This report does not mean the packages are safe.",
	}
	for _, diagnostic := range project.Diagnostics() {
		report.Findings = append(report.Findings, Finding{
			ID: diagnostic.Code, Severity: diagnostic.Severity.String(), Certainty: "confirmed",
			Message: diagnostic.Message,
		})
	}
	if project.Lockfile() == nil {
		report.Findings = append(report.Findings, Finding{
			ID: "lockfile-missing", Severity: "warning", Certainty: "confirmed",
			Message: "dependency integrity cannot be audited without pawn.lock",
		})
		return report
	}
	for _, dependency := range project.Lockfile().Packages {
		report.Findings = append(report.Findings, inspectPackage(dependency)...)
		report.Findings = append(report.Findings, inspectLocalArtifacts(project.Root(), dependency)...)
	}
	if compiler := project.Lockfile().Compiler; compiler != nil && compiler.Checksum == "" {
		report.Findings = append(report.Findings, Finding{ID: "compiler-checksum-missing", Severity: "warning", Certainty: "confirmed", Message: "resolved compiler has no checksum"})
	}
	return report
}

func inspectLocalArtifacts(root string, dependency lockfile.Package) []Finding {
	var findings []Finding
	for _, artifact := range dependency.PlatformArtifacts {
		if artifact.Path == "" {
			continue
		}
		path, err := localArtifactPath(root, artifact.Path)
		if err != nil {
			findings = append(findings, Finding{ID: "artifact-path-invalid/" + artifact.Platform, Severity: "error", Certainty: "confirmed", Package: dependency.Name, Message: "artifact path escapes the project"})
			continue
		}
		file, err := os.Open(path) //nolint:gosec // The path is contained within the project.
		if err != nil {
			findings = append(findings, Finding{ID: "artifact-missing/" + artifact.Platform, Severity: "error", Certainty: "confirmed", Package: dependency.Name, Message: "declared artifact is missing"})
			continue
		}
		if artifact.Checksum != "" {
			hash := sha256.New()
			_, copyErr := io.Copy(hash, file)
			if copyErr != nil {
				findings = append(findings, Finding{ID: "artifact-read-failed/" + artifact.Platform, Severity: "error", Certainty: "confirmed", Package: dependency.Name, Message: "artifact could not be read"})
			} else if got := "sha256:" + hex.EncodeToString(hash.Sum(nil)); got != artifact.Checksum {
				findings = append(findings, Finding{ID: "artifact-checksum-drift/" + artifact.Platform, Severity: "error", Certainty: "confirmed", Package: dependency.Name, Message: "artifact differs from its declared checksum"})
			}
		}
		_ = file.Close()
		if actual := binaryPlatform(path); actual != "" && actual != artifact.Platform {
			findings = append(findings, Finding{ID: "artifact-architecture/" + artifact.Platform, Severity: "error", Certainty: "confirmed", Package: dependency.Name, Message: fmt.Sprintf("artifact is %s, declared as %s", actual, artifact.Platform)})
		}
	}
	return findings
}

func localArtifactPath(root, value string) (string, error) {
	if filepath.IsAbs(filepath.FromSlash(value)) {
		return "", errors.New("artifact path is absolute")
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, filepath.FromSlash(value))
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes project")
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if errors.Is(err, os.ErrNotExist) {
		return candidate, nil
	}
	if err != nil {
		return "", err
	}
	relative, err = filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("symlink escapes project")
	}
	return resolved, nil
}

func binaryPlatform(path string) string {
	if file, err := elf.Open(path); err == nil {
		defer func() { _ = file.Close() }()
		arch := map[elf.Machine]string{elf.EM_386: "x86", elf.EM_X86_64: "x86_64", elf.EM_ARM: "arm", elf.EM_AARCH64: "arm64"}[file.Machine]
		if arch != "" {
			return "linux-" + arch
		}
	}
	if file, err := pe.Open(path); err == nil {
		defer func() { _ = file.Close() }()
		arch := map[uint16]string{pe.IMAGE_FILE_MACHINE_I386: "x86", pe.IMAGE_FILE_MACHINE_AMD64: "x86_64", pe.IMAGE_FILE_MACHINE_ARM64: "arm64"}[file.Machine]
		if arch != "" {
			return "windows-" + arch
		}
	}
	if file, err := macho.Open(path); err == nil {
		defer func() { _ = file.Close() }()
		arch := map[macho.Cpu]string{macho.Cpu386: "x86", macho.CpuAmd64: "x86_64", macho.CpuArm: "arm", macho.CpuArm64: "arm64"}[file.Cpu]
		if arch != "" {
			return "darwin-" + arch
		}
	}
	return ""
}

func inspectPackage(dependency lockfile.Package) []Finding {
	var findings []Finding
	if dependency.Checksum == "" && dependency.Source.Type != lockfile.SourceTypeLocal {
		findings = append(findings, Finding{
			ID: "checksum-missing", Severity: "warning", Certainty: "confirmed", Package: dependency.Name,
			Message: "resolved package has no content checksum",
		})
	}
	if dependency.Source.Type != lockfile.SourceTypeLocal {
		findings = append(findings, Finding{
			ID: "license-metadata-missing", Severity: "warning", Certainty: "confirmed", Package: dependency.Name,
			Message: "lockfile contains no licence metadata for this package",
		})
	}
	if insecureSource(dependency.Source.URL) {
		findings = append(findings, Finding{
			ID: "source-insecure", Severity: "warning", Certainty: "confirmed", Package: dependency.Name,
			Message: "package source does not use a secure transport",
		})
	}
	if sourceHasCredentials(dependency.Source.URL) {
		findings = append(findings, Finding{
			ID: "source-credential", Severity: "error", Certainty: "heuristic", Package: dependency.Name,
			Message: "package source URL may contain credentials; value redacted",
		})
	}
	if len(dependency.PlatformArtifacts) != 0 && !supportsCurrentPlatform(dependency.PlatformArtifacts) {
		findings = append(findings, Finding{
			ID: "platform-artifact-missing", Severity: "warning", Certainty: "confirmed", Package: dependency.Name,
			Message: "no artifact matches " + platformID(),
		})
	}
	for _, artifact := range dependency.PlatformArtifacts {
		if artifact.URL != "" && artifact.Checksum == "" {
			findings = append(findings, Finding{
				ID:       "artifact-checksum-missing/" + artifact.Platform,
				Severity: "warning", Certainty: "confirmed", Package: dependency.Name,
				Message: artifact.Platform + " artifact has no checksum",
			})
		}
	}
	return findings
}

func insecureSource(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" {
		return false
	}
	return parsed.Scheme != "https" && parsed.Scheme != "ssh" && parsed.Scheme != "git+ssh" && parsed.Scheme != "file"
}

func sourceHasCredentials(value string) bool {
	prefix, rest, ok := strings.Cut(value, "://")
	return ok && prefix != "" && strings.Contains(strings.SplitN(rest, "/", 2)[0], "@")
}

func supportsCurrentPlatform(artifacts []lockfile.PlatformArtifact) bool {
	want := platformID()
	for _, artifact := range artifacts {
		if artifact.Platform == want {
			return true
		}
	}
	return false
}

func platformID() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	return runtime.GOOS + "-" + arch
}
