// Package policy implements custom validation rules for environment variables
package policy

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/stackgen-cli/envgraph/internal/models"
	"gopkg.in/yaml.v3"
)

// Rule represents a policy validation rule
type Rule struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Type        string `yaml:"type"` // required, forbidden, pattern, prefix
	Severity    string `yaml:"severity,omitempty"` // error, warning, info (default: error)

	// Type-specific fields
	Pattern   string   `yaml:"pattern,omitempty"`    // For pattern type
	Prefix    string   `yaml:"prefix,omitempty"`     // For prefix type  
	Variables []string `yaml:"variables,omitempty"`  // For required/forbidden types
	Services  []string `yaml:"services,omitempty"`   // Optional: apply only to these services
}

// PolicyFile represents an envgraph policy file
type PolicyFile struct {
	Version     string `yaml:"version"`
	Description string `yaml:"description,omitempty"`
	Rules       []Rule `yaml:"rules"`
}

// Violation represents a policy rule violation
type Violation struct {
	RuleID      string
	RuleName    string
	Severity    models.Severity
	Variable    string
	Message     string
	Service     string
}

// LoadPolicy loads a policy file
func LoadPolicy(path string) (*PolicyFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var policy PolicyFile
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return nil, err
	}

	return &policy, nil
}

// Check runs policy checks against variable names and values (method form)
func (p *PolicyFile) Check(vars map[string]string) []Violation {
	var violations []Violation

	for _, rule := range p.Rules {
		ruleViolations := checkRuleVars(rule, vars)
		violations = append(violations, ruleViolations...)
	}

	return violations
}

// CheckReport runs policy checks against scan results
func CheckReport(policy *PolicyFile, report *models.Report) []Violation {
	var violations []Violation

	for _, rule := range policy.Rules {
		ruleViolations := checkRule(rule, report)
		violations = append(violations, ruleViolations...)
	}

	return violations
}

// checkRuleVars checks a rule against variable map
func checkRuleVars(rule Rule, vars map[string]string) []Violation {
	var violations []Violation
	severity := parseSeverity(rule.Severity)

	switch rule.Type {
	case "required":
		for _, required := range rule.Variables {
			if _, exists := vars[required]; !exists {
				violations = append(violations, Violation{
					RuleID:   rule.ID,
					RuleName: rule.Name,
					Severity: severity,
					Variable: required,
					Message:  fmt.Sprintf("required variable '%s' is missing", required),
				})
			}
		}

	case "forbidden":
		for _, forbidden := range rule.Variables {
			if _, exists := vars[forbidden]; exists {
				violations = append(violations, Violation{
					RuleID:   rule.ID,
					RuleName: rule.Name,
					Severity: severity,
					Variable: forbidden,
					Message:  fmt.Sprintf("forbidden variable '%s' should not be present", forbidden),
				})
			}
		}

	case "pattern":
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return violations
		}
		for name := range vars {
			if !re.MatchString(name) {
				violations = append(violations, Violation{
					RuleID:   rule.ID,
					RuleName: rule.Name,
					Severity: severity,
					Variable: name,
					Message:  fmt.Sprintf("variable '%s' does not match pattern '%s'", name, rule.Pattern),
				})
			}
		}

	case "prefix":
		for name := range vars {
			if len(rule.Prefix) > 0 && !hasPrefix(name, rule.Prefix) {
				violations = append(violations, Violation{
					RuleID:   rule.ID,
					RuleName: rule.Name,
					Severity: severity,
					Variable: name,
					Message:  fmt.Sprintf("variable '%s' must have prefix '%s'", name, rule.Prefix),
				})
			}
		}

	case "no-secrets-in-code":
		// This type needs source location info from report
		// Skip for simple variable checking
	}

	return violations
}

func hasPrefix(name, prefix string) bool {
	return len(name) >= len(prefix) && name[:len(prefix)] == prefix
}

