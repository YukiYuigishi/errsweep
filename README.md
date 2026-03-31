# go-sentinel-analyzer

Go関数が返しうるSentinel Error（`var ErrXxx = errors.New(...)`）を静的解析で抽出するカスタムAnalyzer。

## インストール

### go install（推奨）

Go 1.17 以降が必要です。

```bash
go install github.com/YukiYuigishi/errsweep/cmd/sentinelfind@latest
```

インストール後、`$GOPATH/bin`（または `$GOBIN`）に `sentinelfind` が配置されます。パスが通っていれば次のように実行できます：

```bash
sentinelfind ./...
```

### ローカルビルド

リポジトリをクローンしてビルドします：

```bash
git clone https://github.com/YukiYuigishi/errsweep.git
cd errsweep
go build -o sentinelfind ./cmd/sentinelfind
./sentinelfind ./...
```

### 動作確認

```bash
sentinelfind -V=full   # バージョン情報を表示
```

## このリポジトリ自身での動作例

テストフィクスチャ（`analyzer/testdata/src/`）に対して実行すると、各パターンの検出結果を確認できます。

```bash
git clone https://github.com/YukiYuigishi/errsweep.git
cd errsweep
go build -o sentinelfind ./cmd/sentinelfind
./sentinelfind ./analyzer/testdata/src/...
```

出力：

```
analyzer/testdata/src/basic/basic.go:8:6: FindUser returns sentinels: basic.ErrNotFound
analyzer/testdata/src/basic/basic.go:15:6: GetItem returns sentinels: basic.ErrPermission
analyzer/testdata/src/phi/phi.go:8:6: Fetch returns sentinels: phi.ErrNotFound, phi.ErrTimeout
analyzer/testdata/src/wrapped/wrapped.go:10:6: QueryDB returns sentinels: wrapped.ErrDatabase
analyzer/testdata/src/wrapped/wrapped.go:18:6: doQuery returns sentinels: wrapped.ErrDatabase
```

各ソースファイルの内容と対応：

**`testdata/src/basic/basic.go`** — パッケージレベルの Sentinel を直接 return するケース
```go
var ErrNotFound   = errors.New("not found")
var ErrPermission = errors.New("permission denied")

func FindUser(id int) error {  // → FindUser returns sentinels: basic.ErrNotFound
    if id <= 0 { return ErrNotFound }
    return nil
}

func GetItem(id int) (string, error) {  // → GetItem returns sentinels: basic.ErrPermission
    if id < 0 { return "", ErrPermission }
    return "item", nil
}
```

**`testdata/src/phi/phi.go`** — 複数 Sentinel を条件分岐で return するケース（SSA の Phi ノードを横断）
```go
var ErrNotFound = errors.New("not found")
var ErrTimeout  = errors.New("timeout")

func Fetch(id int, fast bool) error {  // → Fetch returns sentinels: phi.ErrNotFound, phi.ErrTimeout
    if id <= 0 { return ErrNotFound }
    if !fast   { return ErrTimeout }
    return nil
}
```

**`testdata/src/wrapped/wrapped.go`** — `fmt.Errorf("%w", ...)` でラップされた Sentinel のケース
```go
var ErrDatabase = errors.New("database error")

func QueryDB(query string) error {  // → QueryDB returns sentinels: wrapped.ErrDatabase
    if err := doQuery(query); err != nil {
        return fmt.Errorf("QueryDB: %w", ErrDatabase)
    }
    return nil
}

func doQuery(q string) error {  // → doQuery returns sentinels: wrapped.ErrDatabase
    if q == "" { return fmt.Errorf("doQuery: %w", ErrDatabase) }
    return nil
}
```

検出されない（正しく無視される）パターンも確認できます：

```bash
# nilreturn・opaque は何も出力されない
./sentinelfind ./analyzer/testdata/src/nilreturn/...
./sentinelfind ./analyzer/testdata/src/opaque/...
```

