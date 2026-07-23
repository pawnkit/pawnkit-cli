package doctor_test

import (
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
	projectmodel "github.com/pawnkit/pawn-project/project"
	"github.com/pawnkit/pawnkit-cli/pkg/doctor"
	"github.com/pawnkit/pawnkit-core/source"
)

func TestInspectReportsUnpinnedDependencyAndMissingLock(t *testing.T) {
	filesystem := fsx.NewMem()
	filesystem.AddFile("/project/pawn.json", []byte(`{"entry":"main.pwn","dependencies":["owner/package@main"],"experimental":{"build_file":false}}`))
	filesystem.AddFile("/project/main.pwn", []byte("main() {}\n"))
	project, err := projectmodel.Load(source.NewRegistry(), filesystem, "/project", projectmodel.Options{})
	if err != nil {
		t.Fatal(err)
	}
	report := doctor.Inspect(project, filesystem)
	if len(report.Findings) != 2 {
		t.Fatalf("findings = %+v", report.Findings)
	}
	for _, finding := range report.Findings {
		if finding.Certainty != doctor.Confirmed || finding.Remediation == nil || finding.Remediation.Class != doctor.FixReview {
			t.Fatalf("finding = %+v", finding)
		}
		if finding.Remediation.Command != "" {
			t.Fatalf("unsupported command suggested: %+v", finding.Remediation)
		}
	}
}

func TestInspectRedactsSecretsAndFindsCaseCollisions(t *testing.T) {
	filesystem := fsx.NewMem()
	filesystem.AddFile("/project/pawn.json", []byte(`{"entry":"main.pwn","experimental":{"build_file":false}}`))
	filesystem.AddFile("/project/main.pwn", []byte("main() {}\n"))
	filesystem.AddFile("/project/config.json", []byte(`{"api_token":"do-not-print-this"}`))
	filesystem.AddFile("/project/Data/Value.inc", nil)
	filesystem.AddFile("/project/data/value.inc", nil)
	project, err := projectmodel.Load(source.NewRegistry(), filesystem, "/project", projectmodel.Options{})
	if err != nil {
		t.Fatal(err)
	}
	report := doctor.Inspect(project, filesystem)
	if len(report.Findings) != 2 {
		t.Fatalf("findings = %+v", report.Findings)
	}
	for _, finding := range report.Findings {
		if strings.Contains(finding.Message, "do-not-print-this") {
			t.Fatalf("secret leaked: %+v", finding)
		}
	}
}
