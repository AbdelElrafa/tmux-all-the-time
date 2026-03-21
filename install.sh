#!/usr/bin/env bash

set -euo pipefail

PROJECT_NAME="tmux-all-the-time"
REPO_SLUG="abdelelrafa/tmux-all-the-time"
DEFAULT_BIN_DIR="${HOME}/.local/bin"
MARKER_START="# >>> tmux-all-the-time >>>"
MARKER_END="# <<< tmux-all-the-time <<<"

BIN_DIR="${TATT_INSTALL_DIR:-$DEFAULT_BIN_DIR}"
INSTALL_PATH="${BIN_DIR}/${PROJECT_NAME}"
SOURCE_DIR="${TATT_SOURCE_DIR:-}"
RC_FILE_OVERRIDE="${TATT_SHELL_RC:-}"
SKIP_HOOK="${TATT_SKIP_HOOK:-0}"
FORCE_SOURCE="${TATT_INSTALL_FROM_SOURCE:-0}"

log() {
  printf '[tmux-all-the-time] %s\n' "$*"
}

fail() {
  printf '[tmux-all-the-time] ERROR: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing required command: $1"
}

detect_platform() {
  case "$(uname -s)" in
    Darwin) PLATFORM_OS="darwin" ;;
    Linux) PLATFORM_OS="linux" ;;
    *) fail "Unsupported OS: $(uname -s)" ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64) PLATFORM_ARCH="amd64" ;;
    arm64|aarch64) PLATFORM_ARCH="arm64" ;;
    *) fail "Unsupported architecture: $(uname -m)" ;;
  esac
}

detect_rc_file() {
  if [[ -n "$RC_FILE_OVERRIDE" ]]; then
    RC_FILE="$RC_FILE_OVERRIDE"
    return
  fi

  local shell_name
  shell_name="$(basename "${SHELL:-}")"

  case "$shell_name" in
    zsh) RC_FILE="${HOME}/.zshrc" ;;
    bash) RC_FILE="${HOME}/.bashrc" ;;
    *)
      if [[ -f "${HOME}/.zshrc" ]]; then
        RC_FILE="${HOME}/.zshrc"
      elif [[ -f "${HOME}/.bashrc" ]]; then
        RC_FILE="${HOME}/.bashrc"
      else
        RC_FILE="${HOME}/.profile"
      fi
      ;;
  esac
}

tmux_install_hint() {
  case "$PLATFORM_OS" in
    darwin)
      printf 'Install tmux first, for example with: brew install tmux\n'
      ;;
    linux)
      printf 'Install tmux first using your package manager, for example:\n'
      printf '  Ubuntu/Debian: sudo apt install tmux\n'
      printf '  Fedora: sudo dnf install tmux\n'
      printf '  Arch: sudo pacman -S tmux\n'
      ;;
  esac
}

ensure_tmux_present() {
  if command -v tmux >/dev/null 2>&1; then
    return
  fi

  tmux_install_hint >&2
  fail "tmux is required before installing ${PROJECT_NAME}"
}

release_url() {
  printf 'https://github.com/%s/releases/latest/download/%s_%s_%s.tar.gz' \
    "$REPO_SLUG" "$PROJECT_NAME" "$PLATFORM_OS" "$PLATFORM_ARCH"
}

download_release_binary() {
  local tmpdir archive
  tmpdir="$(mktemp -d)"
  archive="${tmpdir}/${PROJECT_NAME}.tar.gz"

  if ! curl -fsSL "$(release_url)" -o "$archive"; then
    rm -rf "$tmpdir"
    return 1
  fi

  tar -xzf "$archive" -C "$tmpdir"

  if [[ ! -f "${tmpdir}/${PROJECT_NAME}" ]]; then
    rm -rf "$tmpdir"
    fail "Release archive did not contain ${PROJECT_NAME}"
  fi

  install -m 0755 "${tmpdir}/${PROJECT_NAME}" "$INSTALL_PATH"
  rm -rf "$tmpdir"
}

build_from_source() {
  need_cmd go

  local build_dir cleanup_dir
  cleanup_dir=""

  if [[ -n "$SOURCE_DIR" ]]; then
    build_dir="$SOURCE_DIR"
  elif [[ -f "./go.mod" && -f "./main.go" ]]; then
    build_dir="$(pwd)"
  else
    cleanup_dir="$(mktemp -d)"
    build_dir="${cleanup_dir}/${PROJECT_NAME}"
    mkdir -p "$build_dir"
    curl -fsSL "https://codeload.github.com/${REPO_SLUG}/tar.gz/refs/heads/main" | tar -xz --strip-components=1 -C "$build_dir"
  fi

  (cd "$build_dir" && go build -o "$INSTALL_PATH" .)

  if [[ -n "$cleanup_dir" ]]; then
    rm -rf "$cleanup_dir"
  fi
}

install_binary() {
  mkdir -p "$BIN_DIR"

  if [[ "$FORCE_SOURCE" == "1" ]]; then
    log "Building ${PROJECT_NAME} from source"
    build_from_source
    return
  fi

  log "Attempting to install ${PROJECT_NAME} ${PLATFORM_OS}/${PLATFORM_ARCH} release binary"
  if download_release_binary; then
    return
  fi

  log "No release binary available; falling back to a local source build"
  build_from_source
}

backup_rc_file() {
  if [[ -f "$RC_FILE" ]]; then
    cp "$RC_FILE" "${RC_FILE}.bak.${PROJECT_NAME}.$(date +%s)"
  else
    touch "$RC_FILE"
  fi
}

strip_managed_block() {
  local tmpfile
  tmpfile="$(mktemp)"
  awk -v start="$MARKER_START" -v end="$MARKER_END" '
    $0 == start { skip=1; next }
    $0 == end { skip=0; next }
    !skip { print }
  ' "$RC_FILE" > "$tmpfile"
  mv "$tmpfile" "$RC_FILE"
}

install_shell_hook() {
  if [[ "$SKIP_HOOK" == "1" ]]; then
    log "Skipping shell hook installation because TATT_SKIP_HOOK=1"
    return
  fi

  backup_rc_file
  strip_managed_block

  cat >> "$RC_FILE" <<EOF

$MARKER_START
case \$- in
  *i*)
    if [ -z "\${TMUX:-}" ] && [ "\${TMUX_ALL_THE_TIME_DISABLE:-0}" != "1" ] && [ -x "$INSTALL_PATH" ]; then
      "$INSTALL_PATH"
    fi
    ;;
esac
$MARKER_END
EOF
}

post_install_notes() {
  log "Installed binary to $INSTALL_PATH"

  if [[ ":${PATH:-}:" != *":${BIN_DIR}:"* ]]; then
    log "Note: ${BIN_DIR} is not currently on your PATH"
  fi

  if [[ "$SKIP_HOOK" != "1" ]]; then
    log "Updated shell config: $RC_FILE"
    log "Open a new terminal session to start using ${PROJECT_NAME}"
    log "Set TMUX_ALL_THE_TIME_DISABLE=1 to bypass it for one shell"
  else
    log "Run ${INSTALL_PATH} manually or add your own shell hook"
  fi
}

main() {
  detect_platform
  detect_rc_file
  ensure_tmux_present
  install_binary
  install_shell_hook
  post_install_notes
}

main "$@"
