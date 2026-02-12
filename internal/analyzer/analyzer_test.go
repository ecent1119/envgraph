// Package analyzer_test tests the validation analyzer
package analyzer

import (
	"testing"

	"github.com/stackgen-cli/envgraph/internal/models"
)

func TestAnalyze_UsedButNotDefined(t *testing.T) {
	result := models.NewScanResult()

	// Variable used but never defined
	result.Variables["UNDEFINED_VAR"] = &models.Variable{
		Name: "UNDEFINED_VAR",
		UsedIn: []models.Source{
			{FilePath: "docker-compose.yml", SourceType: models.SourceComposeYAML},
		},
	}

	report, err := Analyze(result)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should have one issue for undefined var
	foundIssue := false
	for _, issue := range report.Issues {
		if issue.Type == models.IssueUsedButNotDefined && issue.VariableName == "UNDEFINED_VAR" {
			foundIssue = true
			if issue.Severity != models.SeverityError {
				t.Error("used-but-not-defined should be error severity")
			}
		}
	}
	if !foundIssue {
		t.Error("expected UsedButNotDefined issue for UNDEFINED_VAR")
	}
}

func TestAnalyze_DefinedButUnused(t *testing.T) {
	result := models.NewScanResult()

	// Variable defined but never used (not in .env.example)
	result.Variables["UNUSED_VAR"] = &models.Variable{
		Name: "UNUSED_VAR",
		DefinedIn: []models.Source{
			{FilePath: ".env", Layer: models.LayerEnv, SourceType: models.SourceEnvFile},
		},
	}

	report, err := Analyze(result)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	foundIssue := false
	for _, issue := range report.Issues {
		if issue.Type == models.IssueDefinedButUnused && issue.VariableName == "UNUSED_VAR" {
			foundIssue = true
			if issue.Severity != models.SeverityWarning {
				t.Error("defined-but-unused should be warning severity")
			}
		}
	}
	if !foundIssue {
		t.Error("expected DefinedButUnused issue for UNUSED_VAR")
	}
}

func TestAnalyze_SkipsEnvExample(t *testing.T) {
	result := models.NewScanResult()

	// Variable only in .env.example should NOT trigger unused warning
	result.Variables["EXAMPLE_ONLY"] = &models.Variable{
		Name: "EXAMPLE_ONLY",
		DefinedIn: []models.Source{
			{FilePath: ".env.example", Layer: models.LayerEnvExample, SourceType: models.SourceEnvFile},
		},
	}

	report, err := Analyze(result)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	for _, issue := range report.Issues {
		if issue.VariableName == "EXAMPLE_ONLY" {
			t.Error("should not generate issue for variables only in .env.example")
		}
	}
}

func TestAnalyze_MultipleDefinitions(t *testing.T) {
	result := models.NewScanResult()

	// Variable defined in multiple files
	result.Variables["MULTI_DEF"] = &models.Variable{
		Name: "MULTI_DEF",
		DefinedIn: []models.Source{
			{FilePath: ".env", Layer: models.LayerEnv, SourceType: models.SourceEnvFile},
			{FilePath: ".env.local", Layer: models.LayerEnvLocal, SourceType: models.SourceEnvFile},
		},
		UsedIn: []models.Source{
			{FilePath: "docker-compose.yml", SourceType: models.SourceComposeYAML},
		},
	}

	report, err := Analyze(result)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	foundIssue := false
	for _, issue := range report.Issues {
		if issue.Type == models.IssueMultipleDefinitions && issue.VariableName == "MULTI_DEF" {
			foundIssue = true
		}
	}
	if !foundIssue {
		t.Error("expected MultipleDefinitions issue for MULTI_DEF")
	}
}

func TestAnalyze_ComposeOverride(t *testing.T) {
	result := models.NewScanResult()

	// Variable in env file AND compose inline
	result.Variables["OVERRIDE_VAR"] = &models.Variable{
		Name: "OVERRIDE_VAR",
		DefinedIn: []models.Source{
			{FilePath: ".env", Layer: models.LayerComposeEnvFile, SourceType: models.SourceEnvFile},
			{FilePath: "docker-compose.yml", Layer: models.LayerComposeInline, SourceType: models.SourceComposeYAML},
		},
		UsedIn: []models.Source{
			{FilePath: "docker-compose.yml", SourceType: models.SourceComposeYAML},
		},
	}

	report, err := Analyze(result)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	foundIssue := false
	for _, issue := range report.Issues {
		if issue.Type == models.IssueComposeOverride && issue.VariableName == "OVERRIDE_VAR" {
			foundIssue = true
		}
	}
	if !foundIssue {
		t.Error("expected ComposeOverride issue for OVERRIDE_VAR")
	}
}

func TestAnalyze_CaseCollisions(t *testing.T) {
	result := models.NewScanResult()

	// Variables differing only in case
	result.Variables["DB_PASSWORD"] = &models.Variable{
		Name: "DB_PASSWORD",
		DefinedIn: []models.Source{
			{FilePath: ".env", Layer: models.LayerEnv},
		},
	}
	result.Variables["db_password"] = &models.Variable{
		Name: "db_password",
		DefinedIn: []models.Source{
			{FilePath: ".env.local", Layer: models.LayerEnvLocal},
		},
	}
	result.Variables["Db_Password"] = &models.Variable{
		Name: "Db_Password",
		DefinedIn: []models.Source{
			{FilePath: ".env.other", Layer: models.LayerEnvOther},
		},
	}

	report, err := Analyze(result)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	foundIssue := false
	for _, issue := range report.Issues {
		if issue.Type == models.IssueCaseCollision {
			foundIssue = true
		}
	}
	if !foundIssue {
		t.Error("expected CaseCollision issue for DB_PASSWORD variants")
	}
}

func TestAnalyze_NoIssuesCleanProject(t *testing.T) {
	result := models.NewScanResult()

	// Clean project: all variables properly defined and used
	result.Variables["CLEAN_VAR"] = &models.Variable{
		Name: "CLEAN_VAR",
		DefinedIn: []models.Source{
			{FilePath: ".env", Layer: models.LayerEnv, SourceType: models.SourceEnvFile},
		},
		UsedIn: []models.Source{
			{FilePath: "docker-compose.yml", SourceType: models.SourceComposeYAML},
		},
	}

	report, err := Analyze(result)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(report.Issues) > 0 {
		t.Errorf("expected 0 issues for clean project, got %d", len(report.Issues))
		for _, issue := range report.Issues {
			t.Logf("  - %s: %s", issue.Type, issue.Message)
		}
	}
}

func TestAnalyze_Statistics(t *testing.T) {
	result := models.NewScanResult()
	result.EnvFiles = []string{".env", ".env.example"}
	result.ComposeFiles = []string{"docker-compose.yml"}

	result.Variables["VAR1"] = &models.Variable{Name: "VAR1"}
	result.Variables["VAR2"] = &models.Variable{Name: "VAR2"}
	result.Variables["VAR3"] = &models.Variable{Name: "VAR3"}

	result.Services = []models.ServiceDependency{
		{ServiceName: "api"},
		{ServiceName: "db"},
	}

	report, err := Analyze(result)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if report.Stats.TotalVariables != 3 {
		t.Errorf("expected 3 total variables, got %d", report.Stats.TotalVariables)
	}
	if report.Stats.EnvFilesScanned != 2 {
		t.Errorf("expected 2 env files scanned, got %d", report.Stats.EnvFilesScanned)
	}
}
