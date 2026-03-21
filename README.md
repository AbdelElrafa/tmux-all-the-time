# tmux-all-the-time

A small terminal-first app that runs on shell startup, shows your existing `tmux` sessions, and lets you either attach to one or create a new one.

## Stack

- Go
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI
- `tmux` as the session backend

## What the first version does

- Shows one text input for the session name
- Filters existing `tmux` sessions live as you type
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

## Hook it into `zsh`

Add this to your `~/.zshrc`:

```zsh
if [[ -o interactive ]] && [[ -z "$TMUX" ]] && [[ -x "$HOME/Code/tmux-all-the-time/tmux-all-the-time" ]]; then
  "$HOME/Code/tmux-all-the-time/tmux-all-the-time"
fi
```

This prevents nested `tmux` launches and only runs for interactive shells.

## Notes

- `tmux` must already be installed
- If you want to skip the TUI for some terminals later, the `~/.zshrc` guard is the right place to add that logic
- A useful next step is adding a wrapper script or env flag for bypassing the app temporarily
