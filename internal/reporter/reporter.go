// Package reporter provides output formatting for envgraph reports
package reporter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/stackgen-cli/envgraph/internal/models"
)

// FormatText generates a colored text report
func FormatText(report *models.Report) (string, error) {
	var sb strings.Builder

	// Header
	sb.WriteString(color.CyanString("Environment Variable Analysis Report\n"))
	sb.WriteString(color.CyanString("====================================\n\n"))

	// Summary
	sb.WriteString(color.WhiteString("Summary\n"))
	sb.WriteString("-------\n")
	sb.WriteString(fmt.Sprintf("  Total variables:    %d\n", report.Stats.TotalVariables))
	sb.WriteString(fmt.Sprintf("  Defined:            %d\n", report.Stats.DefinedVariables))
	sb.WriteString(fmt.Sprintf("  Used:               %d\n", report.Stats.UsedVariables))
	sb.WriteString(fmt.Sprintf("  Undefined (errors): %d\n", report.Stats.UndefinedCount))
	sb.WriteString(fmt.Sprintf("  Unused (warnings):  %d\n", report.Stats.UnusedCount))
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("  Env files scanned:     %d\n", report.Stats.EnvFilesScanned))
	sb.WriteString(fmt.Sprintf("  Compose files scanned: %d\n", report.Stats.ComposeFilesScanned))
	sb.WriteString(fmt.Sprintf("  Source files scanned:  %d\n", report.Stats.SourceFilesScanned))
	sb.WriteString("\n")

	// Issues
	if len(report.Issues) > 0 {
		sb.WriteString(color.YellowString("Issues Found\n"))
		sb.WriteString("------------\n")

		// Group by severity
		errors := []models.ValidationIssue{}
		warnings := []models.ValidationIssue{}
		info := []models.ValidationIssue{}

		for _, issue := range report.Issues {
			switch issue.Severity {
			case models.SeverityError:
				errors = append(errors, issue)
			case models.SeverityWarning:
				warnings = append(warnings, issue)
			default:
				info = append(info, issue)
			}
		}

		if len(errors) > 0 {
			sb.WriteString(color.RedString("\n❌ Errors\n"))
			for _, issue := range errors {
				sb.WriteString(formatIssue(issue))
			}
		}

		if len(warnings) > 0 {
			sb.WriteString(color.YellowString("\n⚠️  Warnings\n"))
			for _, issue := range warnings {
				sb.WriteString(formatIssue(issue))
			}
		}

		if len(info) > 0 {
			sb.WriteString(color.BlueString("\nℹ️  Info\n"))
			for _, issue := range info {
				sb.WriteString(formatIssue(issue))
			}
		}
	} else {
		sb.WriteString(color.GreenString("✅ No issues found\n"))
	}
	sb.WriteString("\n")

	// Variable list
	if len(report.Variables) > 0 {
		sb.WriteString(color.WhiteString("Variables\n"))
		sb.WriteString("---------\n")

		// Sort variable names
		names := make([]string, 0, len(report.Variables))
		for name := range report.Variables {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			v := report.Variables[name]
			status := "✓"
			statusColor := color.GreenString
			if !v.IsDefined() {
				status = "✗"
				statusColor = color.RedString
			} else if !v.IsUsed() {
				status = "○"
				statusColor = color.YellowString
			}

			layers := []string{}
			for _, src := range v.DefinedIn {
				layers = append(layers, src.Layer.String())
			}
			layerStr := ""
			if len(layers) > 0 {
				layerStr = fmt.Sprintf(" (%s)", strings.Join(unique(layers), ", "))
			}

			sb.WriteString(fmt.Sprintf("  %s %s%s\n", statusColor(status), name, layerStr))
		}
	}

	return sb.String(), nil
}

func formatIssue(issue models.ValidationIssue) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  • %s\n", issue.Message))
	for _, src := range issue.Sources {
		loc := src.FilePath
		if src.LineNumber > 0 {
			loc = fmt.Sprintf("%s:%d", src.FilePath, src.LineNumber)
		}
		sb.WriteString(fmt.Sprintf("      → %s\n", loc))
	}
	return sb.String()
}

