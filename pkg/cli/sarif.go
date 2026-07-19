package cli

import (
	"io"

	"github.com/pawnkit/pawnkit-cli/pkg/workflow"
	"github.com/pawnkit/pawnkit-core/source"
)

type sarifReport struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name string `json:"name"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           sarifRegion   `json:"region"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
}

func writeSARIF(output io.Writer, tasks []workflow.Result) error {
	results := make([]sarifResult, 0)
	for _, task := range tasks {
		for _, finding := range task.Findings {
			results = append(results, sarifResult{
				RuleID: finding.RuleID, Level: sarifLevel(finding.Severity), Message: sarifMessage{Text: finding.Message},
				Locations: []sarifLocation{{PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifact{URI: source.FileURI(finding.Path).String()},
					Region:           sarifRegion{StartLine: max(finding.Line, 1), StartColumn: max(finding.Column, 1)},
				}}},
			})
		}
	}
	report := sarifReport{
		Version: "2.1.0", Schema: "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{Tool: sarifTool{Driver: sarifDriver{Name: "pawn"}}, Results: results}},
	}
	return writeJSON(output, report)
}

func sarifLevel(severity string) string {
	switch severity {
	case "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "note"
	}
}
