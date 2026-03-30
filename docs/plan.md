# 実装計画: go-sentinel-analyzer

Go関数が返しうるSentinel Errorを静的解析で抽出・列挙するカスタムAnalyzer。

## 背景と動機

Go言語ではエラーが「値」として返される設計のため、関数が内部でどのSentinel Errorを返しうるかをソースコードから読み取る認知負荷が高い。現状の gopls や golangci-lint エコシステムにはこの問題を解決する汎用ツールが存在しない。

**ゴール**: 関数をホバーまたはCIで解析した際に「この関数は `io.EOF`, `sql.ErrNoRows` を返しうる」と提示できるようにする。

---

## フェーズ1: Intra-procedural Analyzer（単一関数内解析）

**期間**: 2〜3週間
**目標**: 対象関数が直接 return しているSentinel Errorを抽出する

### 1.1 Analyzer骨格の作成

- `analyzer/analyzer.go` に `analysis.Analyzer` を定義
- `buildssa.Analyzer` を `Requires` に指定してSSA中間表現を取得
- `analysis.Fact` として `SentinelFact{Errors []SentinelInfo}` を定義し、関数オブジェクトにエクスポート

```go
type SentinelInfo struct {
    PkgPath string // e.g. "io"
    Name    string // e.g. "EOF"
}

type SentinelFact struct {
    Errors []SentinelInfo
}

func (*SentinelFact) AFact() {} // analysis.Fact marker

var Analyzer = &analysis.Analyzer{
    Name:      "sentinelfind",
    Doc:       "reports sentinel errors a function may return",
    Run:       run,
    Requires:  []*analysis.Analyzer{buildssa.Analyzer},
    FactTypes: []analysis.Fact{(*SentinelFact)(nil)},
}
```

### 1.2 後方探索（Backward Trace）の実装

`trace.go` にSSA値の後方探索ロジックを実装する。

**探索対象のSSAノードと遷移ルール:**

| ノード | アクション |
|---|---|
| `*ssa.UnOp` (Deref of `*ssa.Global`) | → Sentinel特定。`Global.Pkg()` と `Global.Name()` を記録して終了 |
| `*ssa.MakeInterface` | → `.X`（変換前の具象値）へ遷移 |
| `*ssa.Phi` | → `.Edges` の全オペランドを並行追跡 |
| `*ssa.Call` (`fmt.Errorf` + `%w`) | → `%w` に対応する引数を特定して遷移 |
| `*ssa.Call` (`errors.New`) | → Sentinelではない匿名エラー。スキップ |
| `*ssa.Extract` | → Tupleの該当インデックスの定義元へ遷移 |
| `*ssa.Const` (nil) | → nil return。スキップ |
| その他 | → 探索打ち切り |

**循環検出**: `visited map[ssa.Value]bool` で同一ノードの再訪問を防止。

### 1.3 fmt.Errorf %w のアンラップ

`unwrap.go` に `fmt.Errorf` 呼び出しの特殊処理を実装する。

- `*ssa.Call` の `StaticCallee()` が `fmt.Errorf` かを判定
- 第1引数（フォーマット文字列）が `*ssa.Const` なら文字列を取得し `%w` の位置を特定
- 対応する可変長引数のインデックスを算出し、そのオペランドに対して探索を継続

### 1.4 テストデータの整備

`testdata/src/` 配下に `analysistest` 用フィクスチャを作成する。

```
testdata/src/basic/basic.go        - var ErrX を直接 return
testdata/src/wrapped/wrapped.go    - fmt.Errorf("%w", ErrX) で return
testdata/src/phi/phi.go            - if 分岐で異なるSentinelを return
testdata/src/nilreturn/nil.go      - return nil, nil（検出対象外の確認）
testdata/src/opaque/opaque.go      - errors.New("...") のみ（検出対象外の確認）
```

各ファイルの関数に `// want "returns sentinel: io.EOF"` 形式のコメントを付与。

### 1.5 CLI の作成

```go
// cmd/sentinelfind/main.go
func main() {
    singlechecker.Main(analyzer.Analyzer)
}
```

### フェーズ1 完了基準

- [ ] 直接 return されたSentinel Errorを正しく抽出できる
- [ ] `fmt.Errorf("%w", sentinel)` でラップされたSentinelを抽出できる
- [ ] `Phi` ノード経由の複数Sentinelを全て列挙できる
- [ ] `errors.New("...")` の匿名エラーはSentinelとして報告しない
- [ ] `return nil, nil` を誤検知しない
- [ ] analysistest が全パス

---

## フェーズ2: Shallow Inter-procedural（浅いプロシージャ間解析）

**期間**: 1〜2ヶ月
**目標**: 同一モジュール内の静的関数呼び出しを越えてSentinelを追跡する

### 2.1 呼び出し先関数への再帰探索

`*ssa.Call` ノードに到達した際の分岐ロジック:

```
1. StaticCallee() == nil → インターフェース呼び出し。スキップ
2. 既知エラーマッピング（known.go）にヒット → マッピング結果を返す
3. 同一モジュール内の関数 → callee の関数本体に再帰（depth+1）
4. 外部モジュールの関数 → スキップ（将来のFact連携で対応）
5. depth > maxTraceDepth → 打ち切り
```

### 2.2 Factキャッシュによるサマリー方式

NilAwayの推論グラフに着想を得た設計:

- 各関数の解析結果を `SentinelFact` として `ExportObjectFact` でキャッシュ
- 呼び出し先関数に既にFactが存在すれば、関数本体の再探索をスキップ
- `analysis` フレームワークのパッケージ間Factパッシングにより、依存パッケージの結果も自動的に引き継がれる