// FormatJSON generates a JSON report
func FormatJSON(report *models.Report) (string, error) {
	output := struct {
		SchemaVersion string         `json:"schema_version"`
		*models.Report
	}{
		SchemaVersion: "1.0",
		Report:        report,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatMarkdown generates a Markdown report
func FormatMarkdown(report *models.Report) (string, error) {
	var sb strings.Builder

	sb.WriteString("# Environment Variable Analysis Report\n\n")

	// Summary table
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Total variables | %d |\n", report.Stats.TotalVariables))
	sb.WriteString(fmt.Sprintf("| Defined | %d |\n", report.Stats.DefinedVariables))
	sb.WriteString(fmt.Sprintf("| Used | %d |\n", report.Stats.UsedVariables))
	sb.WriteString(fmt.Sprintf("| Undefined (errors) | %d |\n", report.Stats.UndefinedCount))
	sb.WriteString(fmt.Sprintf("| Unused (warnings) | %d |\n", report.Stats.UnusedCount))
	sb.WriteString(fmt.Sprintf("| Env files scanned | %d |\n", report.Stats.EnvFilesScanned))
	sb.WriteString(fmt.Sprintf("| Compose files scanned | %d |\n", report.Stats.ComposeFilesScanned))
	sb.WriteString(fmt.Sprintf("| Source files scanned | %d |\n", report.Stats.SourceFilesScanned))
	sb.WriteString("\n")

	// Issues
	if len(report.Issues) > 0 {
		sb.WriteString("## Issues\n\n")

		for _, issue := range report.Issues {
			icon := "ℹ️"
			switch issue.Severity {
			case models.SeverityError:
				icon = "❌"
			case models.SeverityWarning:
				icon = "⚠️"
			}

			sb.WriteString(fmt.Sprintf("### %s %s\n\n", icon, issue.VariableName))
			sb.WriteString(fmt.Sprintf("**%s** - %s\n\n", issue.Type.String(), issue.Message))

			if len(issue.Sources) > 0 {
				sb.WriteString("Sources:\n")
				for _, src := range issue.Sources {
					loc := src.FilePath
					if src.LineNumber > 0 {
						loc = fmt.Sprintf("%s:%d", src.FilePath, src.LineNumber)
					}
					sb.WriteString(fmt.Sprintf("- `%s`\n", loc))
				}
				sb.WriteString("\n")
			}
		}
	}

	// Variables table
	if len(report.Variables) > 0 {
		sb.WriteString("## Variables\n\n")
		sb.WriteString("| Variable | Status | Defined In | Used In |\n")
		sb.WriteString("|----------|--------|------------|--------|\n")

		names := make([]string, 0, len(report.Variables))
		for name := range report.Variables {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			v := report.Variables[name]
			status := "✅ OK"
			if !v.IsDefined() {
				status = "❌ Undefined"
			} else if !v.IsUsed() {
				status = "⚠️ Unused"
			}

			definedIn := []string{}
			for _, src := range v.DefinedIn {
				definedIn = append(definedIn, src.Layer.String())
			}

			usedIn := []string{}
			for _, src := range v.UsedIn {
				usedIn = append(usedIn, src.SourceType.String())
			}

			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n",
				name,
				status,
				strings.Join(unique(definedIn), ", "),
				strings.Join(unique(usedIn), ", "),
			))
		}
	}

	// Services
	if len(report.Services) > 0 {
		sb.WriteString("\n## Services\n\n")
		for _, svc := range report.Services {
			sb.WriteString(fmt.Sprintf("### %s\n\n", svc.ServiceName))
			if len(svc.Variables) > 0 {
				sb.WriteString("**Variables:**\n")
				for _, v := range svc.Variables {
					sb.WriteString(fmt.Sprintf("- `%s`\n", v))
				}
			}
			if len(svc.EnvFiles) > 0 {
				sb.WriteString("\n**Env files:**\n")
				for _, f := range svc.EnvFiles {
					sb.WriteString(fmt.Sprintf("- `%s`\n", f))
				}
			}
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// FormatDOT generates a Graphviz DOT representation from the report
func FormatDOT(report *models.Report) (string, error) {
	var sb strings.Builder

	sb.WriteString("digraph envgraph {\n")
	sb.WriteString("  rankdir=TB;\n")
	sb.WriteString("  node [shape=box];\n\n")

	// Variable nodes colored by status
	for name, v := range report.Variables {
		color := "lightgreen"
		if !v.IsDefined() {
			color = "lightcoral"
		} else if !v.IsUsed() {
			color = "lightyellow"
		}
		sb.WriteString(fmt.Sprintf("  \"%s\" [style=filled, fillcolor=%s];\n", name, color))
	}

	// Service nodes
	for _, svc := range report.Services {
		sb.WriteString(fmt.Sprintf("  \"%s\" [shape=box3d, style=filled, fillcolor=lightblue];\n", svc.ServiceName))
		for _, varName := range svc.Variables {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", svc.ServiceName, varName))
		}
	}

	sb.WriteString("}\n")
	return sb.String(), nil
}

// GenerateEnvExample generates a .env.example file content
func GenerateEnvExample(report *models.Report) string {
	var sb strings.Builder

	sb.WriteString("# Environment Variables\n")
	sb.WriteString("# Generated by envgraph\n\n")

	// Sort variables
	names := make([]string, 0, len(report.Variables))
	for name := range report.Variables {
		names = append(names, name)
	}
	sort.Strings(names)

	// Group by first letter or prefix
	currentPrefix := ""
	for _, name := range names {
		v := report.Variables[name]

		// Add section header for common prefixes
		prefix := getPrefix(name)
		if prefix != currentPrefix && prefix != "" {
			sb.WriteString(fmt.Sprintf("\n# %s\n", prefix))
			currentPrefix = prefix
		}

		// Add comment about where it's used
		if len(v.UsedIn) > 0 {
			services := []string{}
			for _, src := range v.UsedIn {
				if src.Service != "" && !containsStr(services, src.Service) {
					services = append(services, src.Service)
				}
			}
			if len(services) > 0 {
				sb.WriteString(fmt.Sprintf("# Used by: %s\n", strings.Join(services, ", ")))
			}
		}

		// Write the variable
		value := ""
		if v.HasValue && v.EffectiveValue != "" {
			// Mask potential secrets
			if isLikelySecret(name) {
				value = "<your-" + strings.ToLower(name) + ">"
			} else {
				value = v.EffectiveValue
			}
		}
		sb.WriteString(fmt.Sprintf("%s=%s\n", name, value))
	}

	return sb.String()
}

func unique(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func getPrefix(name string) string {
	prefixes := []string{
		"DATABASE", "DB", "POSTGRES", "MYSQL", "REDIS", "NEO4J",
		"AWS", "AZURE", "GCP", "GOOGLE",
		"API", "AUTH", "JWT", "OAUTH",
		"SMTP", "EMAIL", "MAIL",
		"LOG", "DEBUG",
	}
	upper := strings.ToUpper(name)
	for _, p := range prefixes {
		if strings.HasPrefix(upper, p+"_") || upper == p {
			return p
		}
	}
	return ""
}

func isLikelySecret(name string) bool {
	lower := strings.ToLower(name)
	secrets := []string{
		"password", "secret", "key", "token", "api_key", "apikey",
		"private", "credential", "auth",
	}
	for _, s := range secrets {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}
