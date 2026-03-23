package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/abdelelrafa/tmux-all-the-time/internal/tmux"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

var sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var (
	oscSequencePattern = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	csiSequencePattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
)

type actionKind int

const (
	noAction actionKind = iota
	attachAction
	attachWindowAction
	createAction
	continueAction
)

type action struct {
	kind        actionKind
	sessionName string
	windowIndex int
}

type option struct {
	kind        actionKind
	sessionName string
	windowIndex int
	label       string
	indent      int
	matched     bool
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
	width         int
	height        int
	message       string
	messageIsErr  bool
	pendingAction action
}

type previewBlock struct {
	meta       []string
	bodyRaw    string
	bodyWidth  int
	bodyHeight int
}

var (
	titleStyle           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff9e64"))
	rowStyle             = lipgloss.NewStyle().PaddingLeft(1)
	helpStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#7c829d"))
	goodStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
	badStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
	selectedWindowStyle  = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#0f111a")).Background(lipgloss.Color("#7aa2f7")).Bold(true)
	selectedSessionStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#0f111a")).Background(lipgloss.Color("#e0af68")).Bold(true)
	selectedCreateStyle  = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#0f111a")).Background(lipgloss.Color("#9ece6a")).Bold(true)
	selectedPlainStyle   = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#0f111a")).Background(lipgloss.Color("#c0caf5")).Bold(true)
	matchedWindowStyle   = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#73daca")).Bold(true)
	matchedSessionStyle  = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#e0af68")).Bold(true)
	windowRowStyle       = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#9aa5ce"))
	sessionRowStyle      = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#c0caf5"))
	createRowStyle       = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#9ece6a"))
	plainRowStyle        = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#a9b1d6"))
	previewHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	previewMetaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#c0caf5"))
	previewLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7dcfff"))
	previewMutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
)

func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
}

func initialModel() model {
	input := textinput.New()
	input.Placeholder = "session-or-window"
	input.CharLimit = 64
	input.Prompt = ""
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionsLoadedMsg:
		m.loading = false
		if msg.err == nil {
			m.sessions = msg.sessions
			m.message = ""
			m.messageIsErr = false
			if strings.TrimSpace(m.input.Value()) == "" {
				m.cursor = m.defaultCursor(m.options())
			}
			m.clampCursor()
			return m, nil
		}

		if errors.Is(msg.err, tmux.ErrNoSessions) {
			m.sessions = nil
			m.message = "No tmux sessions found yet. Type a name to create one or continue without tmux."
			m.messageIsErr = false
			m.cursor = 0
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
				windowIndex: selected.windowIndex,
			}
			return m, tea.Quit
		}

		previous := m.input.Value()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if m.input.Value() != previous {
			m.resetCursorForQuery()
			m.clampCursor()
		}
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		titleStyle.Render("tmux-all-the-time"),
		helpStyle.Render("  enter select  tab/arrows move  ctrl+r reload  ctrl+c quit"),
	)
	searchLine := lipgloss.JoinHorizontal(
		lipgloss.Center,
		previewLabelStyle.Render("> "),
		m.input.View(),
	)

	sections := []string{header, searchLine}

	if m.loading {
		sections = append(sections, "Loading tmux sessions...")
		return pad(strings.Join(sections, "\n"))
	}

	if m.message != "" {
		style := goodStyle
		if m.messageIsErr {
			style = badStyle
		}
		if m.messageIsErr {
			sections = append(sections, style.Render(m.message))
		}
	}

	sections = append(sections, m.renderPanels())

	return pad(strings.Join(sections, "\n"))
}

