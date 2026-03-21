#!/usr/bin/env bash

set -euo pipefail

ensure_session() {
  local session_name="$1"
  local first_window_name="$2"

  if ! tmux has-session -t "$session_name" 2>/dev/null; then
    tmux new-session -d -s "$session_name" -n "$first_window_name"
  fi
}

ensure_window() {
  local session_name="$1"
  local window_name="$2"

  if ! tmux list-windows -t "$session_name" -F '#{window_name}' | grep -Fxq "$window_name"; then
    tmux new-window -d -t "$session_name" -n "$window_name"
  fi
}

window_index() {
  local session_name="$1"
  local window_name="$2"

  tmux list-windows -t "$session_name" -F '#{window_name}:#{window_index}' |
    awk -F: -v name="$window_name" '$1 == name { print $2; exit }'
}

write_preview() {
  local session_name="$1"
  local window_name="$2"
  local tmp_file
  local target

  tmp_file="$(mktemp)"
  cat >"$tmp_file"

  target="${session_name}:$(window_index "$session_name" "$window_name")"
  tmux send-keys -t "$target" C-c
  tmux send-keys -t "$target" "clear; cat '$tmp_file'; rm -f '$tmp_file'; while true; do sleep 3600; done" C-m
  tmux select-pane -t "$target" -T "$window_name"
  sleep 0.1
  tmux clear-history -t "$target"
}

ensure_session "web-client" "app"
ensure_window "web-client" "api"
ensure_window "web-client" "logs"

ensure_session "infra-ops" "deploy"
ensure_window "infra-ops" "metrics"
ensure_window "infra-ops" "ssh-prod"

ensure_session "docs-and-notes" "scratch"
ensure_window "docs-and-notes" "roadmap"
ensure_window "docs-and-notes" "todo"

write_preview "web-client" "app" <<'EOF'
abdel@mac web-client % ./scripts/dev-status

STACK STATUS
------------
frontend  ready     http://localhost:3000
api       ready     http://localhost:8787
db        ready     postgres://localhost:5432/app
queue     idle      12 jobs waiting

RECENT REQUESTS
---------------
GET   /dashboard        200   184ms
GET   /api/projects     200    42ms
POST  /api/sessions     201    18ms
WS    /realtime         ok      3 clients
EOF

write_preview "web-client" "api" <<'EOF'
abdel@mac web-client % pnpm --filter api dev

> api@0.3.0 dev
> tsx watch src/server.ts

[api] listening on :8787
[api] connected to postgres://localhost:5432/app
[api] GET /health 200 3ms
[api] POST /sessions 201 18ms
EOF

write_preview "web-client" "logs" <<'EOF'
abdel@mac web-client % tail -n 8 logs/dev.log

[14:02:11] auth refresh succeeded for user_1284
[14:02:12] sync queued 12 changes for project atlas
[14:02:13] ws client connected from 10.0.0.24
[14:02:14] rebuilt dashboard widgets in 312ms
[14:02:16] GET /api/usage 200 27ms
[14:02:19] billing webhook processed in 91ms
EOF

write_preview "infra-ops" "deploy" <<'EOF'
ops@infra % ./scripts/deploy staging

==> validating terraform plan
==> building image ghcr.io/abdelelrafa/web-client:staging-142
==> pushing image... done
==> rollout started for web-client-staging
==> health checks passing (6/6)
deploy complete in 2m14s
EOF

write_preview "infra-ops" "metrics" <<'EOF'
ops@infra % kubectl top pods -n staging

NAME                                  CPU(cores)   MEMORY(bytes)
web-client-6d9889c5d8-f2z4p          42m          214Mi
api-6d9f84b4b7-s3ph9                 31m          188Mi
worker-7bfccd9d76-v2tdk              18m          143Mi
redis-0                               6m           61Mi

ops@infra % kubectl get deploy -n staging
web-client   3/3   3   3   19d
api          3/3   3   3   19d
EOF

write_preview "infra-ops" "ssh-prod" <<'EOF'
ops@infra % ssh prod-app-1

Last login: Sat Mar 21 13:48:02 2026 from 10.1.0.18
ubuntu@prod-app-1:~$ sudo systemctl status app --no-pager
app.service - Web Client API
  Active: active (running) since Sat 2026-03-21 12:09:11 UTC; 1h 52min ago
  Tasks: 24 (limit: 18955)
  Memory: 312.4M
EOF

write_preview "docs-and-notes" "scratch" <<'EOF'
notes@desk % nvim scratch.md

# Sprint notes

- tighten tmux selector layout
- publish release binaries
- add GIF to README
EOF

write_preview "docs-and-notes" "roadmap" <<'EOF'
notes@desk % sed -n "1,12p" roadmap.md

# Q2 roadmap

1. polish preview formatting
2. add release workflow for linux + macOS
3. improve fuzzy ranking with exact-match bias
4. screenshot + docs pass for launch
EOF

write_preview "docs-and-notes" "todo" <<'EOF'
notes@desk % sed -n "1,10p" todo.txt

[x] installer writes managed shell block
[x] preview panel reads pane scrollback
[ ] test on Ubuntu
[ ] record demo video
[ ] tag v0.1.0
EOF

tmux select-window -t "web-client:app"
tmux select-window -t "infra-ops:metrics"
tmux select-window -t "docs-and-notes:roadmap"
