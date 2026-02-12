// Package models defines the core data structures for envgraph
package models

import (
	"time"
)

// Layer represents the precedence layer of an environment variable definition
type Layer int

const (
	LayerEnv Layer = iota
	LayerEnvLocal
	LayerEnvExample
	LayerEnvOther
	LayerComposeEnvFile
	LayerComposeInline
)

// String returns a human-readable name for the layer
func (l Layer) String() string {
	switch l {
	case LayerEnv:
		return ".env"
	case LayerEnvLocal:
		return ".env.local"
	case LayerEnvExample:
		return ".env.example"
	case LayerEnvOther:
		return ".env.*"
	case LayerComposeEnvFile:
		return "compose env_file"
	case LayerComposeInline:
		return "compose inline"
	default:
		return "unknown"
	}
}

// Precedence returns the precedence order (higher = wins)
func (l Layer) Precedence() int {
	switch l {
	case LayerEnv:
		return 1
	case LayerEnvLocal:
		return 2
	case LayerEnvOther:
		return 3
	case LayerComposeEnvFile:
		return 4
	case LayerComposeInline:
		return 5
	default:
		return 0
	}
}

// SourceType represents the type of source a variable was found in
type SourceType int

const (
	SourceEnvFile SourceType = iota
	SourceComposeYAML
	SourceCode
)

// String returns a human-readable name for the source type
func (s SourceType) String() string {
	switch s {
	case SourceEnvFile:
		return "env file"
	case SourceComposeYAML:
		return "compose"
	case SourceCode:
		return "source code"
	default:
		return "unknown"
	}
}

// Source represents a location where a variable was found
type Source struct {
	FilePath   string     `json:"file_path"`
	SourceType SourceType `json:"source_type"`
	Layer      Layer      `json:"layer,omitempty"`
	LineNumber int        `json:"line_number,omitempty"`
	Service    string     `json:"service,omitempty"` // For compose sources
}

// Variable represents an environment variable discovered during scanning
type Variable struct {
	Name            string   `json:"name"`
	Value           string   `json:"value,omitempty"` // Only set if defined (not for references)
	HasValue        bool     `json:"has_value"`
	DefinedIn       []Source `json:"defined_in,omitempty"`
	UsedIn          []Source `json:"used_in,omitempty"`
	EffectiveLayer  Layer    `json:"effective_layer,omitempty"`
	EffectiveValue  string   `json:"effective_value,omitempty"`
	EffectiveSource *Source  `json:"effective_source,omitempty"`
}

// IsDefined returns true if the variable is defined in at least one source
func (v *Variable) IsDefined() bool {
	return len(v.DefinedIn) > 0
}

// IsUsed returns true if the variable is referenced in at least one place
func (v *Variable) IsUsed() bool {
	return len(v.UsedIn) > 0
}

// IsMultiplyDefined returns true if defined in multiple places
func (v *Variable) IsMultiplyDefined() bool {
	return len(v.DefinedIn) > 1
}

// Severity represents the severity level of a validation issue
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

// String returns a human-readable name for the severity
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// Symbol returns the emoji symbol for the severity
func (s Severity) Symbol() string {
	switch s {
	case SeverityError:
		return "❌"
	case SeverityWarning:
		return "⚠️"
	case SeverityInfo:
		return "ℹ️"
	default:
		return "?"
	}
}

// IssueType represents the type of validation issue
type IssueType int

const (
	IssueUsedButNotDefined IssueType = iota
	IssueDefinedButUnused
	IssueMultipleDefinitions
	IssueComposeOverride
	IssueCaseCollision
)

// String returns a human-readable name for the issue type
func (i IssueType) String() string {
	switch i {
	case IssueUsedButNotDefined:
		return "used but not defined"
	case IssueDefinedButUnused:
		return "defined but unused"
	case IssueMultipleDefinitions:
		return "multiple definitions"
	case IssueComposeOverride:
		return "compose override"
	case IssueCaseCollision:
		return "case collision"
	default:
		return "unknown"
	}
}