func (m model) renderPanels() string {
	contentHeight := m.panelContentHeight()

	if m.width > 0 {
		panelWidth := max(m.width-8, 60)
		if panelWidth >= 72 {
			leftWidth := sidebarWidth(panelWidth)
			rightWidth := panelWidth - leftWidth - 1
			leftLines := strings.Split(m.renderOptions(contentHeight, leftWidth), "\n")
			rightLines := strings.Split(m.renderPreview(contentHeight, rightWidth), "\n")

			leftPad := lipgloss.NewStyle().Width(leftWidth)
			rightPad := lipgloss.NewStyle().Width(rightWidth)
			rows := make([]string, 0, max(len(leftLines), len(rightLines)))

			for i := 0; i < max(len(leftLines), len(rightLines)); i++ {
				left := " "
				right := " "
				if i < len(leftLines) {
					left = leftLines[i]
				}
				if i < len(rightLines) {
					right = rightLines[i]
				}

				rows = append(rows, leftPad.Render(left)+" "+helpStyle.Render("│")+" "+rightPad.Render(right))
			}

			return strings.Join(rows, "\n")
		}
	}

	leftContent := m.renderOptions(contentHeight, 60)
	rightContent := m.renderPreview(contentHeight, 60)
	return leftContent + "\n" + helpStyle.Render(strings.Repeat("─", 40)) + "\n" + rightContent
}

func sidebarWidth(panelWidth int) int {
	const (
		minSidebarWidth = 26
		maxSidebarWidth = 38
		minPreviewWidth = 40
	)

	if panelWidth <= minSidebarWidth+1+minPreviewWidth {
		return minSidebarWidth
	}

	preferred := panelWidth * 32 / 100
	return min(max(preferred, minSidebarWidth), min(maxSidebarWidth, panelWidth-1-minPreviewWidth))
}

func (m model) panelContentHeight() int {
	if m.height <= 0 {
		return 14
	}

	return max(m.height-8, 10)
}

func (m model) renderOptions(contentHeight, contentWidth int) string {
	options := m.options()
	lines := []string{previewHeaderStyle.Render("Sessions")}

	for i, option := range options {
		lines = append(lines, rowStyle.Render(renderOptionLabel(option, m.cursor == i, contentWidth)))
	}

	return strings.Join(fitLines(lines, contentHeight), "\n")
}

func (m model) renderPreview(contentHeight, contentWidth int) string {
	block := previewBlock{
		meta: []string{previewHeaderStyle.Render("Preview")},
	}

	option, ok := m.selectedOption()
	if !ok {
		block.meta = append(block.meta, helpStyle.Render("No selection."))
		return strings.Join(fitPreviewBlock(block, contentWidth, contentHeight), "\n")
	}

	switch option.kind {
	case attachAction:
		session, ok := m.findSession(option.sessionName)
		if !ok {
			block.meta = append(block.meta, badStyle.Render("Selected session no longer exists."))
			return strings.Join(fitPreviewBlock(block, contentWidth, contentHeight), "\n")
		}

		block = m.renderSessionPreview(session, contentWidth)
	case attachWindowAction:
		window, session, ok := m.findWindow(option.sessionName, option.windowIndex)
		if !ok {
			block.meta = append(block.meta, badStyle.Render("Selected window no longer exists."))
			return strings.Join(fitPreviewBlock(block, contentWidth, contentHeight), "\n")
		}

		block = m.renderWindowPreview(session, window, contentWidth)
	case createAction:
		block.meta = append(block.meta,
			previewLine(contentWidth, "New session: "+option.sessionName),
			helpStyle.Render("Press Enter to create and attach to this new tmux session."),
		)
	case continueAction:
		block.meta = append(block.meta,
			previewLine(contentWidth, "Plain shell"),
			helpStyle.Render("Press Enter to continue without attaching to tmux."),
		)
	default:
		block.meta = append(block.meta, helpStyle.Render("No preview available."))
	}

	return strings.Join(fitPreviewBlock(block, contentWidth, contentHeight), "\n")
}