// FormatViolations formats violations as a human-readable string
func FormatViolations(violations []Violation) string {
	if len(violations) == 0 {
		return "✅ No policy violations found.\n"
	}

	var sb strings.Builder
	sb.WriteString("# Policy Violations\n\n")

	// Group by severity
	errors := []Violation{}
	warnings := []Violation{}
	infos := []Violation{}

	for _, v := range violations {
		switch v.Severity {
		case models.SeverityError:
			errors = append(errors, v)
		case models.SeverityWarning:
			warnings = append(warnings, v)
		default:
			infos = append(infos, v)
		}
	}

	if len(errors) > 0 {
		sb.WriteString(fmt.Sprintf("## ❌ Errors (%d)\n", len(errors)))
		for _, v := range errors {
			sb.WriteString(fmt.Sprintf("  - [%s] %s: %s\n", v.RuleID, v.Variable, v.Message))
		}
		sb.WriteString("\n")
	}

	if len(warnings) > 0 {
		sb.WriteString(fmt.Sprintf("## ⚠️ Warnings (%d)\n", len(warnings)))
		for _, v := range warnings {
			sb.WriteString(fmt.Sprintf("  - [%s] %s: %s\n", v.RuleID, v.Variable, v.Message))
		}
		sb.WriteString("\n")
	}

	if len(infos) > 0 {
		sb.WriteString(fmt.Sprintf("## ℹ️ Info (%d)\n", len(infos)))
		for _, v := range infos {
			sb.WriteString(fmt.Sprintf("  - [%s] %s: %s\n", v.RuleID, v.Variable, v.Message))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("**Total:** %d violations (%d errors, %d warnings, %d info)\n",
		len(violations), len(errors), len(warnings), len(infos)))

	return sb.String()
}

// Check runs policy checks against scan results
func Check(policy *PolicyFile, report *models.Report) []Violation {
	var violations []Violation

	for _, rule := range policy.Rules {
		ruleViolations := checkRule(rule, report)
		violations = append(violations, ruleViolations...)
	}

	return violations
}

func checkRule(rule Rule, report *models.Report) []Violation {
	var violations []Violation

	severity := parseSeverity(rule.Severity)

	switch rule.Type {
	case "required":
		// Variables that must be defined
		for _, required := range rule.Variables {
			if v, exists := report.Variables[required]; !exists || !v.IsDefined() {
				violations = append(violations, Violation{
					RuleID:   rule.ID,
					RuleName: rule.Name,
					Severity: severity,
					Variable: required,
					Message:  fmt.Sprintf("Required variable '%s' is not defined", required),
				})
			}
		}

	case "forbidden":
		// Variables that must NOT be defined
		for _, forbidden := range rule.Variables {
			if v, exists := report.Variables[forbidden]; exists && v.IsDefined() {
				violations = append(violations, Violation{
					RuleID:   rule.ID,
					RuleName: rule.Name,
					Severity: severity,
					Variable: forbidden,
					Message:  fmt.Sprintf("Forbidden variable '%s' is defined", forbidden),
				})
			}
		}

	case "pattern":
		// Variables must match a pattern
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return violations
		}

		for name := range report.Variables {
			if !re.MatchString(name) {
				// Check if this variable should match
				for _, prefix := range rule.Variables {
					if matched, _ := regexp.MatchString("^"+prefix, name); matched {
						violations = append(violations, Violation{
							RuleID:   rule.ID,
							RuleName: rule.Name,
							Severity: severity,
							Variable: name,
							Message:  fmt.Sprintf("Variable '%s' does not match required pattern: %s", name, rule.Pattern),
						})
					}
				}
			}
		}

	case "prefix":
		// Certain services should only use prefixed variables
		if len(rule.Services) > 0 {
			for _, svc := range report.Services {
				if !containsService(rule.Services, svc.ServiceName) {
					continue
				}

				for _, varName := range svc.Variables {
					if rule.Prefix != "" && len(varName) > 0 {
						matched, _ := regexp.MatchString("^"+rule.Prefix, varName)
						if !matched {
							violations = append(violations, Violation{
								RuleID:   rule.ID,
								RuleName: rule.Name,
								Severity: severity,
								Variable: varName,
								Service:  svc.ServiceName,
								Message:  fmt.Sprintf("Service '%s' variable '%s' should have prefix '%s'", svc.ServiceName, varName, rule.Prefix),
							})
						}
					}
				}
			}
		}

	case "no-secrets-in-code":
		// Check for certain variable patterns that shouldn't be in code
		secretPatterns := []string{
			"PASSWORD", "SECRET", "KEY", "TOKEN", "CREDENTIAL", "AUTH",
		}
		for name, v := range report.Variables {
			for _, pattern := range secretPatterns {
				if matched, _ := regexp.MatchString("(?i)"+pattern, name); matched {
					// Check if this variable is referenced in source code
					for _, src := range v.UsedIn {
						if src.SourceType == models.SourceCode {
							violations = append(violations, Violation{
								RuleID:   rule.ID,
								RuleName: rule.Name,
								Severity: severity,
								Variable: name,
								Message:  fmt.Sprintf("Potentially sensitive variable '%s' referenced in source code at %s:%d", name, src.FilePath, src.LineNumber),
							})
						}
					}
				}
			}
		}
	}

	return violations
}

func parseSeverity(s string) models.Severity {
	switch s {
	case "warning":
		return models.SeverityWarning
	case "info":
		return models.SeverityInfo
	default:
		return models.SeverityError
	}
}

func containsService(services []string, name string) bool {
	for _, s := range services {
		if s == name {
			return true
		}
	}
	return false
}

// ExamplePolicy returns an example policy file content
func ExamplePolicy() string {
	return `# envgraph.policy.yaml - Environment variable policy rules
version: "1.0"
description: "Example policy rules for environment validation"

rules:
  # Required variables that must be defined
  - id: required-db
    name: "Database Configuration Required"
    type: required
    severity: error
    variables:
      - DATABASE_URL
      - DATABASE_HOST

  # Variables that should not be present
  - id: no-deprecated
    name: "No Deprecated Variables"
    type: forbidden
    severity: warning
    variables:
      - OLD_API_KEY
      - LEGACY_DB_URL

  # Service-specific prefix requirements
  - id: api-prefix
    name: "API Service Prefix"
    type: prefix
    severity: warning
    services:
      - api
      - backend
    prefix: "API_"

  # Security: don't reference secrets in code
  - id: secrets-check
    name: "Sensitive Data in Code"
    type: no-secrets-in-code
    severity: error
`
}
