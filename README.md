# tmux-all-the-time

`tmux-all-the-time` is a small terminal UI that runs when a new shell opens and pushes you to either attach to an existing `tmux` session or create a new one.

I built it to force myself to use `tmux` consistently, so my long-running SSH sessions stay available across devices and I can pick up my Codex and Claude Code sessions from anywhere without rebuilding context every time.

## Why

If you already work in SSH and AI coding tools, `tmux` is the obvious answer for persistence, but actually building the habit can still be annoying. A normal terminal opens into a plain shell, and that friction is enough to fall out of the workflow.

This project removes that gap:

- open a terminal
- choose a session or create one
- keep your work persistent by default

## What it does

- runs on shell startup
- lists existing `tmux` sessions
- shows nested window rows under each session
- filters sessions and windows as you type
- highlights matching window rows directly
- shows a live preview panel for the selected session or window
- lets you attach to an existing session or a specific window
- lets you create a new session from the same screen
- lets you continue without `tmux` when needed

## Stack

- Go
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- `tmux`

## Quick Start

Install `tmux` first:

- macOS: `brew install tmux`
- Ubuntu/Debian: `sudo apt install tmux`
- Fedora: `sudo dnf install tmux`
- Arch: `sudo pacman -S tmux`

Then run:

```bash
curl -fsSL https://raw.githubusercontent.com/abdelelrafa/tmux-all-the-time/main/install.sh | bash
```

The installer:

- installs the binary to `~/.local/bin/tmux-all-the-time`
- adds a managed startup block to your shell rc file
- supports `zsh` and `bash`
- uses a GitHub release binary when available
- falls back to building from source with `go` if no release binary is available

After install, open a new terminal window and the selector should appear automatically.

## Usage

- Type to filter sessions or windows
- Use arrow keys or `Tab` to move
- Press `Enter` to select
- Press `Ctrl+R` to reload session data
- Set `TMUX_ALL_THE_TIME_DISABLE=1` to bypass the hook for one shell

## Build From Source

```bash
go build .
./tmux-all-the-time
```

## Notes

- `tmux` must already be installed
- the installer writes a clearly marked managed block into your shell config
- if `~/.local/bin` is not on your `PATH`, add it so the installed binary is available everywhere
