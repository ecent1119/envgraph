// Package graph_test tests the dependency graph builder
package graph

import (
	"strings"
	"testing"

	"github.com/stackgen-cli/envgraph/internal/models"
)

func TestBuild_BasicGraph(t *testing.T) {
	scanResult := models.NewScanResult()
	scanResult.Services = []models.ServiceDependency{
		{ServiceName: "api", Variables: []string{"DB_URL", "API_KEY"}},
		{ServiceName: "db", Variables: []string{"DB_PASSWORD"}},
	}

	report := models.NewReport("")
	report.Variables = map[string]*models.Variable{
		"DB_URL":      {Name: "DB_URL"},
		"API_KEY":     {Name: "API_KEY"},
		"DB_PASSWORD": {Name: "DB_PASSWORD"},
	}

	g := Build(scanResult, report)

	// Should have 2 services
	if len(g.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(g.Services))
	}

	// Should have 3 variables
	if len(g.Variables) != 3 {
		t.Errorf("expected 3 variables, got %d", len(g.Variables))
	}

	// Verify api service has correct variables
	apiSvc, ok := g.Services["api"]
	if !ok {
		t.Fatal("api service not found")
	}
	if len(apiSvc.Variables) != 2 {
		t.Errorf("api should have 2 variables, got %d", len(apiSvc.Variables))
	}
}

func TestBuild_SharedVariables(t *testing.T) {
	scanResult := models.NewScanResult()
	scanResult.Services = []models.ServiceDependency{
		{ServiceName: "api", Variables: []string{"SHARED_VAR", "API_ONLY"}},
		{ServiceName: "worker", Variables: []string{"SHARED_VAR", "WORKER_ONLY"}},
		{ServiceName: "scheduler", Variables: []string{"SHARED_VAR", "SCHEDULER_ONLY"}},
	}

	report := models.NewReport("")
	report.Variables = map[string]*models.Variable{
		"SHARED_VAR":     {Name: "SHARED_VAR"},
		"API_ONLY":       {Name: "API_ONLY"},
		"WORKER_ONLY":    {Name: "WORKER_ONLY"},
		"SCHEDULER_ONLY": {Name: "SCHEDULER_ONLY"},
	}

	g := Build(scanResult, report)

	// SHARED_VAR should be marked as shared
	sharedNode, ok := g.Variables["SHARED_VAR"]
	if !ok {
		t.Fatal("SHARED_VAR not found")
	}
	if !sharedNode.IsShared {
		t.Error("SHARED_VAR should be marked as shared")
	}
	if len(sharedNode.Services) != 3 {
		t.Errorf("SHARED_VAR should be in 3 services, got %d", len(sharedNode.Services))
	}

	// Should be in SharedVariables list
	if len(g.SharedVariables) != 1 {
		t.Errorf("expected 1 shared variable, got %d", len(g.SharedVariables))
	}
	if g.SharedVariables[0] != "SHARED_VAR" {
		t.Errorf("expected SHARED_VAR in shared list, got %s", g.SharedVariables[0])
	}

	// API_ONLY should not be shared
	apiOnlyNode := g.Variables["API_ONLY"]
	if apiOnlyNode.IsShared {
		t.Error("API_ONLY should not be marked as shared")
	}
}

func TestBuild_EmptyGraph(t *testing.T) {
	scanResult := models.NewScanResult()
	report := models.NewReport("")

	g := Build(scanResult, report)

	if len(g.Services) != 0 {
		t.Errorf("expected 0 services, got %d", len(g.Services))
	}
	if len(g.Variables) != 0 {
		t.Errorf("expected 0 variables, got %d", len(g.Variables))
	}
}

func TestFormatTree(t *testing.T) {
	g := models.NewDependencyGraph()
	g.Services["api"] = &models.ServiceNode{
		Name:      "api",
		Variables: []string{"DB_URL", "API_KEY"},
	}
	g.Services["db"] = &models.ServiceNode{
		Name:      "db",
		Variables: []string{"DB_PASSWORD"},
	}
	g.Variables["DB_URL"] = &models.VariableNode{Name: "DB_URL", Services: []string{"api"}}
	g.Variables["API_KEY"] = &models.VariableNode{Name: "API_KEY", Services: []string{"api"}}
	g.Variables["DB_PASSWORD"] = &models.VariableNode{Name: "DB_PASSWORD", Services: []string{"db"}}

	tree, err := FormatTree(g)
	if err != nil {
		t.Fatalf("FormatTree failed: %v", err)
	}

	// Should contain service names
	if !strings.Contains(tree, "api") {
		t.Error("tree should contain 'api'")
	}
	if !strings.Contains(tree, "db") {
		t.Error("tree should contain 'db'")
	}

	// Should contain variable names
	if !strings.Contains(tree, "DB_URL") {
		t.Error("tree should contain 'DB_URL'")
	}
}

func TestFormatTree_SharedVariables(t *testing.T) {
	g := models.NewDependencyGraph()
	g.Services["api"] = &models.ServiceNode{Name: "api", Variables: []string{"SHARED"}}
	g.Services["worker"] = &models.ServiceNode{Name: "worker", Variables: []string{"SHARED"}}
	g.Variables["SHARED"] = &models.VariableNode{
		Name:     "SHARED",
		Services: []string{"api", "worker"},
		IsShared: true,
	}
	g.SharedVariables = []string{"SHARED"}

	tree, err := FormatTree(g)
	if err != nil {
		t.Fatalf("FormatTree failed: %v", err)
	}

	// Should indicate shared status
	if !strings.Contains(tree, "(shared)") {
		t.Error("tree should indicate shared variables")
	}
}

func TestFormatDOT(t *testing.T) {
	g := models.NewDependencyGraph()
	g.Services["api"] = &models.ServiceNode{
		Name:      "api",
		Variables: []string{"DB_URL"},
	}
	g.Variables["DB_URL"] = &models.VariableNode{Name: "DB_URL", Services: []string{"api"}}

	dot, err := FormatDOT(g)
	if err != nil {
		t.Fatalf("FormatDOT failed: %v", err)
	}

	// Should be a valid DOT graph
	if !strings.Contains(dot, "digraph envgraph") {
		t.Error("should contain 'digraph envgraph'")
	}
	if !strings.Contains(dot, "api") {
		t.Error("DOT should contain 'api' node")
	}
}

func TestFormatTree_ServiceWithEnvFiles(t *testing.T) {
	g := models.NewDependencyGraph()
	g.Services["api"] = &models.ServiceNode{
		Name:      "api",
		Variables: []string{"API_KEY"},
		EnvFiles:  []string{".env", ".env.local"},
	}
	g.Variables["API_KEY"] = &models.VariableNode{Name: "API_KEY", Services: []string{"api"}}

	tree, err := FormatTree(g)
	if err != nil {
		t.Fatalf("FormatTree failed: %v", err)
	}

	// Should show env files
	if !strings.Contains(tree, ".env") {
		t.Error("tree should show .env file")
	}
}
