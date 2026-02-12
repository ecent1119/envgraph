// Package tui provides an interactive terminal UI for envgraph
package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stackgen-cli/envgraph/internal/models"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			Padding(0, 1)

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170"))

	paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(1, 0)

	statusOK = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			SetString("✓")

	statusError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			SetString("✗")

	statusWarning = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			SetString("○")
)

// Run starts the interactive TUI
func Run(report *models.Report, graph *models.DependencyGraph) error {
	p := tea.NewProgram(initialModel(report, graph), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Model is the main TUI model
type Model struct {
	report    *models.Report
	graph     *models.DependencyGraph
	list      list.Model
	viewport  viewport.Model
	ready     bool
	selected  *variableItem
	width     int
	height    int
	showGraph bool
}

// variableItem implements list.Item for variables
type variableItem struct {
	name     string
	variable *models.Variable
}

func (i variableItem) Title() string {
	status := statusOK.String()
	if !i.variable.IsDefined() {
		status = statusError.String()
	} else if !i.variable.IsUsed() {
		status = statusWarning.String()
	}
	return fmt.Sprintf("%s %s", status, i.name)
}

func (i variableItem) Description() string {
	parts := []string{}
	if len(i.variable.DefinedIn) > 0 {
		parts = append(parts, fmt.Sprintf("defined: %d", len(i.variable.DefinedIn)))
	}
	if len(i.variable.UsedIn) > 0 {
		parts = append(parts, fmt.Sprintf("used: %d", len(i.variable.UsedIn)))
	}
	if i.variable.EffectiveValue != "" && len(i.variable.EffectiveValue) < 30 {
		parts = append(parts, fmt.Sprintf("value: %s", i.variable.EffectiveValue))
	}
	return strings.Join(parts, " | ")
}

func (i variableItem) FilterValue() string {
	return i.name
}

func initialModel(report *models.Report, graph *models.DependencyGraph) Model {
	// Create list items
	items := make([]list.Item, 0, len(report.Variables))
	names := make([]string, 0, len(report.Variables))
	for name := range report.Variables {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		items = append(items, variableItem{
			name:     name,
			variable: report.Variables[name],
		})
	}

	// Create list
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Environment Variables"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle

	return Model{
		report:    report,
		graph:     graph,
		list:      l,
		showGraph: false,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(variableItem); ok {
				m.selected = &item
				m.viewport.SetContent(m.renderVariableDetail(item))
			}
		case "esc":
			m.selected = nil
		case "g":
			m.showGraph = !m.showGraph
			if m.showGraph && m.graph != nil {
				m.viewport.SetContent(m.renderGraph())
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 4
		footerHeight := 3

		if !m.ready {
			m.viewport = viewport.New(msg.Width/2, msg.Height-headerHeight-footerHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width / 2
			m.viewport.Height = msg.Height - headerHeight - footerHeight
		}

		m.list.SetSize(msg.Width/2-2, msg.Height-headerHeight-footerHeight)
	}

	var cmds []tea.Cmd

	if m.selected != nil || m.showGraph {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Summary header
	header := m.renderHeader()

	// Left side: list
	leftPane := m.list.View()

	// Right side: detail or graph
	rightPane := ""
	if m.showGraph && m.graph != nil {
		rightPane = m.viewport.View()
	} else if m.selected != nil {
		rightPane = m.viewport.View()
	} else {
		rightPane = m.renderSummary()
	}

	// Combine panes
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " │ ", rightPane)

	// Footer
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m Model) renderHeader() string {
	stats := m.report.Stats
	return titleStyle.Render(fmt.Sprintf(
		"envgraph │ Variables: %d │ Errors: %d │ Warnings: %d",
		stats.TotalVariables,
		stats.ErrorCount,
		stats.WarningCount,
	))
}

func (m Model) renderFooter() string {
	return helpStyle.Render("↑/↓: navigate │ /: filter │ enter: select │ g: graph │ q: quit")
}

func (m Model) renderSummary() string {
	var sb strings.Builder
	stats := m.report.Stats

	sb.WriteString(titleStyle.Render("Summary"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Total variables:    %d\n", stats.TotalVariables))
	sb.WriteString(fmt.Sprintf("Defined:            %d\n", stats.DefinedVariables))
	sb.WriteString(fmt.Sprintf("Used:               %d\n", stats.UsedVariables))
	sb.WriteString(fmt.Sprintf("Undefined (errors): %d\n", stats.UndefinedCount))
	sb.WriteString(fmt.Sprintf("Unused (warnings):  %d\n", stats.UnusedCount))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Files scanned:\n"))
	sb.WriteString(fmt.Sprintf("  • Env files:     %d\n", stats.EnvFilesScanned))
	sb.WriteString(fmt.Sprintf("  • Compose files: %d\n", stats.ComposeFilesScanned))
	sb.WriteString(fmt.Sprintf("  • Source files:  %d\n", stats.SourceFilesScanned))

	if len(m.report.Issues) > 0 {
		sb.WriteString("\n")
		sb.WriteString(titleStyle.Render("Issues"))
		sb.WriteString("\n\n")
		for _, issue := range m.report.Issues {
			icon := "ℹ️"
			switch issue.Severity {
			case models.SeverityError:
				icon = "❌"
			case models.SeverityWarning:
				icon = "⚠️"
			}
			sb.WriteString(fmt.Sprintf("%s %s: %s\n", icon, issue.VariableName, issue.Type.String()))
		}
	}

	return sb.String()
}

func (m Model) renderVariableDetail(item variableItem) string {
	var sb strings.Builder
	v := item.variable

	sb.WriteString(titleStyle.Render(item.name))
	sb.WriteString("\n\n")

	// Status
	status := "✓ OK"
	if !v.IsDefined() {
		status = "✗ Undefined"
	} else if !v.IsUsed() {
		status = "○ Unused"
	}
	sb.WriteString(fmt.Sprintf("Status: %s\n\n", status))

	// Value
	if v.HasValue {
		displayValue := v.EffectiveValue
		if len(displayValue) > 50 {
			displayValue = displayValue[:50] + "..."
		}
		sb.WriteString(fmt.Sprintf("Value: %s\n", displayValue))
		sb.WriteString(fmt.Sprintf("Layer: %s\n\n", v.EffectiveLayer.String()))
	}

	// Definitions
	if len(v.DefinedIn) > 0 {
		sb.WriteString("Defined in:\n")
		for _, src := range v.DefinedIn {
			loc := src.FilePath
			if src.LineNumber > 0 {
				loc = fmt.Sprintf("%s:%d", src.FilePath, src.LineNumber)
			}
			sb.WriteString(fmt.Sprintf("  • %s (%s)\n", loc, src.Layer.String()))
		}
		sb.WriteString("\n")
	}

	// Usages
	if len(v.UsedIn) > 0 {
		sb.WriteString("Used in:\n")
		for _, src := range v.UsedIn {
			loc := src.FilePath
			if src.LineNumber > 0 {
				loc = fmt.Sprintf("%s:%d", src.FilePath, src.LineNumber)
			}
			extra := ""
			if src.Service != "" {
				extra = fmt.Sprintf(" (service: %s)", src.Service)
			}
			sb.WriteString(fmt.Sprintf("  • %s%s\n", loc, extra))
		}
	}

	return sb.String()
}

func (m Model) renderGraph() string {
	if m.graph == nil {
		return "No graph available"
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Service Dependencies"))
	sb.WriteString("\n\n")

	// Services and their variables
	for name, svc := range m.graph.Services {
		sb.WriteString(fmt.Sprintf("📦 %s\n", name))
		for _, varName := range svc.Variables {
			shared := ""
			if node, ok := m.graph.Variables[varName]; ok && node.IsShared {
				shared = " (shared)"
			}
			sb.WriteString(fmt.Sprintf("  └── 🔑 %s%s\n", varName, shared))
		}
		sb.WriteString("\n")
	}

	// Shared variables
	if len(m.graph.SharedVariables) > 0 {
		sb.WriteString(titleStyle.Render("Shared Variables"))
		sb.WriteString("\n\n")
		for _, varName := range m.graph.SharedVariables {
			if node, ok := m.graph.Variables[varName]; ok {
				sb.WriteString(fmt.Sprintf("🔗 %s\n", varName))
				sb.WriteString(fmt.Sprintf("   Used by: %s\n", strings.Join(node.Services, ", ")))
			}
		}
	}

	return sb.String()
}
