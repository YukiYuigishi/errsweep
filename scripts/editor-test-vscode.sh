#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="$(mktemp -d)"

USER_DIR="$TMPDIR/user"
EXT_DIR="$TMPDIR/extensions"
LOG_STD="$TMPDIR/code-stdout.log"
SETTINGS_DIR="$USER_DIR/User"
SETTINGS_JSON="$SETTINGS_DIR/settings.json"
PROXY_LOG="$TMPDIR/proxy.log"
WRAPPER="$TMPDIR/sentinel-lsp-proxy-wrapper.sh"
CODE_PID=""

terminate_pid() {
  local pid="$1"
  if [ -z "$pid" ]; then
    return
  fi
  if ! kill -0 "$pid" >/dev/null 2>&1; then
    return
  fi
  kill "$pid" >/dev/null 2>&1 || true
  sleep 1
  if kill -0 "$pid" >/dev/null 2>&1; then
    kill -9 "$pid" >/dev/null 2>&1 || true
  fi
}

cleanup() {
  if [ -n "$CODE_PID" ]; then
    terminate_pid "$CODE_PID"
    while read -r child; do
      terminate_pid "$child"
    done < <(pgrep -P "$CODE_PID" 2>/dev/null || true)
  fi
  while read -r pid; do
    terminate_pid "$pid"
  done < <(ps -axo pid=,command= | awk -v ud="$USER_DIR" '$0 ~ ud {print $1}')
  rm -rf "$TMPDIR"
}
trap cleanup EXIT INT TERM

mkdir -p "$SETTINGS_DIR" "$EXT_DIR"

cat > "$WRAPPER" <<EOF
#!/usr/bin/env bash
exec "$ROOT/sentinel-lsp-proxy" "\$@" 2>>"$PROXY_LOG"
EOF
chmod +x "$WRAPPER"

cat > "$SETTINGS_JSON" <<EOF
{
  "go.alternateTools": {
    "gopls": "$WRAPPER"
  },
  "go.languageServerFlags": [
    "--gopls=gopls",
    "--sentinelfind=$ROOT/sentinelfind",
    "--workspace=$ROOT",
    "--cache-timeout=120s"
  ]
}
EOF

code --install-extension golang.go --extensions-dir "$EXT_DIR" --force >/dev/null 2>&1 || true

code \
  --user-data-dir "$USER_DIR" \
  --extensions-dir "$EXT_DIR" \
  --new-window \
  --verbose \
  --log trace \
  --disable-workspace-trust \
  "$ROOT" \
  -g "$ROOT/example/usecase/user.go:9:5" \
  >"$LOG_STD" 2>&1 &
CODE_PID=$!

for _ in $(seq 1 40); do
  if grep -q "sentinel-lsp-proxy: loaded " "$PROXY_LOG" 2>/dev/null; then
    break
  fi
  sleep 1
done

terminate_pid "$CODE_PID"
wait "$CODE_PID" 2>/dev/null || true

if grep -q "sentinel-lsp-proxy: loaded " "$PROXY_LOG" 2>/dev/null; then
  echo "vscode editor test: PASS"
else
  echo "vscode editor test: FAIL"
  echo "---- proxy log ----"
  cat "$PROXY_LOG" || true
  echo "---- vscode logs ----"
  if [ -d "$USER_DIR/logs" ]; then
    find "$USER_DIR/logs" -type f | sed "s#^#log: #"
  fi
  echo "---- code stdout ----"
  cat "$LOG_STD"
  exit 1
fi
