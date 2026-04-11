#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

USER_DIR="$TMPDIR/user"
EXT_DIR="$TMPDIR/extensions"
LOG_STD="$TMPDIR/code-stdout.log"
SETTINGS_DIR="$USER_DIR/User"
SETTINGS_JSON="$SETTINGS_DIR/settings.json"

mkdir -p "$SETTINGS_DIR" "$EXT_DIR"

cat > "$SETTINGS_JSON" <<EOF
{
  "go.alternateTools": {
    "gopls": "$ROOT/sentinel-lsp-proxy"
  },
  "go.languageServerFlags": [
    "--gopls=gopls",
    "--sentinelfind=$ROOT/sentinelfind",
    "--workspace=$ROOT",
    "--cache-timeout=120s"
  ],
  "go.useLanguageServer": true
}
EOF

code --install-extension golang.go --extensions-dir "$EXT_DIR" --force >/dev/null 2>&1 || true

code \
  --user-data-dir "$USER_DIR" \
  --extensions-dir "$EXT_DIR" \
  --new-window \
  --verbose \
  --log trace \
  -g "$ROOT/example/usecase/user.go:9:5" \
  >"$LOG_STD" 2>&1 &
CODE_PID=$!

sleep 20
kill "$CODE_PID" >/dev/null 2>&1 || true
wait "$CODE_PID" 2>/dev/null || true

if grep -R "sentinel-lsp-proxy: loaded " "$USER_DIR/logs" >/dev/null 2>&1; then
  echo "vscode editor test: PASS"
else
  echo "vscode editor test: FAIL"
  if [ -d "$USER_DIR/logs" ]; then
    find "$USER_DIR/logs" -type f | sed "s#^#log: #"
  fi
  cat "$LOG_STD"
  exit 1
fi
