// Package doctor aggregates safe project health checks.
package doctor

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	projectdoctor "github.com/pawnkit/pawn-project/doctor"
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/manifest"
	projectmodel "github.com/pawnkit/pawn-project/project"
)

type Certainty string

const (
	Confirmed Certainty = "confirmed"
	Inferred  Certainty = "inferred"
	Heuristic Certainty = "heuristic"
)

type FixClass string

const (
	FixSafe   FixClass = "safe"
	FixReview FixClass = "review-required"
)

type Remediation struct {
	Class   FixClass `json:"class"`
	Command string   `json:"command,omitempty"`
	Message string   `json:"message"`
}

type Finding struct {
	ID          string       `json:"id"`
	Severity    string       `json:"severity"`
	Certainty   Certainty    `json:"certainty"`
	Source      string       `json:"source"`
	Path        string       `json:"path,omitempty"`
	Message     string       `json:"message"`
	Remediation *Remediation `json:"remediation,omitempty"`
}

type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	Findings      []Finding `json:"findings"`
}

func Inspect(project *projectmodel.Project, filesystem fsx.FS) Report {
	report := Report{SchemaVersion: 1}
	environment := projectdoctor.Environment{
		FS: filesystem, Root: project.Root(), Manifest: project.Manifest(),
		Lock: project.Lockfile(), Paths: project.Paths(),
	}
	for _, result := range projectdoctor.Run(environment, projectdoctor.DefaultChecks()) {
		if result.Status == projectdoctor.StatusOK {
			continue
		}
		severity := "warning"
		if result.Status == projectdoctor.StatusFail {
			severity = "error"
		}
		report.Findings = append(report.Findings, Finding{
			ID: "pawn-project/" + result.Name, Severity: severity, Certainty: Confirmed,
			Source: "pawn-project", Message: result.Message,
		})
	}
	report.Findings = append(report.Findings, dependencyFindings(project.Manifest(), project.Lockfile() != nil)...)
	if entry := project.Paths().Entry; entry == "" || !fsx.IsFile(filesystem, entry) {
		report.Findings = append(report.Findings, Finding{
			ID: "entry-missing", Severity: "error", Certainty: Confirmed, Source: "pawn-project",
			Message:     "project entry file is missing",
			Remediation: &Remediation{Class: FixReview, Message: "set manifest entry to an existing Pawn source"},
		})
	}
	files, err := projectFiles(filesystem, project.Root(), 10_000)
	if err != nil {
		report.Findings = append(report.Findings, Finding{
			ID: "project-scan-incomplete", Severity: "warning", Certainty: Inferred,
			Source: "pawn-doctor", Message: "project scan was incomplete",
		})
	} else {
		report.Findings = append(report.Findings, caseFindings(files, project.Root())...)
		report.Findings = append(report.Findings, secretFindings(filesystem, files, project.Root())...)
	}
	sort.Slice(report.Findings, func(i, j int) bool { return report.Findings[i].ID < report.Findings[j].ID })
	return report
}

func dependencyFindings(projectManifest *manifest.Manifest, hasLock bool) []Finding {
	if projectManifest == nil {
		return nil
	}
	dependencies := append([]manifest.Dependency(nil), projectManifest.Dependencies...)
	dependencies = append(dependencies, projectManifest.DevDependencies...)
	var findings []Finding
	if len(dependencies) != 0 && !hasLock {
		findings = append(findings, Finding{
			ID: "lockfile-missing", Severity: "warning", Certainty: Confirmed, Source: "pawn-project",
			Message:     "dependencies are declared but pawn.lock is missing",
			Remediation: &Remediation{Class: FixReview, Message: "resolve dependencies and review the generated lockfile"},
		})
	}
	for _, dependency := range dependencies {
		if dependency.RefKind != manifest.RefNone && dependency.RefKind != manifest.RefBranch {
			continue
		}
		detail := "has no version pin"
		if dependency.RefKind == manifest.RefBranch {
			detail = fmt.Sprintf("uses mutable branch %q", dependency.Ref)
		}
		findings = append(findings, Finding{
			ID:       "dependency-unpinned/" + strings.ToLower(dependency.Name()),
			Severity: "warning", Certainty: Confirmed, Source: "pawn-project",
			Message:     fmt.Sprintf("dependency %s %s", dependency.Name(), detail),
			Remediation: &Remediation{Class: FixReview, Message: "pin and review an immutable version or commit"},
		})
	}
	return findings
}

func projectFiles(filesystem fsx.FS, root string, limit int) ([]string, error) {
	var files []string
	var walk func(string) error
	walk = func(directory string) error {
		entries, err := filesystem.ReadDir(directory)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if len(files) >= limit {
				return errors.New("file limit exceeded")
			}
			child := path.Join(directory, entry.Name())
			if entry.IsDir() {
				if entry.Name() != ".git" && entry.Name() != ".pawn-cache" {
					if err := walk(child); err != nil {
						return err
					}
				}
			} else {
				files = append(files, child)
			}
		}
		return nil
	}
	return files, walk(root)
}

func caseFindings(files []string, root string) []Finding {
	seen := make(map[string]string, len(files))
	var findings []Finding
	for _, file := range files {
		relative := strings.TrimPrefix(strings.TrimPrefix(file, root), "/")
		key := strings.ToLower(relative)
		if previous, exists := seen[key]; exists && previous != relative {
			findings = append(findings, Finding{
				ID: "path-case-collision/" + key, Severity: "warning", Certainty: Confirmed,
				Source: "pawn-doctor", Path: relative,
				Message:     "path differs only by case from " + previous,
				Remediation: &Remediation{Class: FixReview, Message: "rename one path and update its references"},
			})
		} else {
			seen[key] = relative
		}
	}
	return findings
}

var secretPattern = regexp.MustCompile(`(?i)["']?(password|token|secret|api[_-]?key)["']?\s*[:=]\s*["']?[^\s"',}]+`)

func secretFindings(filesystem fsx.FS, files []string, root string) []Finding {
	var findings []Finding
	for _, file := range files {
		if !configLike(file) {
			continue
		}
		content, err := filesystem.ReadFile(file)
		if err != nil || len(content) > 1<<20 || !secretPattern.Match(content) {
			continue
		}
		relative := strings.TrimPrefix(strings.TrimPrefix(file, root), "/")
		findings = append(findings, Finding{
			ID: "possible-secret/" + strings.ToLower(relative), Severity: "warning", Certainty: Heuristic,
			Source: "pawn-doctor", Path: relative,
			Message:     "possible credential found; value redacted",
			Remediation: &Remediation{Class: FixReview, Message: "move the value to a secret store and rotate it if exposed"},
		})
	}
	return findings
}

func configLike(file string) bool {
	switch strings.ToLower(path.Ext(file)) {
	case ".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".env":
		return true
	default:
		return path.Base(file) == ".env"
	}
}
