# 実装計画（更新版）: go-sentinel-analyzer

Go関数が返しうる Sentinel Error を静的解析し、CLI・LSP hover で提示する。

## 現在の到達点（2026-04時点）

### 1. 基盤機能（完了）

- `analysis.Analyzer` + `buildssa` を使った SSA 解析基盤
- `SentinelFact` の Export/Import による関数サマリー伝播
- `cmd/sentinelfind` による単体実行（通常/JSON）

### 2. 解析ロジック（実装済み）

- 単一関数内解析（`Return`, `UnOp`, `Phi`, `MakeInterface`, `Extract`, `Const(nil)`）
- `fmt.Errorf(...%w...)` のアンラップ（varargs SSA パターン対応）
- `errors.New` 由来の匿名エラー除外
- 再帰呼び出し・循環呼び出し対策（`visited`, `visitedFuncs`, `maxTraceDepth`）
- 関数間解析（静的 call / cross-package Fact 利用）
- 既知標準ライブラリエラーマップ（`io`, `bufio`, `os`, `database/sql` 主要パス）
- DI系の追加追跡:
  - 関数変数経由呼び出し（`var f FuncType = concrete`）
  - interface invoke の具象解決（compile-time assertion ベース）
- カスタムエラー型（exported named type）を Sentinel として扱う拡張

### 3. ツール連携（実装済み）

- `sentinel-lsp-proxy`（gopls hover 応答へ Sentinel 情報を追記）
- `sentinel-lsp`（hover 専用ミニマル LSP）
- Neovim / VS Code の E2E テスト導線（Makefile + scripts）

### 4. テスト整備（実装済み）

- `analysistest` フィクスチャ:
  `basic`, `wrapped`, `phi`, `nilreturn`, `opaque`, `deferred`,
  `interprocedural`, `stdlib`, `callee/caller`, `funcvar`, `ifacecallee/ifacecaller`, `customtype`
- CLI / proxy / LSP の単体テスト

---

## 更新後の目標

当初の「PoC 構築」段階は完了。今後は **精度・運用性・拡張性** を主目標に進める。

1. **精度向上**: 取りこぼしと誤検知を減らし、実コードでの信頼性を上げる  
2. **運用性能**: 大規模コードベースでも実用速度を維持する  
3. **利用体験**: エディタ連携を安定化し、導入コストを下げる

---

## 次フェーズ（優先度順）

## フェーズA: 解析精度の強化（最優先）

- 完了（2026-04 更新）
  - interface invoke 解決を SSA 値フロー + 実装マップ + RTA runtime type 絞り込みで補強
  - `%w` 以外の `fmt.Errorf` を同一性喪失として明示診断
  - known error map を `net/http`, `net`, `syscall`, `context` まで段階拡張し、回帰テストで固定化

**完了条件**

- 代表ケースでの誤検知/見逃し傾向が可視化され、改善サイクルが回る状態（達成）
- interface 経由ケースの検出率向上がテストで確認できる状態（達成）

## フェーズB: キャッシュ・性能最適化

- キャッシュ更新の粒度改善（差分再解析）
- 解析結果キャッシュ形式の改善（JSON中心 → 必要なら SQLite 併用）
- 大規模リポジトリ向けの計測・ボトルネック分析

**完了条件**

- 解析時間の回帰検知ができる
- キャッシュ再利用時の体感待ち時間が短い

## フェーズC: LSP 体験の安定化

- proxy 経由 hover 追記の堅牢化（失敗時の劣化挙動を安定）
- エディタ別の設定テンプレートと検証ケース拡充
- hover 表示文言の最適化（情報量と可読性のバランス）

**完了条件**

- VS Code / Neovim の標準的設定で再現性高く利用可能
- hover 追記が既存 gopls 体験を阻害しない

## フェーズD: 設計支援 Linter（並行検討）

- layer boundary 越境 sentinel 伝播の警告
- sentinel 宣言品質ルール（宣言位置・コメント等）

---

## 非ゴール（現時点）

- 完全な動的ディスパッチ解決（100% の call target 解決）
- すべての外部ライブラリに対する網羅的 sentinel 推定
- gopls 本体フォークへの即時統合

---

## 成功指標（更新）

- Analyzer の新規改善が `analysistest` と E2E で継続的に検証される
- 主要ユースケース（CLI + proxy hover）での誤検知報告が減少傾向
- 実プロジェクト適用時に「どの関数が何を返すか」を短時間で判断できる状態を維持する