`nilreturn` は `return nil` のみ、`opaque` は `errors.New(...)` を直接 return するだけで変数名が `Err` で始まらないため、どちらも Sentinel として扱われません。

---

## 使い方

### CLIとして実行

```bash
# 単一パッケージ
sentinelfind ./pkg/repository

# モジュール全体
sentinelfind ./...

# 標準入力としてGoファイルを渡す場合
sentinelfind github.com/yourorg/yourapp/internal/...
```

### 出力例

```
pkg/repository/user.go:14:6: FindUser returns sentinels: sql.ErrNoRows
pkg/repository/item.go:28:6: GetItem returns sentinels: io.EOF, repository.ErrNotFound
```

各行の形式：`<ファイル>:<行>:<列>: <関数名> returns sentinels: <パッケージ名>.<変数名>, ...`

**終了コード**

| コード | 意味 |
|--------|------|
| 0 | 診断なし（問題なし） |
| 1 | 内部エラーまたはパッケージのロード失敗 |
| 3 | 診断あり（Sentinel Errorを検出） |

### フラグ一覧

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `-json` | false | 診断を JSON 形式で出力する。CI スクリプトや他ツールとの連携に使う |
| `-c N` | -1（無効） | 診断行の前後 N 行のソースコードを合わせて出力する |
| `-test` | true | `_test.go` ファイルも解析対象に含める。`-test=false` で除外 |
| `-flags` | — | このアナライザーが受け付けるフラグの一覧を JSON で出力して終了する |
| `-V=full` | — | バイナリのバージョン情報を出力して終了する |
| `-cpuprofile FILE` | — | CPU プロファイルを指定ファイルに書き出す（パフォーマンス調査用） |
| `-memprofile FILE` | — | メモリプロファイルを指定ファイルに書き出す |
| `-trace FILE` | — | 実行トレースを指定ファイルに書き出す |
| `-debug CHARS` | — | デバッグ出力を有効にする。`f`=ファクト `p`=パッケージ `s`=スコープ `t`=型 `v`=詳細 |

**`-json` 出力の構造：**

```json
{
  "パッケージパス": {
    "sentinelfind": [
      {
        "posn": "file.go:8:6",
        "end":  "file.go:8:6",
        "message": "FindUser returns sentinels: pkg.ErrFoo"
      }
    ]
  }
}
```

**`-c` 使用例：**

```bash
# 診断行の前後 2 行を表示
sentinelfind -c 2 ./pkg/...
```

```
pkg/repo/user.go:12:6: FindUser returns sentinels: repo.ErrNotFound
10  var ErrPermission = errors.New("permission denied")
11
12  func FindUser(id int) error {
13      if id <= 0 {
14          return ErrNotFound
```

**`-test=false` 使用例：**

```bash
# テストコード内の関数は解析しない
sentinelfind -test=false ./...
```

### VS Code（go.alternateTools）

`sentinelfind` は `singlechecker.Main` ベースで `staticcheck` と同じ出力フォーマットを持つため、VS Code Go 拡張の `go.alternateTools` で `staticcheck` の代替として登録できます。

1. `sentinelfind` をインストール：

```bash
go install github.com/YukiYuigishi/errsweep/cmd/sentinelfind@latest
```

2. `.vscode/settings.json` に追記：

```json
{
  "go.lintTool": "staticcheck",
  "go.alternateTools": {
    "staticcheck": "sentinelfind"
  },
  "go.lintOnSave": "package"
}
```

保存時に `sentinelfind` が実行され、検出した Sentinel Error がエディタ上に警告として表示されます。

> **注意**: この設定では `staticcheck` 本来のチェックは無効になります。両方を使いたい場合は multichecker を自作して同様に登録してください。

### golangci-lint プラグインとして使う

`golangci-lint` の `custom` ローダーに組み込む（将来対応）：

```yaml
# .golangci.yml
linters:
  enable:
    - sentinelfind
custom:
  sentinelfind:
    path: ./sentinelfind.so
    description: Reports sentinel errors a function may return
    original-url: github.com/YukiYuigishi/errsweep
```