func (m model) renderSessionPreview(session tmux.Session, contentWidth int) previewBlock {
	block := previewBlock{
		meta: []string{previewHeaderStyle.Render("Preview")},
	}
	block.meta = append(block.meta, compactPreviewLineWidth(contentWidth,
		"session", session.Name,
		"windows", fmt.Sprintf("%d", session.WindowCount),
	))

	activeWindow := activeWindowForSession(session)
	if activeWindow == nil {
		block.meta = append(block.meta, helpStyle.Render("No windows found in this session."))
		return block
	}

	block.meta = append(block.meta, compactPreviewLineWidth(contentWidth,
		"active", fmt.Sprintf("%d:%s", activeWindow.Index, activeWindow.Name),
		"cmd", fallbackValue(activeWindow.CurrentCommand, "-"),
		"state", windowState(*activeWindow),
	))
	if activeWindow.CurrentPath != "" {
		block.meta = append(block.meta, pathPreviewLine(contentWidth, "pwd", activeWindow.CurrentPath))
	}
	if title := compactPaneTitle(activeWindow.PaneTitle); title != "" {
		block.meta = append(block.meta, compactPreviewLineWidth(contentWidth, "title", title))
	}
	block.meta = append(block.meta, previewLabelStyle.Render("Captured output"))
	block.bodyRaw = activeWindow.Preview
	block.bodyWidth = activeWindow.PaneWidth
	block.bodyHeight = activeWindow.PaneHeight
	return block
}

func (m model) renderWindowPreview(session tmux.Session, window tmux.Window, contentWidth int) previewBlock {
	block := previewBlock{
		meta: []string{
			previewHeaderStyle.Render("Preview"),
			compactPreviewLineWidth(contentWidth,
				"session", session.Name,
				"window", fmt.Sprintf("%d:%s", window.Index, window.Name),
			),
			compactPreviewLineWidth(contentWidth,
				"cmd", fallbackValue(window.CurrentCommand, "-"),
				"state", windowState(window),
			),
		},
	}
	if window.CurrentPath != "" {
		block.meta = append(block.meta, pathPreviewLine(contentWidth, "pwd", window.CurrentPath))
	}
	if title := compactPaneTitle(window.PaneTitle); title != "" {
		block.meta = append(block.meta, compactPreviewLineWidth(contentWidth, "title", title))
	}
	block.meta = append(block.meta, previewLabelStyle.Render("Captured output"))
	block.bodyRaw = window.Preview
	block.bodyWidth = window.PaneWidth
	block.bodyHeight = window.PaneHeight
	return block
}

func renderPreviewLines(raw string, paneWidth, paneHeight, contentWidth, contentHeight int) []string {
	raw = sanitizePreviewANSI(raw)
	if strings.TrimSpace(ansi.Strip(raw)) == "" {
		return []string{helpStyle.Render("No captured text for this pane yet.")}
	}
	if contentWidth <= 0 || contentHeight <= 0 {
		return nil
	}
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 {
		return []string{helpStyle.Render("No captured text for this pane yet.")}
	}

	rowStart, rowEnd := previewRowRange(lines, contentHeight)
	colStart := previewColumnStart(paneWidth, contentWidth)

	rendered := make([]string, 0, rowEnd-rowStart)
	for _, line := range lines[rowStart:rowEnd] {
		rendered = append(rendered, previewContentViewport(line, colStart, contentWidth))
	}

	return rendered
}

func previewRowRange(lines []string, contentHeight int) (int, int) {
	if len(lines) == 0 {
		return 0, 0
	}

	lastTextRow := -1
	for i, line := range lines {
		if strings.TrimSpace(ansi.Strip(line)) != "" {
			lastTextRow = i
		}
	}

	if lastTextRow < 0 {
		rowStart := max(0, len(lines)-contentHeight)
		return rowStart, min(len(lines), rowStart+contentHeight)
	}

	rowEnd := min(len(lines), lastTextRow+1)
	rowStart := max(0, rowEnd-contentHeight)
	return rowStart, rowEnd
}

func previewColumnStart(paneWidth, contentWidth int) int {
	if paneWidth <= contentWidth {
		return 0
	}

	return 0
}

func previewContentViewport(line string, start, width int) string {
	if width <= 0 {
		return ""
	}

	return ansi.CutWc(line, start, width) + ansi.ResetStyle
}

