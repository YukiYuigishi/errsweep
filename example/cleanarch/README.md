# Clean Architecture Example

`cleanarch/` は、DI やインターフェース抽象化が多用された実運用寄りの API サーバー構造で、
errsweep の **検出能力と限界** を体系的に示すためのサンプルです。

## 目的

1. errsweep がどこまで検出でき、どこから検出できないかの境界を明確にする
2. AI アシスタントが errsweep の能力を正確に把握するための参照資料として使う
3. Clean Architecture / DDD の典型的な DI パターンと errsweep の相性を示す

## アーキテクチャ

```
handler/        <- プレゼンテーション層（HTTP ハンドラ、ミドルウェア）
  | 依存
di/             <- DI コンテナ（依存関係の組み立て）
  | 依存
application/    <- ユースケース層（ビジネスロジックのオーケストレーション）
  | 依存
domain/         <- ドメイン層（エンティティ、値オブジェクト、インターフェース定義）
  ^ 実装
infra/          <- インフラ層（DB、外部 API の具象実装）
```

## パターン一覧と検出可否

| ID | パターン | ファイル | 検出 |
|----|---------|---------|------|
| A | compile-time assertion による interface 解決 | `application/place_order.go` | **可能** |
| B | 複数 interface 連携（OrderRepository + PaymentGateway） | `application/cancel_order.go` | **可能** |
| C | CQRS Query - 複数 concrete の breakdown | `application/query_order.go` | **可能** |
| D-1 | クロージャ / 関数パラメータ | `application/undetectable.go` | **不可能** |
| D-2 | ファクトリ関数が interface を返す（非エクスポート具象） | `application/undetectable.go` | **不可能** |
| D-3 | カスタムエラーラッパー（fmt.Errorf %w 以外） | `application/undetectable.go` | **union 断絶** *1 |
| D-4 | map ベースのディスパッチ | `application/undetectable.go` | **不可能** |
| D-5 | メソッド値のコールバック | `application/undetectable.go` | **不可能** |
| D-6 | fmt.Errorf %v（identity 喪失） | `application/undetectable.go` | **警告 + breakdown** *1 |
| E | 関数変数 DI（var f = ConcreteFunc） | `application/stock_service.go` | **可能** |
| F | 動的レジストリ（map[string]any） | `di/registry.go` | **不可能** |
| G | 静的 DI コンテナ経由（具象型フィールド） | `handler/order_handler.go` | **可能** |
| H | Registry -> 具象型 TypeAssert | `handler/order_handler.go` | **可能** *2 |
| I | interface フィールド DI（assertion 有無で変動） | `handler/order_handler.go` | **条件付き** |
| J | ミドルウェアチェーン（クロージャ） | `handler/middleware.go` | **部分的** *3 |
| K | Registry -> interface 型 TypeAssert | `handler/order_handler.go` | **条件付き** |
| - | デコレータパターン（inner interface 委譲） | `infra/persistence/cached_order_repo.go` | **条件付き** |
| - | 非エクスポート Sentinel（errValidation） | `domain/order/errors.go` | **不可能** |
| - | 動的 errors.New（関数内生成） | `domain/order/errors.go` | **不可能** |
| - | 非エクスポート concrete 型 | `application/undetectable.go` | **不可能** |

### 注記

- ***1 union 断絶**: カスタムラッパーや `%v` は union sentinel の連鎖を切断するが、`collectInvokeBreakdown` による concrete ごとの内訳は別途報告される。union と breakdown の乖離が発生するパターン。
- ***2 具象型 TypeAssert**: `svc.(*ConcreteType)` のアサーション後は SSA 上で静的呼び出しになるため、errsweep は追跡可能。interface 型へのアサーション（パターン K）との対比に注意。
- ***3 部分的**: ミドルウェア自身の sentinel（ErrUnauthorized 等）は検出されるが、`next(ctx)` のクロージャパラメータ経由の sentinel は追跡不可能。

## 検出可能の条件

errsweep が interface 経由の sentinel を検出するには、以下のいずれかが必要:

1. **compile-time assertion**: `var _ Interface = (*Concrete)(nil)` が解析対象パッケージに存在する
2. **auto-discovery**: 具象型が解析対象パッケージまたは直接インポート先のスコープに存在し、`types.Implements` が真
3. **RTA (Rapid Type Analysis)**: ランタイムで具象型がインスタンス化されるコードパスが到達可能

### 検出不可能になるケース

- 関数パラメータ / クロージャ経由の呼び出し（SSA 上で静的に解決不可能）
- `map` / `any` から取り出した値の interface メソッド呼び出し（具象型アサーション後は検出可能）
- `fmt.Errorf` の `%w` 以外によるエラーラップ（`%v`、独自 Wrap 関数）
- 非エクスポート変数（`errFoo`）や関数内 `errors.New`
- 探索深度が `maxTraceDepth`（5）を超えるコールチェーン
- 非エクスポート具象型は compile-time assertion / auto-discovery の対象外

## union と breakdown の違い

errsweep は 2 種類の情報を出力する:

- **union（合算）**: return 文から SSA を逆方向に辿って到達した sentinel の集合。Fact エクスポートに使われる。
- **breakdown（内訳）**: 関数内の interface invoke 呼び出しを走査し、concrete ごとの sentinel を報告。

カスタムラッパーや `%v` は union の連鎖を断ち切るが、breakdown は invoke を直接走査するため影響を受けない。
これにより「union には出ないが breakdown には出る」という乖離が発生しうる。

## 実行方法

```bash
cd example/cleanarch

# CLI で解析
../../errsweep ./...

# JSON 出力
../../errsweep -json ./...
```
