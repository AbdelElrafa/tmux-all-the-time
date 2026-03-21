# Repository Guidelines

## Project Structure & Module Organization

- `main.go`: Bubble Tea entrypoint and TUI layout, selection logic, and preview rendering.
- `internal/tmux/`: tmux integration layer for listing sessions/windows, attaching, and capturing preview data.
- `install.sh`: cross-platform installer that places the binary in `~/.local/bin` and adds the managed shell hook.
- `README.md`: user-facing install and usage documentation.
- `go.mod` / `go.sum`: Go module and dependency lock data.

This repo currently has no dedicated `testdata/`, `cmd/`, or `assets/` directories.

## Build, Test, and Development Commands

- `go build .`
  Builds the local binary in the repo root.
- `go test ./...`
  Runs all Go tests across the module.
- `gofmt -w main.go internal/tmux/tmux.go`
  Formats the main app and tmux integration code.
- `./tmux-all-the-time`
  Runs the selector locally for manual UI testing.
- `bash -n install.sh`
  Checks installer shell syntax without executing it.

## Coding Style & Naming Conventions

- Follow standard Go formatting with `gofmt`; do not hand-format around it.
- Keep code ASCII unless a file already requires Unicode.
- Use short, direct function names that describe behavior, for example `renderPreview`, `ListSessions`, `AttachWindow`.
- Keep package layout simple: app flow in `main.go`, tmux shelling in `internal/tmux/`.
- Prefer small helpers over deeply nested rendering logic.

## Testing Guidelines

- There are currently no committed Go test files; rely on `go test ./...` plus manual terminal verification.
- For UI changes, verify:
  - empty search behavior
  - session/window filtering
  - preview rendering
  - attach/create flows
- For installer changes, test with temp rc files and temp bin dirs before touching a real shell config.

## Commit & Pull Request Guidelines

- Use short imperative commit messages, for example:
  - `Add installer and smarter default selection`
  - `Add tmux session preview pane`
- Keep commits focused on one logical change.
- PRs should explain user-facing behavior changes, install/setup impact, and any manual verification performed.
- Include terminal screenshots or recordings for visible TUI changes when possible.

## Security & Configuration Tips

- Do not hardcode machine-specific paths in app logic or docs.
- Keep shell-hook edits inside the managed installer block only.
- Treat `~/.zshrc`, `~/.bashrc`, and tmux session contents as user state; change them conservatively.