func sanitizePreviewANSI(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")
	s = oscSequencePattern.ReplaceAllString(s, "")
	s = csiSequencePattern.ReplaceAllStringFunc(s, func(seq string) string {
		if strings.HasSuffix(seq, "m") {
			return seq
		}
		return ""
	})

	var b strings.Builder
	b.Grow(len(s))
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			s = s[size:]
			continue
		}
		if r == '\x1b' || r == '\n' || r == '\t' || r >= ' ' {
			b.WriteRune(r)
		}
		s = s[size:]
	}

	return b.String()
}

func fitPreviewBlock(block previewBlock, width, height int) []string {
	lines := append([]string{}, block.meta...)
	if strings.TrimSpace(ansi.Strip(block.bodyRaw)) != "" && height > len(lines) {
		available := height - len(lines)
		bodyLines := renderPreviewLines(block.bodyRaw, block.bodyWidth, block.bodyHeight, width, available)
		if len(bodyLines) > available {
			bodyLines = bodyLines[len(bodyLines)-available:]
		}
		lines = append(lines, bodyLines...)
	}

	return fitLines(lines, height)
}

func fitLines(lines []string, height int) []string {
	if height <= 0 {
		return lines
	}

	if len(lines) > height {
		if height == 1 {
			return []string{lines[0]}
		}

		fitted := append([]string{}, lines[:height-1]...)
		fitted = append(fitted, previewMutedStyle.Render("..."))
		return fitted
	}

	for len(lines) < height {
		lines = append(lines, " ")
	}

	return lines
}

func truncateText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	if runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return runewidth.Truncate(s, maxWidth, "")
	}

	return runewidth.Truncate(s, maxWidth, "...")
}

func compactPreviewLineWidth(width int, parts ...string) string {
	chunks := make([]string, 0, len(parts)/2)
	for i := 0; i+1 < len(parts); i += 2 {
		label := previewMutedStyle.Render(parts[i] + " ")
		value := previewMetaStyle.Render(parts[i+1])
		chunks = append(chunks, label+value)
	}

	return clipStyledLine(strings.Join(chunks, previewMutedStyle.Render("  |  ")), width)
}

func pathPreviewLine(width int, label, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return compactPreviewLineWidth(width, label, "-")
	}

	displayPath := strings.Replace(path, os.Getenv("HOME"), "~", 1)
	labelText := label + " "
	available := max(width-len(labelText), 8)
	if runewidth.StringWidth(displayPath) > available {
		displayPath = runewidth.TruncateLeft(displayPath, available, "...")
	}

	return previewMutedStyle.Render(labelText) + previewMetaStyle.Render(displayPath)
}

func previewLine(width int, text string) string {
	if width <= 0 {
		return ""
	}
	if width <= 3 {
		return ansi.TruncateWc(text, width, "")
	}

	return ansi.TruncateWc(text, width, "...")
}

func previewContentLine(width int, text string) string {
	if width <= 0 {
		return ""
	}

	return ansi.CutWc(text, 0, width)
}

func clipStyledLine(line string, width int) string {
	if width <= 0 || lipgloss.Width(line) <= width {
		return line
	}
	if width <= 3 {
		return ansi.TruncateWc(line, width, "")
	}

	return ansi.TruncateWc(line, width, "...")
}

func compactPaneTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.TrimSuffix(title, ".local")
	if title == "" {
		return ""
	}

	return title
}

func fallbackValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}

	return value
}

func windowState(window tmux.Window) string {
	if window.Active {
		return "active"
	}

	return "inactive"
}