### `go/analysis` フレームワークへの組み込み

自前のmulticheckerや解析パイプラインに追加できます：

```go
import (
    "golang.org/x/tools/go/analysis/multichecker"
    "errsweep/analyzer"
)

func main() {
    multichecker.Main(
        analyzer.Analyzer,
        // 他のAnalyzerと併用可能
    )
}
```

## 検出ルール

| パターン | 検出 | 理由 |
|---|---|---|
| `var ErrFoo = errors.New(...)` を直接 return | ✅ | パッケージレベル Sentinel |
| `fmt.Errorf("...: %w", ErrFoo)` を return | ✅ | `%w` でラップされた Sentinel |
| 複数 Sentinel を条件分岐で return | ✅ | Phi ノードの全経路を追跡 |
| `errors.New("...")` をその場で return | ❌ | 無名エラーは Sentinel でない |
| `return nil` | ❌ | nil は Sentinel でない |
| `%v` でラップされた Sentinel | ❌ | `%v` は元エラーの同一性を失う |
| インターフェース経由の動的ディスパッチ | ❌ | フェーズ2（RTA）で対応予定 |

## テスト実行

```bash
# 全テスト
go test ./analyzer/... -v

# 特定ケースのみ
go test ./analyzer/ -run TestAnalyzer_Basic -v
go test ./analyzer/ -run TestAnalyzer_Wrapped -v
go test ./analyzer/ -run TestAnalyzer_Phi -v
```

## テストフィクスチャの追加方法

`analyzer/testdata/src/<カテゴリ>/` にGoファイルを置き、検出してほしい関数に `// want` コメントを付けます。

```go
// 診断のみ
func MyFunc() error { // want `returns sentinels: mypkg\.ErrFoo`
    return ErrFoo
}

// 診断 + Factの両方を検証する場合
func MyFunc() error { // want `returns sentinels: mypkg\.ErrFoo` MyFunc:`SentinelFact\(mypkg\.ErrFoo\)`
    return ErrFoo
}
```

パターンは正規表現（Go `regexp` 構文）で、`// want` の右辺に書きます。

## アーキテクチャ

```
analyzer/
├── analyzer.go   # analysis.Analyzer 定義、エントリポイント
├── facts.go      # SentinelFact（analysis.Fact）と SentinelInfo 型
├── trace.go      # SSA 後方探索（Return → Global の経路追跡）
├── unwrap.go     # fmt.Errorf %w の varargs スライス展開
└── known.go      # 標準ライブラリ既知エラーのマッピング
```

### 探索アルゴリズム

`*ssa.Return` の各結果値から後方に辿り、`Err` で始まるパッケージレベルグローバル変数（`*ssa.Global`）に到達したものを Sentinel として記録します。

| SSA ノード | 処理 |
|---|---|
| `*ssa.Global` | `Err` プレフィクスなら Sentinel として記録 |
| `*ssa.UnOp`（`*`） | グローバルの deref → `*ssa.Global` へ |
| `*ssa.MakeInterface` | 変換前の具象値へ |
| `*ssa.ChangeInterface` | 変換前の値へ |
| `*ssa.Phi` | 全エッジを並行追跡 |
| `*ssa.Call`（`fmt.Errorf %w`） | varargs スライスから `%w` 引数を取り出して継続 |
| `*ssa.Const`（nil） | スキップ |
| その他 | 探索打ち切り |

循環を防ぐため `visited map[ssa.Value]bool` で訪問済み管理、探索深度上限は `maxTraceDepth = 5`。

## 制限事項（フェーズ1）

- **同一パッケージ内のみ解析**。別パッケージの関数が返す Sentinel は追跡しない（フェーズ2でFact連携により対応予定）
- **インターフェース経由の呼び出し**は追跡しない
- **標準ライブラリ**は `analyzer/known.go` の静的マッピングのみ対応（`io.EOF`、`sql.ErrNoRows` など）
