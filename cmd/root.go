package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"
	cfgFile string
	dryRun  bool
	quiet   bool
)

var rootCmd = &cobra.Command{
	Use:   "envgraph",
	Short: "Analyze, validate, and visualize environment variables",
	Long: color.New(color.FgCyan).Sprint(`
envgraph - Environment Variable Analyzer & Graph Tool

Scan your projects to discover, validate, and visualize environment
variables across .env files, Docker Compose, and source code.

`) + color.New(color.FgHiBlack).Sprint("Features:") + `
  • Variable inventory across all sources
  • Validation checks (unused, undefined, conflicts)
  • Layer precedence analysis
  • Service dependency graphs
  • Multiple output formats (text, JSON, markdown, DOT)

` + color.New(color.FgYellow).Sprint("Note: envgraph observes and reports. It does not enforce or guarantee."),
	Version: version,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./envgraph.yaml)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "preview changes without writing files")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "minimal output (for CI)")
}
