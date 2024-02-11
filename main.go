package main

import (
	"fmt"
	"os"
	"path/filepath"

	jira "github.com/andygrunwald/go-jira"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/trivago/tgo/tcontainer"
	"gopkg.in/yaml.v3"
)

const maxWidth = 160

var (
	red    = lipgloss.AdaptiveColor{Light: "#FE5F86", Dark: "#FE5F86"}
	indigo = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	green  = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
)

type Styles struct {
	Base,
	HeaderText,
	Status,
	StatusHeader,
	Highlight,
	ErrorHeaderText,
	Help lipgloss.Style
}

func NewStyles(lg *lipgloss.Renderer) *Styles {
	s := Styles{}
	s.Base = lg.NewStyle().
		Padding(1, 4, 0, 1)
	s.HeaderText = lg.NewStyle().
		Foreground(indigo).
		Bold(true).
		Padding(0, 1, 0, 2)
	s.Status = lg.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(indigo).
		PaddingLeft(1).
		MarginTop(1)
	s.StatusHeader = lg.NewStyle().
		Foreground(green).
		Bold(true)
	s.Highlight = lg.NewStyle().
		Foreground(lipgloss.Color("212"))
	s.ErrorHeaderText = s.HeaderText.Copy().
		Foreground(red)
	s.Help = lg.NewStyle().
		Foreground(lipgloss.Color("240"))
	return &s
}

type state int

const (
	statusNormal state = iota
	stateDone
)

var (
	summary     string
	description string
)

type Config struct {
	JiraUrl     string            `yaml:"jira_url"`
	Username    string            `yaml:"username"`
	ApiKey      string            `yaml:"api_key"`
	CreateIssue CreateIssueConfig `yaml:"create_issue"`
}

type CreateIssueConfig struct {
	Project      string                `yaml:"project"`
	CustomFields tcontainer.MarshalMap `yaml:"custom_fields"`
}

type Model struct {
	state  state
	lg     *lipgloss.Renderer
	styles *Styles
	form   *huh.Form
	width  int
}

func NewModel() Model {
	m := Model{width: maxWidth}
	m.lg = lipgloss.DefaultRenderer()
	m.styles = NewStyles(m.lg)
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Summary:").Value(&summary),
			huh.NewText().Title("Description:").Value(&description),
		),
	).
		WithWidth(45).
		WithShowHelp(false).
		WithShowErrors(false)
	return m
}

func (m Model) Init() tea.Cmd {
	return m.form.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = min(msg.Width, maxWidth) - m.styles.Base.GetHorizontalFrameSize()
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	var cmds []tea.Cmd

	// Process the form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
		cmds = append(cmds, cmd)
	}

	if m.form.State == huh.StateCompleted {
		cmds = append(cmds, tea.Quit)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	s := m.styles

	switch m.form.State {
	default:

		errors := m.form.Errors()
		header := m.appBoundaryView("Create a JIRA Ticket")
		if len(errors) > 0 {
			header = m.appErrorBoundaryView(m.errorView())
		}

		footer := m.appBoundaryView(m.form.Help().ShortHelpView(m.form.KeyBinds()))
		if len(errors) > 0 {
			footer = m.appErrorBoundaryView("")
		}

		return s.Base.Render(header + "\n" + m.form.View() + "\n\n" + footer)
	}
}

func (m Model) errorView() string {
	var s string
	for _, err := range m.form.Errors() {
		s += err.Error()
	}
	return s
}

func (m Model) appBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		m.styles.HeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceForeground(indigo),
	)
}

func (m Model) appErrorBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		m.styles.ErrorHeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceForeground(red),
	)
}

func main() {
	dirname, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Oh no:", err)
		os.Exit(1)
	}

	f, err := os.ReadFile(filepath.Join(dirname, ".config", "lazyjira", "config.yaml"))
	if err != nil {
		fmt.Println("Oh no:", err)
		os.Exit(1)
	}

	var c Config

	if err := yaml.Unmarshal(f, &c); err != nil {
		fmt.Println("Oh no:", err)
		os.Exit(1)
	}

	model := NewModel()
	_, err2 := tea.NewProgram(model).Run()
	if err2 != nil {
		fmt.Println("Oh no:", err2)
		os.Exit(1)
	}

	tp := jira.BasicAuthTransport{Username: c.Username, Password: c.ApiKey}
	jiraClient, _ := jira.NewClient(tp.Client(), c.JiraUrl)

	i := jira.Issue{
		Fields: &jira.IssueFields{
			Description: description,
			Type: jira.IssueType{
				Name: "Bug",
			},
			Project: jira.Project{
				Key: c.CreateIssue.Project,
			},
			Summary:  summary,
			Unknowns: c.CreateIssue.CustomFields,
		},
	}

	issue, _, err := jiraClient.Issue.Create(&i)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s: %v\n", issue.Key, issue.Self)
}
