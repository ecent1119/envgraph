// Package baseline implements baseline comparison for environment variables
package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/stackgen-cli/envgraph/internal/models"
)

// Baseline represents a saved environment state
type Baseline struct {
	Version   string               `json:"version"`
	CreatedAt time.Time            `json:"created_at"`
	Path      string               `json:"path"`
	Variables map[string]BaselineVar `json:"variables"`
	Services  []string             `json:"services"`
}

// BaselineVar represents a variable in the baseline
type BaselineVar struct {
	Name      string   `json:"name"`
	Value     string   `json:"value,omitempty"`
	IsDefined bool     `json:"is_defined"`
	IsUsed    bool     `json:"is_used"`
	DefFiles  []string `json:"defined_in,omitempty"`
	UseFiles  []string `json:"used_in,omitempty"`
}

// CompareResult holds comparison between baseline and current state
type CompareResult struct {
	AddedVars   []string
	RemovedVars []string
	ChangedVars []ChangedVar
	Same        []string
}

// ChangedVar represents a variable that has changed
type ChangedVar struct {
	Name     string
	Field    string // "value", "defined", "used"
	OldValue string
	NewValue string
}

// CreateBaseline creates a baseline from current scan results
func CreateBaseline(report *models.Report, path string) *Baseline {
	baseline := &Baseline{
		Version:   "1.0",
		CreatedAt: time.Now(),
		Path:      path,
		Variables: make(map[string]BaselineVar),
	}

	for name, v := range report.Variables {
		bv := BaselineVar{
			Name:      name,
			Value:     v.EffectiveValue,
			IsDefined: v.IsDefined(),
			IsUsed:    v.IsUsed(),
		}

		for _, src := range v.DefinedIn {
			bv.DefFiles = append(bv.DefFiles, src.FilePath)
		}
		for _, src := range v.UsedIn {
			bv.UseFiles = append(bv.UseFiles, src.FilePath)
		}

		baseline.Variables[name] = bv
	}

	for _, svc := range report.Services {
		baseline.Services = append(baseline.Services, svc.ServiceName)
	}

	return baseline
}

// Save saves a baseline to a file
func (b *Baseline) Save(path string) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load loads a baseline from a file
func Load(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var baseline Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, err
	}

	return &baseline, nil
}

// Compare compares a baseline with current report
func Compare(baseline *Baseline, report *models.Report) *CompareResult {
	result := &CompareResult{}

	// Build current state map
	current := make(map[string]BaselineVar)
	for name, v := range report.Variables {
		current[name] = BaselineVar{
			Name:      name,
			Value:     v.EffectiveValue,
			IsDefined: v.IsDefined(),
			IsUsed:    v.IsUsed(),
		}
	}

	// Find added vars
	for name := range current {
		if _, exists := baseline.Variables[name]; !exists {
			result.AddedVars = append(result.AddedVars, name)
		}
	}

	// Find removed vars
	for name := range baseline.Variables {
		if _, exists := current[name]; !exists {
			result.RemovedVars = append(result.RemovedVars, name)
		}
	}

	// Find changed vars
	for name, baseVar := range baseline.Variables {
		if curVar, exists := current[name]; exists {
			// Check for changes
			if baseVar.Value != curVar.Value {
				result.ChangedVars = append(result.ChangedVars, ChangedVar{
					Name:     name,
					Field:    "value",
					OldValue: baseVar.Value,
					NewValue: curVar.Value,
				})
			} else if baseVar.IsDefined != curVar.IsDefined {
				result.ChangedVars = append(result.ChangedVars, ChangedVar{
					Name:     name,
					Field:    "defined",
					OldValue: fmt.Sprintf("%v", baseVar.IsDefined),
					NewValue: fmt.Sprintf("%v", curVar.IsDefined),
				})
			} else if baseVar.IsUsed != curVar.IsUsed {
				result.ChangedVars = append(result.ChangedVars, ChangedVar{
					Name:     name,
					Field:    "used",
					OldValue: fmt.Sprintf("%v", baseVar.IsUsed),
					NewValue: fmt.Sprintf("%v", curVar.IsUsed),
				})
			} else {
				result.Same = append(result.Same, name)
			}
		}
	}

	// Sort results
	sort.Strings(result.AddedVars)
	sort.Strings(result.RemovedVars)
	sort.Strings(result.Same)
	sort.Slice(result.ChangedVars, func(i, j int) bool {
		return result.ChangedVars[i].Name < result.ChangedVars[j].Name
	})

	return result
}

// FormatCompareResult formats a compare result as text
func FormatCompareResult(result *CompareResult) string {
	var sb strings.Builder

	sb.WriteString("# Baseline Comparison Report\n\n")

	if len(result.AddedVars) == 0 && len(result.RemovedVars) == 0 && len(result.ChangedVars) == 0 {
		sb.WriteString("✅ No changes detected from baseline.\n")
		return sb.String()
	}

	if len(result.AddedVars) > 0 {
		sb.WriteString(fmt.Sprintf("## ➕ Added (%d)\n", len(result.AddedVars)))
		for _, name := range result.AddedVars {
			sb.WriteString(fmt.Sprintf("  - %s\n", name))
		}
		sb.WriteString("\n")
	}

	if len(result.RemovedVars) > 0 {
		sb.WriteString(fmt.Sprintf("## ➖ Removed (%d)\n", len(result.RemovedVars)))
		for _, name := range result.RemovedVars {
			sb.WriteString(fmt.Sprintf("  - %s\n", name))
		}
		sb.WriteString("\n")
	}

	if len(result.ChangedVars) > 0 {
		sb.WriteString(fmt.Sprintf("## 🔄 Changed (%d)\n", len(result.ChangedVars)))
		for _, cv := range result.ChangedVars {
			sb.WriteString(fmt.Sprintf("  - %s (%s): %s → %s\n", cv.Name, cv.Field, cv.OldValue, cv.NewValue))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("**Summary:** %d added, %d removed, %d changed, %d unchanged\n",
		len(result.AddedVars), len(result.RemovedVars), len(result.ChangedVars), len(result.Same)))

	return sb.String()
}
