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
	Windows     []Window
}

type Window struct {
	Index  int
	Name   string
	Active bool
}

func ListSessions() ([]Session, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}\x1f#{session_windows}\x1f#{session_attached}")
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
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected tmux session format: %q", line)
		}

		windows, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parse tmux window count %q: %w", fields[1], err)
		}

		sessions = append(sessions, Session{
			Name:        fields[0],
			WindowCount: windows,
			Attached:    fields[2] == "1",
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
	cmd := exec.Command("tmux", "list-windows", "-a", "-F", "#{session_name}\x1f#{window_index}\x1f#{window_name}\x1f#{window_active}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isNoServerError(strings.TrimSpace(string(output))) {
			return map[string][]Window{}, nil
		}

		return nil, fmt.Errorf("list tmux windows: %w: %s", err, strings.TrimSpace(string(output)))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	windowsBySession := make(map[string][]Window)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, "\x1f")
		if len(fields) != 4 {
			return nil, fmt.Errorf("unexpected tmux window format: %q", line)
		}

		index, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parse tmux window index %q: %w", fields[1], err)
		}

		windowsBySession[fields[0]] = append(windowsBySession[fields[0]], Window{
			Index:  index,
			Name:   fields[2],
			Active: fields[3] == "1",
		})
	}

	for sessionName := range windowsBySession {
		sort.Slice(windowsBySession[sessionName], func(i, j int) bool {
			return windowsBySession[sessionName][i].Index < windowsBySession[sessionName][j].Index
		})
	}

	return windowsBySession, nil
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
