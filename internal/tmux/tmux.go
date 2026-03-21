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
	Name     string
	Windows  int
	Attached bool
}

func ListSessions() ([]Session, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_attached}")
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

		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected tmux session format: %q", line)
		}

		windows, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parse tmux window count %q: %w", fields[1], err)
		}

		sessions = append(sessions, Session{
			Name:     fields[0],
			Windows:  windows,
			Attached: fields[2] == "1",
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return strings.ToLower(sessions[i].Name) < strings.ToLower(sessions[j].Name)
	})

	if len(sessions) == 0 {
		return nil, ErrNoSessions
	}

	return sessions, nil
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
