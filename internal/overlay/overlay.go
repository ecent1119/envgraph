// Package overlay implements graph combining functionality
package overlay

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stackgen-cli/envgraph/internal/models"
)

// CombineMode determines how graphs are combined
type CombineMode string

const (
	ModeUnion     CombineMode = "union"     // Include all variables from all reports
	ModeIntersect CombineMode = "intersect" // Include only variables that appear in all reports
	ModeDiff      CombineMode = "diff"      // Show only differences between reports
)

// OverlayResult holds the combined result of multiple reports
type OverlayResult struct {
	Mode       CombineMode
	Sources    []string               // Source paths/names
	Variables  map[string]*OverlayVar
	ServiceMap map[string][]string     // variable -> services
}

// OverlayVar holds variable information across multiple sources
type OverlayVar struct {
	Name        string
	InSources   []string          // Which sources contain this variable
	Values      map[string]string // Source -> value mapping
	IsConflict  bool              // Values differ across sources
	IsDefined   map[string]bool   // Is defined per source
	IsUsed      map[string]bool   // Is used per source
}

// Combine combines multiple reports into an overlay result
func Combine(reports []*models.Report, sources []string, mode CombineMode) *OverlayResult {
	result := &OverlayResult{
		Mode:       mode,
		Sources:    sources,
		Variables:  make(map[string]*OverlayVar),
		ServiceMap: make(map[string][]string),
	}

	// First pass: collect all variables
	allVars := make(map[string]bool)
	varCounts := make(map[string]int)
	for i, report := range reports {
		for name := range report.Variables {
			allVars[name] = true
			varCounts[name]++
			
			// Track services
			for _, svc := range report.Services {
				for _, varName := range svc.Variables {
					if varName == name {
						if !contains(result.ServiceMap[name], svc.ServiceName) {
							result.ServiceMap[name] = append(result.ServiceMap[name], svc.ServiceName)
						}
					}
				}
			}
		}
		_ = i
	}

	// Apply mode filter
	for name := range allVars {
		include := false
		switch mode {
		case ModeUnion:
			include = true
		case ModeIntersect:
			include = varCounts[name] == len(reports)
		case ModeDiff:
			include = varCounts[name] < len(reports)
		}

		if include {
			result.Variables[name] = &OverlayVar{
				Name:      name,
				Values:    make(map[string]string),
				IsDefined: make(map[string]bool),
				IsUsed:    make(map[string]bool),
			}
		}
	}

	// Second pass: populate values
	for i, report := range reports {
		source := sources[i]
		for name, ov := range result.Variables {
			if v, exists := report.Variables[name]; exists {
				ov.InSources = append(ov.InSources, source)
				ov.Values[source] = v.EffectiveValue
				ov.IsDefined[source] = v.IsDefined()
				ov.IsUsed[source] = v.IsUsed()
			}
		}
	}

	// Detect conflicts
	for _, ov := range result.Variables {
		values := make(map[string]bool)
		for _, v := range ov.Values {
			values[v] = true
		}
		ov.IsConflict = len(values) > 1
	}

	return result
}

// FormatOverlay formats an overlay result as text
func FormatOverlay(result *OverlayResult) string {
	var sb strings.Builder

	sb.WriteString("# Graph Overlay Report\n\n")
	sb.WriteString(fmt.Sprintf("**Mode:** %s\n", result.Mode))
	sb.WriteString(fmt.Sprintf("**Sources:** %s\n\n", strings.Join(result.Sources, ", ")))

	// Sort variables
	var names []string
	for name := range result.Variables {
		names = append(names, name)
	}
	sort.Strings(names)

	// Count conflicts
	conflictCount := 0
	for _, ov := range result.Variables {
		if ov.IsConflict {
			conflictCount++
		}
	}

	sb.WriteString(fmt.Sprintf("**Variables:** %d total, %d with conflicts\n\n", len(names), conflictCount))

	// List conflicts first if any
	if conflictCount > 0 {
		sb.WriteString("## ⚠️ Conflicts\n\n")
		for _, name := range names {
			ov := result.Variables[name]
			if ov.IsConflict {
				sb.WriteString(fmt.Sprintf("### %s\n", name))
				for source, value := range ov.Values {
					displayValue := value
					if value == "" {
						displayValue = "(empty)"
					}
					sb.WriteString(fmt.Sprintf("  - %s: `%s`\n", source, displayValue))
				}
				sb.WriteString("\n")
			}
		}
	}

	// List all variables
	sb.WriteString("## Variables\n\n")
	sb.WriteString("| Variable | Sources | Status |\n")
	sb.WriteString("|----------|---------|--------|\n")
	for _, name := range names {
		ov := result.Variables[name]
		status := "✅"
		if ov.IsConflict {
			status = "⚠️ conflict"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", name, strings.Join(ov.InSources, ", "), status))
	}

	return sb.String()
}

// GetConflicts returns only conflicting variables
func (r *OverlayResult) GetConflicts() []*OverlayVar {
	var conflicts []*OverlayVar
	for _, ov := range r.Variables {
		if ov.IsConflict {
			conflicts = append(conflicts, ov)
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].Name < conflicts[j].Name
	})
	return conflicts
}

// GetUniqueToSource returns variables unique to a specific source
func (r *OverlayResult) GetUniqueToSource(source string) []*OverlayVar {
	var unique []*OverlayVar
	for _, ov := range r.Variables {
		if len(ov.InSources) == 1 && ov.InSources[0] == source {
			unique = append(unique, ov)
		}
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].Name < unique[j].Name
	})
	return unique
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