// DefaultSeverity returns the default severity for an issue type
func (i IssueType) DefaultSeverity() Severity {
	switch i {
	case IssueUsedButNotDefined:
		return SeverityError
	case IssueDefinedButUnused:
		return SeverityWarning
	case IssueMultipleDefinitions:
		return SeverityWarning
	case IssueComposeOverride:
		return SeverityWarning
	case IssueCaseCollision:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

// ValidationIssue represents a problem found during analysis
type ValidationIssue struct {
	Type         IssueType `json:"type"`
	Severity     Severity  `json:"severity"`
	VariableName string    `json:"variable_name"`
	Message      string    `json:"message"`
	Sources      []Source  `json:"sources,omitempty"`
	RelatedVars  []string  `json:"related_vars,omitempty"` // For case collisions
}

// ServiceDependency represents a compose service and its required variables
type ServiceDependency struct {
	ServiceName string   `json:"service_name"`
	Variables   []string `json:"variables"`
	EnvFiles    []string `json:"env_files,omitempty"`
}

// Report is the final analysis result
type Report struct {
	// Metadata
	ScanPath  string    `json:"scan_path"`
	ScanTime  time.Time `json:"scan_time"`
	
	// Inventory
	Variables map[string]*Variable `json:"variables"`

	// Analysis results
	Issues []ValidationIssue `json:"issues"`

	// Service dependencies
	Services []ServiceDependency `json:"services,omitempty"`

	// Statistics
	Stats ReportStats `json:"stats"`
}

// ReportStats contains summary statistics
type ReportStats struct {
	TotalVariables    int `json:"total_variables"`
	DefinedVariables  int `json:"defined_variables"`
	UsedVariables     int `json:"used_variables"`
	UndefinedCount    int `json:"undefined_count"`
	UnusedCount       int `json:"unused_count"`
	ErrorCount        int `json:"error_count"`
	WarningCount      int `json:"warning_count"`
	EnvFilesScanned   int `json:"env_files_scanned"`
	ComposeFilesScanned int `json:"compose_files_scanned"`
	SourceFilesScanned  int `json:"source_files_scanned"`
}

// DependencyGraph represents the service-variable dependency graph
type DependencyGraph struct {
	// Services maps service names to their dependencies
	Services map[string]*ServiceNode `json:"services"`

	// Variables maps variable names to which services use them
	Variables map[string]*VariableNode `json:"variables"`

	// SharedVariables are used by multiple services
	SharedVariables []string `json:"shared_variables"`
}

// ServiceNode represents a service in the dependency graph
type ServiceNode struct {
	Name      string   `json:"name"`
	Variables []string `json:"variables"`
	EnvFiles  []string `json:"env_files,omitempty"`
}

// VariableNode represents a variable in the dependency graph
type VariableNode struct {
	Name     string   `json:"name"`
	Services []string `json:"services"`
	IsShared bool     `json:"is_shared"`
}

// ScanResult holds the raw results from parsing
type ScanResult struct {
	// Variables collected from all sources
	Variables map[string]*Variable

	// Files scanned
	EnvFiles     []string
	ComposeFiles []string
	SourceFiles  []string

	// Services found in compose files
	Services []ServiceDependency

	// Any parse errors (non-fatal)
	Errors []error
}

// NewScanResult creates an initialized ScanResult
func NewScanResult() *ScanResult {
	return &ScanResult{
		Variables:    make(map[string]*Variable),
		EnvFiles:     []string{},
		ComposeFiles: []string{},
		SourceFiles:  []string{},
		Services:     []ServiceDependency{},
		Errors:       []error{},
	}
}

// NewReport creates an initialized Report
func NewReport(scanPath string) *Report {
	return &Report{
		ScanPath:  scanPath,
		ScanTime:  time.Now(),
		Variables: make(map[string]*Variable),
		Issues:    []ValidationIssue{},
		Services:  []ServiceDependency{},
	}
}

// NewDependencyGraph creates an initialized DependencyGraph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Services:        make(map[string]*ServiceNode),
		Variables:       make(map[string]*VariableNode),
		SharedVariables: []string{},
	}
}
