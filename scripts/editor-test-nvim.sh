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
touch "$NVIM_LOG" "$PROXY_LOG"

cat > "$WRAPPER" <<EOF
#!/usr/bin/env bash
exec "$ROOT/sentinel-lsp-proxy" "\$@" 2>>"$PROXY_LOG"
EOF
chmod +x "$WRAPPER"

cat > "$INIT_LUA" <<EOF
vim.cmd("filetype on")
vim.cmd("edit $ROOT/proxy/cache.go")
local bufnr = vim.api.nvim_get_current_buf()
local client_id
if vim.lsp.start then
  client_id = vim.lsp.start({
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
  }, { bufnr = bufnr })
else
  client_id = vim.lsp.start_client({
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
  if client_id then
    vim.lsp.buf_attach_client(bufnr, client_id)
  end
end

local line = 1
local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
for i, l in ipairs(lines) do
  if l:find("func%s+ParseSentinelfindJSON", 1) then
    line = i
    break
  end
end
local text = lines[line] or ""
local col = (string.find(text, "ParseSentinelfindJSON", 1, true) or 6) - 1
vim.api.nvim_win_set_cursor(0, {line, col})

local function content_to_text(content)
  if type(content) == "string" then
    return content
  end
  if type(content) ~= "table" then
    return ""
  end
  if content.value and type(content.value) == "string" then
    return content.value
  end
  if content.language and content.value and type(content.value) == "string" then
    return content.value
  end
  local out = {}
  for _, c in ipairs(content) do
    table.insert(out, content_to_text(c))
  end
  return table.concat(out, "\n")
end

local function get_hover_text()
  local client = client_id and vim.lsp.get_client_by_id(client_id) or nil
  local enc = (client and client.offset_encoding) or "utf-16"
  local params = vim.lsp.util.make_position_params(0, enc)
  local resp = vim.lsp.buf_request_sync(bufnr, "textDocument/hover", params, 700)
  if type(resp) ~= "table" then
    return ""
  end
  for _, item in pairs(resp) do
    if item and item.result and item.result.contents then
      return content_to_text(item.result.contents)
    end
  end
  return ""
end

local function lsp_clients_for_buf(bufnr)
  if vim.lsp.get_clients then
    return vim.lsp.get_clients({ bufnr = bufnr })
  end
  if vim.lsp.buf_get_clients then
    local by_id = vim.lsp.buf_get_clients(bufnr)
    local out = {}
    for _, c in pairs(by_id or {}) do
      table.insert(out, c)
    end
    return out
  end
  if vim.lsp.get_active_clients then
    return vim.lsp.get_active_clients()
  end
  return {}
end

vim.defer_fn(function()
  vim.wait(1500, function()
    local clients = lsp_clients_for_buf(bufnr)
    if not clients or #clients == 0 then
      return false
    end
    for _, c in ipairs(clients) do
      if c.initialized then
        return true
      end
    end
    return false
  end, 100)

  local last = ""
  for _ = 1, 8 do
    local txt = get_hover_text()
    last = txt
    if txt:find("Possible Sentinel Errors", 1, true) and txt:find("InvalidUnmarshalError", 1, true) then
      print("HOVER_OK")
      vim.cmd("qa!")
      return
    end
    vim.wait(250)
  end
  print("HOVER_FAIL: " .. last)
  vim.cmd("qa!")
end, 2000)

vim.defer_fn(function()
  print("HOVER_TIMEOUT")
  vim.cmd("qa!")
end, 14000)
EOF

NVIM_APPNAME="errsweep-nvim-test" \
XDG_CONFIG_HOME="$XDG_CONFIG_HOME" \
XDG_DATA_HOME="$XDG_DATA_HOME" \
XDG_STATE_HOME="$XDG_STATE_HOME" \
XDG_CACHE_HOME="$XDG_CACHE_HOME" \
  nvim --headless --noplugin -u NONE -i NONE \
  -c "luafile $INIT_LUA" \
  >"$NVIM_LOG" 2>&1 &
NVIM_PID=$!

for _ in $(seq 1 30); do
  if ! kill -0 "$NVIM_PID" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if kill -0 "$NVIM_PID" >/dev/null 2>&1; then
  kill "$NVIM_PID" >/dev/null 2>&1 || true
  sleep 1
  if kill -0 "$NVIM_PID" >/dev/null 2>&1; then
    kill -9 "$NVIM_PID" >/dev/null 2>&1 || true
  fi
fi
wait "$NVIM_PID" 2>/dev/null || true

if grep -q "sentinel-lsp-proxy: loaded " "$PROXY_LOG" && grep -q "HOVER_OK" "$NVIM_LOG"; then
  echo "nvim editor test: PASS"
else
  echo "nvim editor test: FAIL"
  echo "---- proxy log ----"
  cat "$PROXY_LOG" || true
  echo "---- nvim log ----"
  cat "$NVIM_LOG"
  exit 1
fi
