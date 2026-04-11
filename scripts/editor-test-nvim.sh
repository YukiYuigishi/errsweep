#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

NVIM_LOG="$TMPDIR/nvim.log"
INIT_LUA="$TMPDIR/init.lua"
PROXY_LOG="$TMPDIR/proxy.log"
WRAPPER="$TMPDIR/sentinel-lsp-proxy-wrapper.sh"
XDG_CONFIG_HOME="$TMPDIR/xdg-config"
XDG_DATA_HOME="$TMPDIR/xdg-data"
XDG_STATE_HOME="$TMPDIR/xdg-state"
XDG_CACHE_HOME="$TMPDIR/xdg-cache"

mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME" "$XDG_STATE_HOME" "$XDG_CACHE_HOME"

cat > "$WRAPPER" <<EOF
#!/usr/bin/env bash
exec "$ROOT/sentinel-lsp-proxy" "\$@" 2>>"$PROXY_LOG"
EOF
chmod +x "$WRAPPER"

cat > "$INIT_LUA" <<EOF
vim.cmd("filetype on")
vim.cmd("edit $ROOT/example/usecase/user.go")
local bufnr = vim.api.nvim_get_current_buf()
local id = vim.lsp.start_client({
  name = "gopls",
  cmd = {
    "$WRAPPER",
    "--gopls=gopls",
    "--sentinelfind=$ROOT/sentinelfind",
    "--workspace=$ROOT",
    "--cache-timeout=120s",
    "serve",
  },
  root_dir = "$ROOT",
})
if id then
  vim.lsp.buf_attach_client(bufnr, id)
end
vim.defer_fn(function()
  pcall(vim.lsp.buf.hover)
end, 1800)
vim.defer_fn(function()
  vim.cmd("qa!")
end, 5000)
EOF

NVIM_APPNAME="errsweep-nvim-test" \
XDG_CONFIG_HOME="$XDG_CONFIG_HOME" \
XDG_DATA_HOME="$XDG_DATA_HOME" \
XDG_STATE_HOME="$XDG_STATE_HOME" \
XDG_CACHE_HOME="$XDG_CACHE_HOME" \
  nvim --headless --noplugin -u NONE -i NONE \
  -c "luafile $INIT_LUA" \
  >"$NVIM_LOG" 2>&1 || true

if grep -q "sentinel-lsp-proxy: loaded " "$PROXY_LOG"; then
  echo "nvim editor test: PASS"
else
  echo "nvim editor test: FAIL"
  echo "---- proxy log ----"
  cat "$PROXY_LOG" || true
  echo "---- nvim log ----"
  cat "$NVIM_LOG"
  exit 1
fi