func (m model) options() []option {
	query := strings.TrimSpace(m.input.Value())
	lowerQuery := strings.ToLower(query)
	options := make([]option, 0, len(m.sessions)*3+2)

	for _, session := range filterSessions(m.sessions, query) {
		status := "detached"
		if session.Attached {
			status = "attached"
		}

		options = append(options, option{
			kind:        attachAction,
			sessionName: session.Name,
			label:       fmt.Sprintf("%s (%d windows, %s)", session.Name, session.WindowCount, status),
			matched:     query != "" && sessionNameMatches(session.Name, lowerQuery),
		})

		for _, window := range session.Windows {
			options = append(options, option{
				kind:        attachWindowAction,
				sessionName: session.Name,
				windowIndex: window.Index,
				label:       formatWindowLabel(window),
				indent:      1,
				matched:     query != "" && windowMatchesQuery(session.Name, window, lowerQuery),
			})
		}
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

func (m *model) resetCursorForQuery() {
	query := strings.TrimSpace(m.input.Value())
	options := m.options()

	if query == "" || len(options) == 0 {
		m.cursor = m.defaultCursor(options)
		return
	}

	for i, option := range options {
		if option.kind == attachWindowAction && option.matched {
			m.cursor = i
			return
		}
	}

	for i, option := range options {
		if option.kind == attachAction && option.matched {
			m.cursor = i
			return
		}
	}

	m.cursor = 0
}

func (m model) defaultCursor(options []option) int {
	bestIndex := -1
	var bestWindowActivity int64 = -1
	var bestSessionActivity int64 = -1

	for i, option := range options {
		if option.kind != attachWindowAction {
			continue
		}

		window, session, ok := m.findWindow(option.sessionName, option.windowIndex)
		if !ok {
			continue
		}

		if window.Activity > bestWindowActivity || (window.Activity == bestWindowActivity && session.Activity > bestSessionActivity) {
			bestIndex = i
			bestWindowActivity = window.Activity
			bestSessionActivity = session.Activity
		}
	}

	if bestIndex >= 0 {
		return bestIndex
	}

	bestIndex = 0
	bestSessionActivity = -1
	for i, option := range options {
		if option.kind != attachAction {
			continue
		}

		session, ok := m.findSession(option.sessionName)
		if !ok {
			continue
		}

		if session.Activity > bestSessionActivity {
			bestIndex = i
			bestSessionActivity = session.Activity
		}
	}

	return bestIndex
}

func (m model) selectedOption() (option, bool) {
	options := m.options()
	if len(options) == 0 || m.cursor < 0 || m.cursor >= len(options) {
		return option{}, false
	}

	return options[m.cursor], true
}

func (m model) findSession(name string) (tmux.Session, bool) {
	for _, session := range m.sessions {
		if session.Name == name {
			return session, true
		}
	}

	return tmux.Session{}, false
}

func (m model) findWindow(sessionName string, windowIndex int) (tmux.Window, tmux.Session, bool) {
	for _, session := range m.sessions {
		if session.Name != sessionName {
			continue
		}

		for _, window := range session.Windows {
			if window.Index == windowIndex {
				return window, session, true
			}
		}
	}

	return tmux.Window{}, tmux.Session{}, false
}

func activeWindowForSession(session tmux.Session) *tmux.Window {
	for i := range session.Windows {
		if session.Windows[i].Active {
			return &session.Windows[i]
		}
	}
	if len(session.Windows) == 0 {
		return nil
	}

	return &session.Windows[0]
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
		score, ok := sessionMatchScore(session, lowerQuery)
		if ok {
			ranked = append(ranked, rankedSession{session: session, score: score})
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

func sessionMatchScore(session tmux.Session, lowerQuery string) (int, bool) {
	switch {
	case sessionNameMatches(session.Name, lowerQuery):
		lowerName := strings.ToLower(session.Name)
		switch {
		case lowerName == lowerQuery:
			return 0, true
		case strings.HasPrefix(lowerName, lowerQuery):
			return 1, true
		default:
			return 2, true
		}
	}

	bestWindowScore := 99
	for _, window := range session.Windows {
		if score, ok := windowMatchScore(session.Name, window, lowerQuery); ok && score < bestWindowScore {
			bestWindowScore = score
		}
	}

	if bestWindowScore != 99 {
		return 3 + bestWindowScore, true
	}

	return 0, false
}

func sessionNameMatches(sessionName, lowerQuery string) bool {
	lowerName := strings.ToLower(sessionName)
	return lowerName == lowerQuery || strings.HasPrefix(lowerName, lowerQuery) || strings.Contains(lowerName, lowerQuery)
}

func windowMatchesQuery(sessionName string, window tmux.Window, lowerQuery string) bool {
	_, ok := windowMatchScore(sessionName, window, lowerQuery)
	return ok
}

func windowMatchScore(sessionName string, window tmux.Window, lowerQuery string) (int, bool) {
	candidates := []string{
		strings.ToLower(window.Name),
		strings.ToLower(fmt.Sprintf("%d", window.Index)),
		strings.ToLower(fmt.Sprintf("%s:%d", sessionName, window.Index)),
		strings.ToLower(fmt.Sprintf("%s:%s", sessionName, window.Name)),
		strings.ToLower(window.CurrentCommand),
	}

	bestScore := 99
	for _, candidate := range candidates {
		switch {
		case candidate == lowerQuery:
			if 0 < bestScore {
				bestScore = 0
			}
		case strings.HasPrefix(candidate, lowerQuery):
			if 1 < bestScore {
				bestScore = 1
			}
		case strings.Contains(candidate, lowerQuery):
			if 2 < bestScore {
				bestScore = 2
			}
		}
	}

	if bestScore == 99 {
		return 0, false
	}

	return bestScore, true
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
		return "The selected row previews on the right. Press Enter on an indented window row to open that exact window."
	case hasMatchingWindow(sessions, query):
		return "Matching windows stay nested under their session. The preview follows the highlighted row."
	case sessionExists(sessions, query):
		return fmt.Sprintf("Press Enter to attach to the existing session %q.", query)
	case !sessionNamePattern.MatchString(query):
		return "Search can include session or window names. New session names can use only letters, numbers, hyphens, and underscores."
	default:
		return fmt.Sprintf("Press Enter to create a new session named %q if that row is selected.", query)
	}
}

func hasMatchingWindow(sessions []tmux.Session, query string) bool {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery == "" {
		return false
	}

	for _, session := range sessions {
		for _, window := range session.Windows {
			if windowMatchesQuery(session.Name, window, lowerQuery) {
				return true
			}
		}
	}

	return false
}

func formatWindowLabel(window tmux.Window) string {
	marker := " "
	if window.Active {
		marker = "*"
	}

	suffix := window.Name
	if window.CurrentCommand != "" {
		suffix = fmt.Sprintf("%s [%s]", suffix, window.CurrentCommand)
	}

	return fmt.Sprintf("[%s] %d:%s", marker, window.Index, suffix)
}

func renderOptionLabel(option option, selected bool, contentWidth int) string {
	style := optionStyle(option, selected)
	prefix := "  "
	if selected {
		prefix = "> "
	}

	indent := strings.Repeat("  ", option.indent)
	return style.Render(truncateText(indent+prefix+option.label, max(contentWidth-4, 8)))
}

func optionStyle(option option, selected bool) lipgloss.Style {
	switch option.kind {
	case attachWindowAction:
		if selected {
			return selectedWindowStyle
		}
		if option.matched {
			return matchedWindowStyle
		}
		return windowRowStyle
	case attachAction:
		if selected {
			return selectedSessionStyle
		}
		if option.matched {
			return matchedSessionStyle
		}
		return sessionRowStyle
	case createAction:
		if selected {
			return selectedCreateStyle
		}
		return createRowStyle
	default:
		if selected {
			return selectedPlainStyle
		}
		return plainRowStyle
	}
}

func pad(content string) string {
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
	case attachWindowAction:
		if err := tmux.AttachWindow(result.pendingAction.sessionName, result.pendingAction.windowIndex); err != nil {
			fmt.Fprintf(os.Stderr, "attach session window %q:%d: %v\n", result.pendingAction.sessionName, result.pendingAction.windowIndex, err)
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
