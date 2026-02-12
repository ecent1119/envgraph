// Package reporter_test tests the output reporters
package reporter

import (
	"strings"
	"testing"

	"github.com/stackgen-cli/envgraph/internal/models"
)

func createTestReport() *models.Report {
	report := models.NewReport("/test/path")
	report.Variables["API_KEY"] = &models.Variable{
		Name: "API_KEY",
		DefinedIn: []models.Source{
			{FilePath: ".env", Layer: models.LayerEnv},
		},
		UsedIn: []models.Source{
			{FilePath: "docker-compose.yml", SourceType: models.SourceComposeYAML},
		},
	}
	report.Variables["UNDEFINED_VAR"] = &models.Variable{
		Name: "UNDEFINED_VAR",
		UsedIn: []models.Source{
			{FilePath: "docker-compose.yml", SourceType: models.SourceComposeYAML},
		},
	}

	report.Issues = []models.ValidationIssue{
		{
			Type:         models.IssueUsedButNotDefined,
			Severity:     models.SeverityError,
			VariableName: "UNDEFINED_VAR",
			Message:      "Variable 'UNDEFINED_VAR' is used but not defined",
		},
	}

	report.Services = []models.ServiceDependency{
		{ServiceName: "api", Variables: []string{"API_KEY", "UNDEFINED_VAR"}},
	}

	report.Stats = models.ReportStats{
		TotalVariables:  2,
		EnvFilesScanned: 1,
		ErrorCount:      1,
	}

	return report
}

func TestTextReporter(t *testing.T) {
	report := createTestReport()

	output, err := FormatText(report)
	if err != nil {
		t.Fatalf("FormatText failed: %v", err)
	}

	// Should contain header
	if !strings.Contains(output, "Environment Variable Analysis") {
		t.Error("text output should contain header")
	}

	// Should list variables
	if !strings.Contains(output, "API_KEY") {
		t.Error("text output should contain API_KEY")
	}

	// Should show issues
	if !strings.Contains(output, "UNDEFINED_VAR") {
		t.Error("text output should show undefined variable issue")
	}
}

func TestJSONReporter(t *testing.T) {
	report := createTestReport()

	output, err := FormatJSON(report)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}

	// Should be valid JSON
	if !strings.Contains(output, "{") || !strings.Contains(output, "}") {
		t.Error("output should be valid JSON")
	}

	// Should have expected fields
	if !strings.Contains(output, "variables") {
		t.Error("JSON should have 'variables' field")
	}
	if !strings.Contains(output, "issues") {
		t.Error("JSON should have 'issues' field")
	}
}

func TestMarkdownReporter(t *testing.T) {
	report := createTestReport()

	output, err := FormatMarkdown(report)
	if err != nil {
		t.Fatalf("FormatMarkdown failed: %v", err)
	}

	// Should have markdown headers
	if !strings.Contains(output, "#") {
		t.Error("markdown should contain headers")
	}

	// Should have variables section
	if !strings.Contains(output, "API_KEY") {
		t.Error("markdown should show API_KEY")
	}
}

func TestEmptyReport(t *testing.T) {
	report := models.NewReport("/empty")

	// Text should not panic
	if _, err := FormatText(report); err != nil {
		t.Errorf("FormatText failed on empty report: %v", err)
	}

	// JSON should not panic
	if _, err := FormatJSON(report); err != nil {
		t.Errorf("FormatJSON failed on empty report: %v", err)
	}

	// Markdown should not panic
	if _, err := FormatMarkdown(report); err != nil {
		t.Errorf("FormatMarkdown failed on empty report: %v", err)
	}
}

func TestReportWithManyIssues(t *testing.T) {
	report := models.NewReport("/test")

	// Add many issues of different types
	types := []models.IssueType{
		models.IssueUsedButNotDefined,
		models.IssueDefinedButUnused,
		models.IssueMultipleDefinitions,
		models.IssueComposeOverride,
		models.IssueCaseCollision,
	}

	for i, issueType := range types {
		report.Issues = append(report.Issues, models.ValidationIssue{
			Type:         issueType,
			Severity:     models.SeverityWarning,
			VariableName: "TEST_VAR_" + string(rune('A'+i)),
			Message:      "Test issue " + string(rune('A'+i)),
		})
	}

	output, err := FormatText(report)
	if err != nil {
		t.Errorf("FormatText failed with many issues: %v", err)
	}

	// Should handle multiple issue types
	if len(output) == 0 {
		t.Error("output should not be empty")
	}
}
