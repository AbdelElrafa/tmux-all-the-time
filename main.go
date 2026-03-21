package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/abdelelrafa/tmux-all-the-time/internal/tmux"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type actionKind int

const (
	noAction actionKind = iota
	attachAction
	createAction
	continueAction
)

type action struct {
	kind        actionKind
	sessionName string
}

type option struct {
	kind        actionKind
	sessionName string
	label       string
}

type sessionsLoadedMsg struct {
	sessions []tmux.Session
	err      error
}

type model struct {
	sessions      []tmux.Session
	cursor        int
	input         textinput.Model
	loading       bool
	message       string
	messageIsErr  bool
	pendingAction action
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	rowStyle   = lipgloss.NewStyle().PaddingLeft(2)
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	goodStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	badStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func initialModel() model {
	input := textinput.New()
	input.Placeholder = "session-name"
	input.CharLimit = 64
	input.Prompt = "> "
	input.Focus()

	return model{
		loading: true,
		input:   input,
	}
}

func loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := tmux.ListSessions()
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (m model) Init() tea.Cmd {
	return loadSessionsCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsLoadedMsg:
		m.loading = false
		if msg.err == nil {
			m.sessions = msg.sessions
			m.message = ""
			m.messageIsErr = false
			m.clampCursor()
			return m, nil
		}

		if errors.Is(msg.err, tmux.ErrNoSessions) {
			m.sessions = nil
			m.message = "No tmux sessions found yet. Type a name to create one or continue without tmux."
			m.messageIsErr = false
			m.clampCursor()
			return m, nil
		}

		m.message = msg.err.Error()
		m.messageIsErr = true
		m.clampCursor()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "up", "shift+tab", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "tab", "ctrl+n":
			if m.cursor < len(m.options())-1 {
				m.cursor++
			}
			return m, nil
		case "ctrl+r":
			m.loading = true
			m.message = ""
			m.messageIsErr = false
			return m, loadSessionsCmd()
		case "enter":
			options := m.options()
			if len(options) == 0 {
				return m, nil
			}

			selected := options[m.cursor]
			m.pendingAction = action{
				kind:        selected.kind,
				sessionName: selected.sessionName,
			}
			return m, tea.Quit
		}

		previous := m.input.Value()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if m.input.Value() != previous {
			m.cursor = 0
			m.clampCursor()
		}
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	var body strings.Builder

	body.WriteString(titleStyle.Render("tmux-all-the-time"))
	body.WriteString("\n")
	body.WriteString(helpStyle.Render("Type a session name, attach to a match, create a new one, or continue without tmux."))
	body.WriteString("\n\n")

	body.WriteString("Session name\n")
	body.WriteString(m.input.View())
	body.WriteString("\n\n")

	if m.loading {
		body.WriteString("Loading tmux sessions...")
		return pad(body.String())
	}

	if m.message != "" {
		style := goodStyle
		if m.messageIsErr {
			style = badStyle
		}
		body.WriteString(style.Render(m.message))
		body.WriteString("\n\n")
	}

	if validation := currentValidationMessage(strings.TrimSpace(m.input.Value()), m.sessions); validation != "" {
		body.WriteString(helpStyle.Render(validation))
		body.WriteString("\n\n")
	}

	body.WriteString(m.renderOptions())
	body.WriteString("\n\n")
	body.WriteString(helpStyle.Render("type to filter  enter select  tab/arrows move  ctrl+r refresh  ctrl+c quit"))

	return pad(body.String())
}

func (m model) renderOptions() string {
	options := m.options()
	lines := make([]string, 0, len(options))

	for i, option := range options {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		lines = append(lines, rowStyle.Render(fmt.Sprintf("%s %s", cursor, option.label)))
	}

	return strings.Join(lines, "\n")
}

func (m model) options() []option {
	query := strings.TrimSpace(m.input.Value())
	options := make([]option, 0, len(m.sessions)+2)

	for _, session := range filterSessions(m.sessions, query) {
		status := "detached"
		if session.Attached {
			status = "attached"
		}

		options = append(options, option{
			kind:        attachAction,
			sessionName: session.Name,
			label:       fmt.Sprintf("Attach to %s (%d windows, %s)", session.Name, session.Windows, status),
		})
	}

	if query != "" && sessionNamePattern.MatchString(query) && !sessionExists(m.sessions, query) {
		options = append(options, option{
			kind:        createAction,
			sessionName: query,
			label:       fmt.Sprintf("Create new session %q", query),
		})
	}

	options = append(options, option{
		kind:  continueAction,
		label: "Continue without tmux",
	})

	return options
}

func (m *model) clampCursor() {
	options := m.options()
	if len(options) == 0 {
		m.cursor = 0
		return
	}

	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(options) {
		m.cursor = len(options) - 1
	}
}

func filterSessions(sessions []tmux.Session, query string) []tmux.Session {
	if query == "" {
		return sessions
	}

	lowerQuery := strings.ToLower(query)

	type rankedSession struct {
		session tmux.Session
		score   int
	}

	ranked := make([]rankedSession, 0, len(sessions))
	for _, session := range sessions {
		lowerName := strings.ToLower(session.Name)

		switch {
		case lowerName == lowerQuery:
			ranked = append(ranked, rankedSession{session: session, score: 0})
		case strings.HasPrefix(lowerName, lowerQuery):
			ranked = append(ranked, rankedSession{session: session, score: 1})
		case strings.Contains(lowerName, lowerQuery):
			ranked = append(ranked, rankedSession{session: session, score: 2})
		}
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score < ranked[j].score
		}

		return strings.ToLower(ranked[i].session.Name) < strings.ToLower(ranked[j].session.Name)
	})

	filtered := make([]tmux.Session, 0, len(ranked))
	for _, candidate := range ranked {
		filtered = append(filtered, candidate.session)
	}

	return filtered
}

func sessionExists(sessions []tmux.Session, name string) bool {
	for _, session := range sessions {
		if strings.EqualFold(session.Name, name) {
			return true
		}
	}

	return false
}

func currentValidationMessage(query string, sessions []tmux.Session) string {
	switch {
	case query == "":
		return "Leave the field empty to browse all sessions, or type a new name to create one."
	case !sessionNamePattern.MatchString(query):
		return "New session names can use only letters, numbers, hyphens, and underscores."
	case sessionExists(sessions, query):
		return fmt.Sprintf("Press Enter to attach to the existing session %q.", query)
	default:
		return fmt.Sprintf("Press Enter to create a new session named %q if that row is selected.", query)
	}
}

func pad(content string) string {
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func main() {
	m := initialModel()
	program := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "run TUI: %v\n", err)
		os.Exit(1)
	}

	result, ok := finalModel.(model)
	if !ok {
		fmt.Fprintln(os.Stderr, "unexpected final model type")
		os.Exit(1)
	}

	switch result.pendingAction.kind {
	case attachAction:
		if err := tmux.Attach(result.pendingAction.sessionName); err != nil {
			fmt.Fprintf(os.Stderr, "attach session %q: %v\n", result.pendingAction.sessionName, err)
			os.Exit(1)
		}
	case createAction:
		if err := tmux.CreateAndAttach(result.pendingAction.sessionName); err != nil {
			fmt.Fprintf(os.Stderr, "create session %q: %v\n", result.pendingAction.sessionName, err)
			os.Exit(1)
		}
	case continueAction, noAction:
		return
	}
}
