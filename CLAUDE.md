# go-sentinel-analyzer

Go関数が返しうる定義済みエラー（Sentinel Error）を静的解析で抽出するカスタムAnalyzer。

## プロジェクト概要

- `go/analysis` フレームワーク上で動作するAnalyzer
- SSA（静的単一代入）中間表現を用いた後方データフロー解析で `*ssa.Return` → `*ssa.Global` のパスを追跡
- golangci-lint カスタムLinter / gopls 連携を最終目標とする

## 技術スタック

- Go 1.26+
- `golang.org/x/tools/go/analysis` - Analyzerフレームワーク
- `golang.org/x/tools/go/ssa` + `golang.org/x/tools/go/ssa/ssautil` - SSA構築
- `golang.org/x/tools/go/analysis/passes/buildssa` - analysis経由のSSA取得
- `golang.org/x/tools/go/callgraph/rta` - コールグラフ構築（フェーズ2）

## ディレクトリ構成

```
.
├── cmd/
│   └── sentinelfind/        # スタンドアロンCLI（singlechecker/multichecker）
│       └── main.go
├── analyzer/
│   ├── analyzer.go          # analysis.Analyzer 定義・エントリポイント
│   ├── analyzer_test.go     # analysistest ベースのテスト
│   ├── trace.go             # SSA後方探索ロジック
│   ├── trace_test.go
│   ├── unwrap.go            # fmt.Errorf %w / カスタムラッパーのアンラップ
│   ├── facts.go             # analysis.Fact 定義（SentinelFact）
│   └── known.go             # 標準ライブラリ既知エラーのマッピングテーブル
├── testdata/                # analysistest 用テストフィクスチャ
│   └── src/
│       ├── basic/           # 基本的なSentinel return
│       ├── wrapped/         # fmt.Errorf %w パターン
│       ├── phi/             # if-else 分岐でのPhi合流
│       ├── interprocedural/ # 関数呼び出し越えの追跡
│       └── interface/       # インターフェース経由（検出限界のテスト）
├── docs/
│   └── plan.md
├── tmp # 外部のgoが使われているプロジェクトをcloneして動作検証
├── go.mod
├── go.sum
└── .CLAUDE.md
```

## コーディング規約

- t-wadaのTDDで開発する
- エラーは必ず `fmt.Errorf("funcName: %w", err)` でコンテキスト付与して返す
- テストは `analysistest.Run` を使い、testdata/src 配下にGoファイルを配置する
- テストフィクスチャ内の期待値は `// want "..."` コメントで記述する（analysistest標準方式）
- SSA命令のswitch文は網羅性コメントを付ける（`// handled: Return, Global, Call, Phi, MakeInterface, UnOp`）
- 探索深度の上限は定数 `maxTraceDepth` で管理し、ハードコードしない
- Factのシリアライズは `encoding/gob` 互換にする（goplsキャッシュ対応）

## 重要な設計判断

- インターフェース経由の動的ディスパッチは追跡しない（フェーズ2でRTA導入時に再検討）
- `errors.New("...")` のような匿名エラーはSentinelとみなさない（`var Err* =` で宣言されたもののみ対象）
- `fmt.Errorf` の `%w` 動詞のみをラップとして認識する（`%v` は元エラーの同一性が失われるため除外）
- 循環呼び出しは visited set で検出し、無限ループを防止する

## よく使うコマンド

```bash
# テスト実行
go test ./analyzer/... -v

# テストデータのみ実行
go test ./analyzer/ -run TestAnalyzer -v

# スタンドアロンCLIビルド・実行
go build -o sentinelfind ./cmd/sentinelfind
./sentinelfind ./path/to/target/package

# analysistest のデバッグ（SSAダンプ付き）
GODEBUG=ssadump=1 go test ./analyzer/... -v -run TestTrace
```