### 2.3 標準ライブラリ既知エラーマッピング

`known.go` に標準ライブラリの頻出関数→Sentinelのマッピングをハードコードする。

```go
var knownErrorMap = map[string][]SentinelInfo{
    "(*os.File).Read":       {{PkgPath: "io", Name: "EOF"}},
    "(*database/sql.DB).QueryRow": {},  // ErrNoRows は Scan() で返る
    "(*database/sql.Row).Scan":    {{PkgPath: "database/sql", Name: "ErrNoRows"}},
    "io.ReadAll":            {{PkgPath: "io", Name: "EOF"}},  // 内部で吸収されるが参考情報として
    // ...段階的に拡充
}
```

初期は手動メンテナンス。将来的にはフェーズ2のAnalyzer自身で標準ライブラリを解析して自動生成する。

### 2.4 探索深度の制御

```go
const maxTraceDepth = 5
```

根拠: 一般的なGoアプリケーションでは Handler → UseCase → Repository → Driver の4階層。余裕を持たせて5。プロファイリングで調整する。

### フェーズ2 完了基準

- [ ] 同一パッケージ内の関数呼び出しを越えてSentinelを追跡できる
- [ ] 同一モジュール内の別パッケージの関数呼び出しを越えて追跡できる
- [ ] Factキャッシュにより同一関数の再解析が発生しない
- [ ] 標準ライブラリの主要関数（io, os, database/sql, net/http）のマッピングが整備されている
- [ ] 探索深度上限で無限再帰が発生しない
- [ ] 循環呼び出し（A→B→A）で無限ループしない
- [ ] 1万行規模のプロジェクトで10秒以内に解析完了

---

## フェーズ3: gopls / エディタ連携

**期間**: 1〜2ヶ月
**目標**: エディタ上で関数ホバー時にSentinel Error一覧を表示する

### 3.1 アプローチ選定

| 方式 | メリット | デメリット |
|---|---|---|
| A. gopls にAnalyzerを組み込み | ネイティブ統合。Fact活用可能 | gopls のフォーク保守が必要 |
| B. LSPプロキシサーバー | gopls非依存。保守コスト低 | Hover応答の改変がハック的 |
| C. エディタ拡張（VS Code Extension） | 最も独立性が高い | エディタごとに実装が必要 |

**推奨: 方式B → 将来的にAへ移行**

### 3.2 LSPプロキシの設計

```
Editor ←→ sentinel-lsp-proxy ←→ gopls
                ↓
        sentinelfind (事前解析結果のJSON/SQLiteキャッシュ)
```

- `textDocument/hover` レスポンスをインターセプト
- 対象関数の `SentinelFact` をキャッシュから検索
- gopls のMarkdown応答に `---\n**Possible Sentinel Errors:**\n- io.EOF\n- sql.ErrNoRows` を追記

### 3.3 キャッシュ戦略

- ワークスペース初回オープン時にバックグラウンドでフル解析を実行
- ファイル保存時に変更パッケージのみ差分再解析
- 結果はSQLiteに格納（`package, func_name, sentinel_pkg, sentinel_name`）

### フェーズ3 完了基準

- [ ] VS Code + gopls 環境で関数ホバー時にSentinel Error一覧が表示される
- [ ] ファイル保存後5秒以内にキャッシュが更新される
- [ ] gopls の既存機能（型情報、ドキュメント）を壊さない

---

## フェーズ4: コードベース設計改善の支援Linter（並行実施）

**期間**: 継続的
**目標**: 解析精度を高めるコード規約をLinterで強制する

### 4.1 layer-error-boundary Linter

レイヤー境界（設定ファイルで定義）を越えて下位パッケージのSentinelがそのまま返されている場合に警告する。

```yaml
# .sentinelfind.yaml
layers:
  - name: handler
    packages: ["github.com/myapp/internal/handler/..."]
  - name: usecase
    packages: ["github.com/myapp/internal/usecase/..."]
  - name: repository
    packages: ["github.com/myapp/internal/repository/..."]

rules:
  - from: handler
    deny_direct_sentinel_from: repository  # repository層のSentinelがhandler層に漏れたら警告
```

### 4.2 sentinel-declaration Linter

- `var Err* = errors.New(...)` が関数内ローカルで宣言されている場合に「パッケージレベルに移動すべき」と警告
- Sentinel宣言にgodocコメントがない場合に警告

---

## リスクと軽減策

| リスク | 影響 | 軽減策 |
|---|---|---|
| SSA API の破壊的変更 | Analyzerが動作不能に | `go/ssa` のバージョンをpinし、CI で定期ビルド |
| 誤検知（false positive）の多発 | ユーザーの信頼喪失 | フェーズ1で保守的な検出に徹する。`// sentinel:ignore` コメントで抑制可能にする |
| gopls 内部APIの変更 | フェーズ3のプロキシが破損 | 方式Bで gopls 本体への依存を最小化 |
| インターフェース経由の未追跡 | 検出漏れ | ドキュメントで制限事項として明記。フェーズ4のLinterで設計改善を促す |
| 大規模モノレポでのパフォーマンス | CI のボトルネック | Factキャッシュ + 差分解析。プロファイリングで `maxTraceDepth` を調整 |

---

## 成功指標

- フェーズ1完了時点で、テストプロジェクトにおいてSentinel Errorの **80%以上** を正しく抽出
- フェーズ2完了時点で、false positive率が **5%以下**
- フェーズ3完了時点で、Hover応答のレイテンシ増加が **50ms以下**
