# errsweep

Go 関数が返しうる Sentinel Error（`var ErrXxx = ...`）を静的解析で抽出する Analyzer / CLI / LSP 補助ツールです。

## 提供バイナリ

| バイナリ | 用途 |
|---|---|
| `errsweep` | CLI 解析 |
| `errsweep-lsp-proxy` | gopls の hover に Sentinel 情報を追記する LSP プロキシ |
| `errsweep-lsp` | スタンドアロン LSP（hover のみ） |

## インストール

```bash
go install github.com/YukiYuigishi/errsweep/cmd/errsweep@latest
go install github.com/YukiYuigishi/errsweep/cmd/errsweep-lsp-proxy@latest
go install github.com/YukiYuigishi/errsweep/cmd/errsweep-lsp@latest
```

## 開発環境セットアップ

```bash
make dev-setup
```

`dev-setup` で以下を実施します。

- 依存取得 (`go mod download`)
- 開発ツール導入（`gopls`, `golangci-lint`, 各バイナリ）
- 必須コマンドチェック（`go`, `nvim`, `code` など）
- バイナリビルド
- Git hook 設定（`core.hooksPath=.githooks`）

## Lint / Hook

```bash
# lint
make lint-go

# 自動修正付き lint
make lint-fix

# hook のみ設定
make setup-hooks
```

`pre-commit` では次を実行します。

1. `golangci-lint run --fix ./...`
2. `make test-all`
3. 差分が発生した場合はコミットを停止（再ステージを要求）

## テスト

```bash
# Go テスト
make test

# エディタ E2E（Neovim + VS Code）
make test-editor

# 全部まとめて（lintはCI側で別実行）
make test-all
```

## ベンチマーク（cache-pattern 比較）

`errsweep-lsp-proxy --cache-pattern` の効果を比較するための簡易ベンチです。

```bash
# デフォルト対象（example module）で比較
make bench-cache-pattern

# moby向けプリセットで比較（tmp/moby を使用）
make bench-cache-pattern-moby

# 対象リポジトリを手動指定
CACHE_BENCH_REPO="$PWD/tmp/moby" CACHE_BENCH_PRESET=moby make bench-cache-pattern

# パターンを明示指定
./scripts/bench-cache-pattern.sh ./... ./daemon/... ./pkg/...

# しきい値チェック付き（回帰検知）
make bench-cache-pattern-check
```

出力はデフォルトでターミナル向けプレーン表形式です。Markdown テーブルが必要な場合は
`CACHE_BENCH_FORMAT=markdown` を指定してください。

しきい値を直接指定する場合:

```bash
CACHE_BENCH_MAX_AVG_REAL=2.0 CACHE_BENCH_MAX_AVG_EXIT=0.0 ./scripts/bench-cache-pattern.sh
```

### エディタ E2E の内容

- `test-editor-nvim`
  - ユーザー設定を使わない隔離構成（`-u NONE --noplugin` + 分離 XDG）
  - `errsweep-lsp-proxy` 経由で hover 実行
  - hover に `Possible Sentinel Errors` と期待 sentinel が含まれることを検証
- `test-editor-vscode`
  - 隔離 `--user-data-dir` / `--extensions-dir`
  - `--disable-workspace-trust`
  - カスタムテスト拡張で `vscode.executeHoverProvider` を実行し hover 内容を検証
  - テスト後に VS Code プロセスを自動終了

## CI

GitHub Actions: `.github/workflows/test-all.yml`

- トリガー: `push`, `pull_request`
- 実行内容:
  1. `make lint-go`
  2. `xvfb-run -a make test-all`
- キャッシュ:
  - Go modules/build cache（`actions/setup-go`）
  - VS Code extensions（`.ci-cache/vscode-extensions`）

## CLI 使い方

```bash
# 単一パッケージ
errsweep ./pkg/repository

# モジュール全体
errsweep ./...

# JSON 出力
errsweep -json ./...
```

終了コード:

| code | 意味 |
|---|---|
| 0 | 診断なし |
| 1 | 内部エラー / ロード失敗 |
| 3 | 診断あり |

## LSP 利用

### VS Code（推奨: proxy）

```json
{
  "go.alternateTools": {
    "gopls": "errsweep-lsp-proxy"
  },
  "go.languageServerFlags": [
    "--gopls=gopls",
    "--errsweep=errsweep",
    "--workspace=${workspaceFolder}",
    "--cache-file=${workspaceFolder}/.errsweep/cache.gob"
  ]
}
```

> `errsweep-lsp` 単体を VS Code の `gopls` 代替として使う構成は実用上非推奨です。  
> Go 拡張は `gopls` の機能セット前提で動作するため、現状は `errsweep-lsp-proxy` 経由を前提にしてください。

### Neovim（proxy）

```lua
vim.lsp.config('gopls', {
  cmd = {
    vim.fn.exepath('errsweep-lsp-proxy'),
    '--gopls=' .. vim.fn.exepath('gopls'),
    '--errsweep=' .. vim.fn.exepath('errsweep'),
    '--workspace=' .. vim.fn.getcwd(),
  },
})
vim.lsp.enable('gopls')
```

### Neovim（proxy を使わない: errsweep-lsp 単体）

```lua
vim.lsp.config('gopls', {
  cmd = {
    vim.fn.exepath('errsweep-lsp'),
    '--errsweep=' .. vim.fn.exepath('errsweep'),
    '--workspace=' .. vim.fn.getcwd(),
  },
})
vim.lsp.enable('gopls')
```

## 既知の制限（現行）

- インターフェース経由の動的ディスパッチは限定的
- 標準ライブラリ既知エラーはマッピングベース
- `%w` ラップのみ同一性維持として追跡（`%v` は対象外）

## OSS ガイドライン

- [Contributing](CONTRIBUTING.md)
