package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/stackgen-cli/envgraph/internal/analyzer"
	"github.com/stackgen-cli/envgraph/internal/baseline"
	"github.com/stackgen-cli/envgraph/internal/graph"
	"github.com/stackgen-cli/envgraph/internal/models"
	"github.com/stackgen-cli/envgraph/internal/overlay"
	"github.com/stackgen-cli/envgraph/internal/parser"
	"github.com/stackgen-cli/envgraph/internal/policy"
	"github.com/stackgen-cli/envgraph/internal/reporter"
	"github.com/stackgen-cli/envgraph/internal/tui"
)

var (
	composeFiles    []string
	outputFormat    string
	graphFormat     string
	excludePaths    []string
	includeSource   bool
	fixMode         bool
	tuiMode         bool
	policyFile      string
	baselineFile    string
	saveBaseline    string
	overlayPaths    []string
	overlayMode     string
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a directory for environment variables",
	Long: `Scan a directory to discover and analyze environment variables.

Scans:
  • .env, .env.local, .env.example, .env.* files
  • docker-compose.yml and docker-compose.*.yml
  • Source code (with --include-source flag)

Features:
  • Policy checks with custom rules (--policy)
  • Baseline comparison to detect drift (--baseline, --save-baseline)
  • Graph overlays to compare multiple projects (--overlay)

Examples:
  envgraph scan .
  envgraph scan ./myproject
  envgraph scan --compose docker-compose.yml
  envgraph scan --format json
  envgraph scan --graph dot > deps.dot
  envgraph scan --fix
  envgraph scan --policy .envgraph-policy.yaml
  envgraph scan --save-baseline .envgraph-baseline.json
  envgraph scan --baseline .envgraph-baseline.json
  envgraph scan --overlay ../other-project --overlay-mode diff`,
	Args: cobra.MaximumNArgs(1),
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringSliceVarP(&composeFiles, "compose", "c", nil, "compose file(s) to scan")
	scanCmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "output format (text, json, markdown, dot)")
	scanCmd.Flags().StringVarP(&graphFormat, "graph", "g", "", "graph format (tree, dot)")
	scanCmd.Flags().StringSliceVarP(&excludePaths, "exclude", "e", nil, "paths to exclude")
	scanCmd.Flags().BoolVar(&includeSource, "include-source", false, "scan source code for env references")
	scanCmd.Flags().BoolVar(&fixMode, "fix", false, "generate cleaned .env.example")
	scanCmd.Flags().BoolVar(&tuiMode, "tui", false, "launch interactive TUI")

	// Policy checks
	scanCmd.Flags().StringVarP(&policyFile, "policy", "p", "", "policy file for validation rules")

	// Baseline comparison
	scanCmd.Flags().StringVar(&baselineFile, "baseline", "", "compare against baseline file")
	scanCmd.Flags().StringVar(&saveBaseline, "save-baseline", "", "save current state as baseline")

	// Graph overlays
	scanCmd.Flags().StringSliceVar(&overlayPaths, "overlay", nil, "additional paths to overlay/compare")
	scanCmd.Flags().StringVar(&overlayMode, "overlay-mode", "union", "overlay mode (union, intersect, diff)")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Determine scan path
	scanPath := "."
	if len(args) > 0 {
		scanPath = args[0]
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Verify path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Build scan options
	opts := parser.ScanOptions{
		Path:          absPath,
		ComposeFiles:  composeFiles,
		ExcludePaths:  excludePaths,
		IncludeSource: includeSource,
	}

	// Run parser
	if !quiet {
		color.Cyan("Scanning %s...\n", absPath)
	}

	scanResult, err := parser.ScanDirectory(opts)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Run analyzer
	report, err := analyzer.Analyze(scanResult)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Handle overlay mode - scan additional paths and combine
	if len(overlayPaths) > 0 {
		return runOverlay(report, absPath, overlayPaths)
	}

	// Handle policy validation
	if policyFile != "" {
		return runPolicy(report, policyFile)
	}

	// Handle save baseline
	if saveBaseline != "" {
		return runSaveBaseline(report, absPath, saveBaseline)
	}

	// Handle compare against baseline
	if baselineFile != "" {
		return runCompareBaseline(report, baselineFile)
	}

	// Build graph if requested
	var depGraph *models.DependencyGraph
	if graphFormat != "" || tuiMode {
		depGraph = graph.Build(scanResult, report)
	}

	// Launch TUI if requested
	if tuiMode {
		return tui.Run(report, depGraph)
	}

	// Handle fix mode
	if fixMode {
		return runFix(report, absPath)
	}

	// Generate graph output if requested
	if graphFormat != "" {
		return outputGraph(depGraph, graphFormat)
	}

	// Generate report output
	return outputReport(report, outputFormat)
}

