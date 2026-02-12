// Package analyzer provides validation and analysis for environment variables
package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stackgen-cli/envgraph/internal/models"
)

// Analyze performs validation analysis on scan results
func Analyze(scanResult *models.ScanResult) (*models.Report, error) {
	scanPath := ""
	if len(scanResult.ComposeFiles) > 0 {
		scanPath = scanResult.ComposeFiles[0]
	} else if len(scanResult.EnvFiles) > 0 {
		scanPath = scanResult.EnvFiles[0]
	}
	report := models.NewReport(scanPath)

	// Copy variables to report
	report.Variables = scanResult.Variables
	report.Services = scanResult.Services

	// Run validation checks
	checkUsedButNotDefined(report)
	checkDefinedButUnused(report)
	checkMultipleDefinitions(report)
	checkComposeOverrides(report)
	checkCaseCollisions(report)

	// Calculate statistics
	calculateStats(report, scanResult)

	return report, nil
}

// checkUsedButNotDefined finds variables that are referenced but never defined
func checkUsedButNotDefined(report *models.Report) {
	for name, v := range report.Variables {
		if !v.IsDefined() && v.IsUsed() {
			issue := models.ValidationIssue{
				Type:         models.IssueUsedButNotDefined,
				Severity:     models.SeverityError,
				VariableName: name,
				Message:      fmt.Sprintf("Variable '%s' is used but not defined", name),
				Sources:      v.UsedIn,
			}
			report.Issues = append(report.Issues, issue)
		}
	}
}

// checkDefinedButUnused finds variables that are defined but never used
func checkDefinedButUnused(report *models.Report) {
	for name, v := range report.Variables {
		if v.IsDefined() && !v.IsUsed() {
			// Skip .env.example - those are documentation
			isOnlyInExample := true
			for _, src := range v.DefinedIn {
				if src.Layer != models.LayerEnvExample {
					isOnlyInExample = false
					break
				}
			}
			if isOnlyInExample {
				continue
			}

			issue := models.ValidationIssue{
				Type:         models.IssueDefinedButUnused,
				Severity:     models.SeverityWarning,
				VariableName: name,
				Message:      fmt.Sprintf("Variable '%s' is defined but not used", name),
				Sources:      v.DefinedIn,
			}
			report.Issues = append(report.Issues, issue)
		}
	}
}

// checkMultipleDefinitions finds variables defined in multiple places
func checkMultipleDefinitions(report *models.Report) {
	for name, v := range report.Variables {
		if v.IsMultiplyDefined() {
			// Group by unique file paths
			files := make(map[string]bool)
			for _, src := range v.DefinedIn {
				files[src.FilePath] = true
			}

			if len(files) > 1 {
				issue := models.ValidationIssue{
					Type:         models.IssueMultipleDefinitions,
					Severity:     models.SeverityWarning,
					VariableName: name,
					Message:      fmt.Sprintf("Variable '%s' is defined in %d different files", name, len(files)),
					Sources:      v.DefinedIn,
				}
				report.Issues = append(report.Issues, issue)
			}
		}
	}
}

// checkComposeOverrides detects when compose inline env overrides env files
func checkComposeOverrides(report *models.Report) {
	for name, v := range report.Variables {
		if !v.IsMultiplyDefined() {
			continue
		}

		hasEnvFile := false
		hasComposeInline := false
		var sources []models.Source

		for _, src := range v.DefinedIn {
			if src.Layer == models.LayerComposeEnvFile || src.SourceType == models.SourceEnvFile {
				hasEnvFile = true
				sources = append(sources, src)
			}
			if src.Layer == models.LayerComposeInline {
				hasComposeInline = true
				sources = append(sources, src)
			}
		}

		if hasEnvFile && hasComposeInline {
			issue := models.ValidationIssue{
				Type:         models.IssueComposeOverride,
				Severity:     models.SeverityWarning,
				VariableName: name,
				Message:      fmt.Sprintf("Variable '%s' in env file is overridden by compose inline definition", name),
				Sources:      sources,
			}
			report.Issues = append(report.Issues, issue)
		}
	}
}

// checkCaseCollisions finds variables that differ only by case
func checkCaseCollisions(report *models.Report) {
	// Build a map of lowercase names to actual names
	lowerToNames := make(map[string][]string)
	for name := range report.Variables {
		lower := strings.ToLower(name)
		lowerToNames[lower] = append(lowerToNames[lower], name)
	}

	// Find collisions
	reported := make(map[string]bool)
	for _, names := range lowerToNames {
		if len(names) > 1 {
			// Sort for deterministic output
			sort.Strings(names)
			key := strings.Join(names, ",")
			if reported[key] {
				continue
			}
			reported[key] = true

			// Collect all sources
			var sources []models.Source
			for _, name := range names {
				v := report.Variables[name]
				sources = append(sources, v.DefinedIn...)
				sources = append(sources, v.UsedIn...)
			}

			issue := models.ValidationIssue{
				Type:         models.IssueCaseCollision,
				Severity:     models.SeverityWarning,
				VariableName: names[0], // Primary name
				Message:      fmt.Sprintf("Case collision detected: %s", strings.Join(names, ", ")),
				Sources:      sources,
				RelatedVars:  names,
			}
			report.Issues = append(report.Issues, issue)
		}
	}
}

// calculateStats computes summary statistics
func calculateStats(report *models.Report, scanResult *models.ScanResult) {
	stats := &report.Stats

	stats.TotalVariables = len(report.Variables)
	stats.EnvFilesScanned = len(scanResult.EnvFiles)
	stats.ComposeFilesScanned = len(scanResult.ComposeFiles)
	stats.SourceFilesScanned = len(scanResult.SourceFiles)

	for _, v := range report.Variables {
		if v.IsDefined() {
			stats.DefinedVariables++
		}
		if v.IsUsed() {
			stats.UsedVariables++
		}
		if !v.IsDefined() && v.IsUsed() {
			stats.UndefinedCount++
		}
		if v.IsDefined() && !v.IsUsed() {
			stats.UnusedCount++
		}
	}

	for _, issue := range report.Issues {
		switch issue.Severity {
		case models.SeverityError:
			stats.ErrorCount++
		case models.SeverityWarning:
			stats.WarningCount++
		}
	}
}
