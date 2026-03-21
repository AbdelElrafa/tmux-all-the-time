# tmux-all-the-time

A small terminal-first app that runs on shell startup, shows your existing `tmux` sessions, and lets you either attach to one or create a new one.

## Stack

- Go
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI
- `tmux` as the session backend

## Install

Install `tmux` first.

- macOS: `brew install tmux`
- Ubuntu/Debian: `sudo apt install tmux`
- Fedora: `sudo dnf install tmux`
- Arch: `sudo pacman -S tmux`

Then install the app and shell hook:

```bash
curl -fsSL https://raw.githubusercontent.com/abdelelrafa/tmux-all-the-time/main/install.sh | bash
```

What the installer does:

- installs the binary to `~/.local/bin/tmux-all-the-time`
- adds a managed startup block to your shell rc file
- works with `zsh` and `bash`
- uses a GitHub release binary when available
- falls back to building from source with `go` if needed

## What the first version does

- Shows one text input for the session name
- Filters existing `tmux` sessions live as you type
- Shows the windows under each listed session
- Lets search match window names and highlight the matching nested window row directly
- Shows a live preview panel for the selected session or window
- Lets you attach to a matching session on `Enter`
- Lets you create a new session from the same screen when the name does not exist
- Lets you continue without `tmux`

## Build

```bash
go build .
```

This creates a binary named `tmux-all-the-time` in the project root.

## Run

```bash
./tmux-all-the-time
```

Use arrow keys or `Tab` to move through actions, then press `Enter`.

## Notes

- `tmux` must already be installed
- Set `TMUX_ALL_THE_TIME_DISABLE=1` to bypass the startup hook for one shell
- The installer writes a clearly marked managed block so it can be updated cleanly later