func outputReport(report *models.Report, format string) error {
	var output string
	var err error

	switch format {
	case "text":
		output, err = reporter.FormatText(report)
	case "json":
		output, err = reporter.FormatJSON(report)
	case "markdown", "md":
		output, err = reporter.FormatMarkdown(report)
	case "dot":
		output, err = reporter.FormatDOT(report)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}

	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}

func outputGraph(g *models.DependencyGraph, format string) error {
	var output string
	var err error

	switch format {
	case "tree":
		output, err = graph.FormatTree(g)
	case "dot":
		output, err = graph.FormatDOT(g)
	default:
		return fmt.Errorf("unknown graph format: %s", format)
	}

	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}

func runFix(report *models.Report, basePath string) error {
	// Generate .env.example content
	content := reporter.GenerateEnvExample(report)

	// Determine output path
	outputPath := filepath.Join(basePath, ".env.example")

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil {
		// File exists, use .new suffix
		outputPath = filepath.Join(basePath, ".env.example.new")
		if !quiet {
			color.Yellow("⚠️  .env.example exists, writing to .env.example.new")
		}
	}

	// Dry run - just print
	if dryRun {
		color.Cyan("Would write to %s:\n", outputPath)
		fmt.Println(content)
		return nil
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if !quiet {
		color.Green("✅ Generated %s", outputPath)
	}

	// Also output JSON summary for CI
	if outputFormat == "json" {
		summary := map[string]interface{}{
			"action": "fix",
			"output": outputPath,
			"variables": len(report.Variables),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	return nil
}

func runPolicy(report *models.Report, policyPath string) error {
	// Load policy file
	pol, err := policy.LoadPolicy(policyPath)
	if err != nil {
		return fmt.Errorf("failed to load policy: %w", err)
	}

	if !quiet {
		color.Cyan("Checking policy %s...\n", policyPath)
	}

	// Get variable names and values
	vars := make(map[string]string)
	for name, v := range report.Variables {
		vars[name] = v.EffectiveValue
	}

	// Run policy checks
	violations := pol.Check(vars)

	// Output results
	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"policy":     policyPath,
			"violations": violations,
			"passed":     len(violations) == 0,
		})
	}

	// Text output
	output := policy.FormatViolations(violations)
	fmt.Print(output)

	if len(violations) > 0 {
		return fmt.Errorf("policy check failed with %d violations", len(violations))
	}

	return nil
}

func runSaveBaseline(report *models.Report, scanPath, outputPath string) error {
	bl := baseline.CreateBaseline(report, scanPath)

	if dryRun {
		color.Cyan("Would save baseline to %s\n", outputPath)
		return nil
	}

	if err := bl.Save(outputPath); err != nil {
		return fmt.Errorf("failed to save baseline: %w", err)
	}

	if !quiet {
		color.Green("✅ Saved baseline to %s (%d variables)\n", outputPath, len(bl.Variables))
	}

	return nil
}

func runCompareBaseline(report *models.Report, baselinePath string) error {
	bl, err := baseline.Load(baselinePath)
	if err != nil {
		return fmt.Errorf("failed to load baseline: %w", err)
	}

	if !quiet {
		color.Cyan("Comparing against baseline %s...\n", baselinePath)
	}

	result := baseline.Compare(bl, report)

	// Output results
	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"baseline":    baselinePath,
			"created_at":  bl.CreatedAt,
			"added":       result.AddedVars,
			"removed":     result.RemovedVars,
			"changed":     result.ChangedVars,
			"unchanged":   len(result.Same),
			"has_changes": len(result.AddedVars) > 0 || len(result.RemovedVars) > 0 || len(result.ChangedVars) > 0,
		})
	}

	// Text output
	output := baseline.FormatCompareResult(result)
	fmt.Print(output)

	return nil
}

func runOverlay(mainReport *models.Report, mainPath string, otherPaths []string) error {
	reports := []*models.Report{mainReport}
	sources := []string{mainPath}

	// Scan each overlay path
	for _, p := range otherPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("failed to resolve overlay path %s: %w", p, err)
		}

		opts := parser.ScanOptions{
			Path:          absPath,
			ExcludePaths:  excludePaths,
			IncludeSource: includeSource,
		}

		scanResult, err := parser.ScanDirectory(opts)
		if err != nil {
			return fmt.Errorf("failed to scan overlay path %s: %w", p, err)
		}

		report, err := analyzer.Analyze(scanResult)
		if err != nil {
			return fmt.Errorf("failed to analyze overlay path %s: %w", p, err)
		}

		reports = append(reports, report)
		sources = append(sources, absPath)
	}

	// Combine reports
	mode := overlay.CombineMode(overlayMode)
	result := overlay.Combine(reports, sources, mode)

	// Output results
	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]interface{}{
			"mode":      result.Mode,
			"sources":   result.Sources,
			"variables": result.Variables,
			"conflicts": result.GetConflicts(),
		})
	}

	// Text output
	output := overlay.FormatOverlay(result)
	fmt.Print(output)

	return nil
}
