package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

var ErrNoSessions = errors.New("no tmux sessions found")

type Session struct {
	Name        string
	WindowCount int
	Attached    bool
	Activity    int64
	Windows     []Window
}

type Window struct {
	Index          int
	Name           string
	Active         bool
	Activity       int64
	PaneID         string
	CurrentCommand string
	PaneTitle      string
	Preview        []string
}

type windowKey struct {
	SessionName string
	WindowIndex int
}

type paneDetails struct {
	PaneID         string
	CurrentCommand string
	PaneTitle      string
	Preview        []string
}

func ListSessions() ([]Session, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}\x1f#{session_windows}\x1f#{session_attached}\x1f#{session_activity}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isNoServerError(strings.TrimSpace(string(output))) {
			return nil, ErrNoSessions
		}

		return nil, fmt.Errorf("list tmux sessions: %w: %s", err, strings.TrimSpace(string(output)))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	sessions := make([]Session, 0, len(lines))

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, "\x1f")
		if len(fields) != 4 {
			return nil, fmt.Errorf("unexpected tmux session format: %q", line)
		}

		windows, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parse tmux window count %q: %w", fields[1], err)
		}

		activity, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse tmux session activity %q: %w", fields[3], err)
		}

		sessions = append(sessions, Session{
			Name:        fields[0],
			WindowCount: windows,
			Attached:    fields[2] == "1",
			Activity:    activity,
		})
	}

	windowsBySession, err := listWindows()
	if err != nil {
		return nil, err
	}

	sort.Slice(sessions, func(i, j int) bool {
		return strings.ToLower(sessions[i].Name) < strings.ToLower(sessions[j].Name)
	})

	for i := range sessions {
		sessions[i].Windows = windowsBySession[sessions[i].Name]
	}

	if len(sessions) == 0 {
		return nil, ErrNoSessions
	}

	return sessions, nil
}

func listWindows() (map[string][]Window, error) {
	cmd := exec.Command("tmux", "list-windows", "-a", "-F", "#{session_name}\x1f#{window_index}\x1f#{window_name}\x1f#{window_active}\x1f#{window_activity}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isNoServerError(strings.TrimSpace(string(output))) {
			return map[string][]Window{}, nil
		}

		return nil, fmt.Errorf("list tmux windows: %w: %s", err, strings.TrimSpace(string(output)))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	windowsBySession := make(map[string][]Window)
	activePanes, err := listActivePanes()
	if err != nil {
		return nil, err
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, "\x1f")
		if len(fields) != 5 {
			return nil, fmt.Errorf("unexpected tmux window format: %q", line)
		}

		index, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parse tmux window index %q: %w", fields[1], err)
		}

		activity, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse tmux window activity %q: %w", fields[4], err)
		}

		key := windowKey{SessionName: fields[0], WindowIndex: index}
		pane := activePanes[key]

		windowsBySession[fields[0]] = append(windowsBySession[fields[0]], Window{
			Index:          index,
			Name:           fields[2],
			Active:         fields[3] == "1",
			Activity:       activity,
			PaneID:         pane.PaneID,
			CurrentCommand: pane.CurrentCommand,
			PaneTitle:      pane.PaneTitle,
			Preview:        pane.Preview,
		})
	}

	for sessionName := range windowsBySession {
		sort.Slice(windowsBySession[sessionName], func(i, j int) bool {
			return windowsBySession[sessionName][i].Index < windowsBySession[sessionName][j].Index
		})
	}

	return windowsBySession, nil
}

func listActivePanes() (map[windowKey]paneDetails, error) {
	cmd := exec.Command("tmux", "list-panes", "-a", "-F", "#{session_name}\x1f#{window_index}\x1f#{pane_active}\x1f#{pane_id}\x1f#{pane_current_command}\x1f#{pane_title}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isNoServerError(strings.TrimSpace(string(output))) {
			return map[windowKey]paneDetails{}, nil
		}

		return nil, fmt.Errorf("list tmux panes: %w: %s", err, strings.TrimSpace(string(output)))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	panes := make(map[windowKey]paneDetails)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, "\x1f")
		if len(fields) != 6 {
			return nil, fmt.Errorf("unexpected tmux pane format: %q", line)
		}
		if fields[2] != "1" {
			continue
		}

		windowIndex, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parse tmux pane window index %q: %w", fields[1], err)
		}

		panes[windowKey{SessionName: fields[0], WindowIndex: windowIndex}] = paneDetails{
			PaneID:         fields[3],
			CurrentCommand: fields[4],
			PaneTitle:      fields[5],
			Preview:        capturePanePreview(fields[3], 20),
		}
	}

	return panes, nil
}

func capturePanePreview(targetPane string, lineCount int) []string {
	cmd := exec.Command("tmux", "capture-pane", "-p", "-q", "-t", targetPane, "-S", fmt.Sprintf("-%d", lineCount), "-E", "-")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.ReplaceAll(string(output), "\r\n", "\n"), "\n")
	lines = trimBlankLines(lines)
	if len(lines) == 0 {
		return nil
	}

	if len(lines) > lineCount {
		lines = lines[len(lines)-lineCount:]
	}

	return lines
}

func trimBlankLines(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	return lines[start:end]
}

func HasSession(name string) (bool, error) {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, fmt.Errorf("check tmux session %q: %w", name, err)
}

func Attach(name string) error {
	return runInteractive("tmux", "attach-session", "-t", name)
}

func AttachWindow(sessionName string, windowIndex int) error {
	target := fmt.Sprintf("%s:%d", sessionName, windowIndex)
	if err := runInteractive("tmux", "attach-session", "-t", target); err == nil {
		return nil
	}

	if err := exec.Command("tmux", "select-window", "-t", target).Run(); err != nil {
		return fmt.Errorf("select tmux window %q: %w", target, err)
	}

	return runInteractive("tmux", "attach-session", "-t", sessionName)
}

func CreateAndAttach(name string) error {
	return runInteractive("tmux", "new-session", "-s", name)
}

func runInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}

	return nil
}

func isNoServerError(output string) bool {
	return strings.Contains(output, "no server running") ||
		strings.Contains(output, "failed to connect to server") ||
		strings.Contains(output, "error connecting to")
}
