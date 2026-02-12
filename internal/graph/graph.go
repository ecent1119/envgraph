// Package graph provides dependency graph generation and formatting
package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stackgen-cli/envgraph/internal/models"
)

// Build creates a dependency graph from scan results and report
func Build(scanResult *models.ScanResult, report *models.Report) *models.DependencyGraph {
	g := models.NewDependencyGraph()

	// Build service nodes from compose services
	for _, svc := range scanResult.Services {
		node := &models.ServiceNode{
			Name:      svc.ServiceName,
			Variables: svc.Variables,
			EnvFiles:  svc.EnvFiles,
		}
		g.Services[svc.ServiceName] = node
	}

	// Build variable nodes
	for name := range report.Variables {
		node := &models.VariableNode{
			Name:     name,
			Services: []string{},
		}

		// Find which services use this variable
		for _, svc := range scanResult.Services {
			for _, varName := range svc.Variables {
				if varName == name {
					node.Services = append(node.Services, svc.ServiceName)
					break
				}
			}
		}

		// Check if shared across multiple services
		node.IsShared = len(node.Services) > 1
		if node.IsShared {
			g.SharedVariables = append(g.SharedVariables, name)
		}

		g.Variables[name] = node
	}

	// Sort shared variables for deterministic output
	sort.Strings(g.SharedVariables)

	return g
}

// FormatTree generates an ASCII tree representation of the graph
func FormatTree(g *models.DependencyGraph) (string, error) {
	var sb strings.Builder

	sb.WriteString("Environment Variable Dependency Graph\n")
	sb.WriteString("=====================================\n\n")

	// Get sorted service names for deterministic output
	serviceNames := make([]string, 0, len(g.Services))
	for name := range g.Services {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)

	for i, serviceName := range serviceNames {
		svc := g.Services[serviceName]
		isLast := i == len(serviceNames)-1
		prefix := "├──"
		childPrefix := "│   "
		if isLast {
			prefix = "└──"
			childPrefix = "    "
		}

		sb.WriteString(fmt.Sprintf("%s 📦 %s\n", prefix, serviceName))

		// Sort variables
		vars := make([]string, len(svc.Variables))
		copy(vars, svc.Variables)
		sort.Strings(vars)

		for j, varName := range vars {
			varIsLast := j == len(vars)-1
			varPrefix := "├──"
			if varIsLast {
				varPrefix = "└──"
			}

			// Check if shared
			shared := ""
			if node, ok := g.Variables[varName]; ok && node.IsShared {
				shared = " (shared)"
			}

			sb.WriteString(fmt.Sprintf("%s%s 🔑 %s%s\n", childPrefix, varPrefix, varName, shared))
		}

		// Show env files if any
		if len(svc.EnvFiles) > 0 {
			for j, envFile := range svc.EnvFiles {
				envIsLast := j == len(svc.EnvFiles)-1
				envPrefix := "├──"
				if envIsLast {
					envPrefix = "└──"
				}
				sb.WriteString(fmt.Sprintf("%s%s 📄 %s\n", childPrefix, envPrefix, envFile))
			}
		}

		sb.WriteString("\n")
	}

	// Show shared variables summary
	if len(g.SharedVariables) > 0 {
		sb.WriteString("Shared Variables\n")
		sb.WriteString("----------------\n")
		for _, varName := range g.SharedVariables {
			if node, ok := g.Variables[varName]; ok {
				sb.WriteString(fmt.Sprintf("  🔗 %s: %s\n", varName, strings.Join(node.Services, ", ")))
			}
		}
	}

	return sb.String(), nil
}

// FormatDOT generates a Graphviz DOT representation of the graph
func FormatDOT(g *models.DependencyGraph) (string, error) {
	var sb strings.Builder

	sb.WriteString("digraph envgraph {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box];\n\n")

	// Define service nodes
	sb.WriteString("  // Services\n")
	sb.WriteString("  subgraph cluster_services {\n")
	sb.WriteString("    label=\"Services\";\n")
	sb.WriteString("    style=filled;\n")
	sb.WriteString("    color=lightgrey;\n")
	for name := range g.Services {
		sb.WriteString(fmt.Sprintf("    \"%s\" [shape=box3d, style=filled, fillcolor=lightblue];\n", name))
	}
	sb.WriteString("  }\n\n")

	// Define variable nodes
	sb.WriteString("  // Variables\n")
	sb.WriteString("  subgraph cluster_variables {\n")
	sb.WriteString("    label=\"Environment Variables\";\n")
	for name, node := range g.Variables {
		color := "white"
		if node.IsShared {
			color = "lightyellow"
		}
		sb.WriteString(fmt.Sprintf("    \"var_%s\" [label=\"%s\", shape=ellipse, style=filled, fillcolor=%s];\n", name, name, color))
	}
	sb.WriteString("  }\n\n")

	// Define service -> variable edges
	sb.WriteString("  // Dependencies\n")
	for serviceName, svc := range g.Services {
		for _, varName := range svc.Variables {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"var_%s\";\n", serviceName, varName))
		}
	}

	sb.WriteString("}\n")

	return sb.String(), nil
}
