package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/stackgen-cli/envgraph/internal/policy"
)

var initPolicyCmd = &cobra.Command{
	Use:   "init-policy [output]",
	Short: "Generate an example policy file",
	Long: `Generate an example .envgraph-policy.yaml file with common validation rules.

This creates a starting point for configuring custom environment variable
validation rules including:
  • Required variables
  • Forbidden variables
  • Naming patterns
  • Prefix requirements
  • Secret detection

Examples:
  envgraph init-policy
  envgraph init-policy .envgraph-policy.yaml
  envgraph init-policy --output custom-policy.yaml`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInitPolicy,
}

var policyOutputPath string

func init() {
	rootCmd.AddCommand(initPolicyCmd)
	initPolicyCmd.Flags().StringVarP(&policyOutputPath, "output", "o", ".envgraph-policy.yaml", "output path for policy file")
}

func runInitPolicy(cmd *cobra.Command, args []string) error {
	outputPath := policyOutputPath
	if len(args) > 0 {
		outputPath = args[0]
	}

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil && !dryRun {
		return fmt.Errorf("file already exists: %s (use --dry-run to preview)", outputPath)
	}

	// Get example policy
	content := policy.ExamplePolicy()

	if dryRun {
		color.Cyan("Would create %s:\n\n", outputPath)
		fmt.Println(content)
		return nil
	}

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write policy file: %w", err)
	}

	if !quiet {
		color.Green("✅ Created policy file: %s", outputPath)
		color.White("\nEdit this file to customize your validation rules, then run:")
		color.Cyan("  envgraph scan --policy %s", outputPath)
	}

	return nil
}
