# Example module

`example/` は、`sentinelfind` と hover 連携を試すための公開向けサンプルモジュールです。

## 目的

1. Sentinel Error の解析結果を CLI で確認する
2. エディタ hover で `Possible Sentinel Errors` が表示されることを確認する
3. interface 経由呼び出し・呼び出し側 hover の挙動を再現する

## ディレクトリ構成

| パス | 役割 |
|---|---|
| `repository/` | DB / HTTP など外部I/Oを扱う実運用寄りサンプル |
| `usecase/` | `repository` を束ねるユースケース層（interface DI 含む） |
| `catalogrepo/` | 呼び出し側 hover 再現用の被呼び出しパッケージ |
| `catalogservice/` | `catalogrepo` を呼ぶ呼び出し側パッケージ |
| `.vscode/settings.json` | `sentinel-lsp-proxy` 前提の VS Code 設定例 |

## 1. CLI で解析結果を確認

```bash
cd example
go test ./...
../sentinelfind ./...
```

JSON 形式で確認する場合:

```bash
../sentinelfind -json ./... > /tmp/example-sentinelfind.json
```

## 2. VS Code で hover を確認

リポジトリルートで以下を実行:

```bash
make build
go install ./cmd/sentinel-lsp-proxy
go install ./cmd/sentinelfind
```

その後、VS Code で `example/` を開いてウィンドウを再読み込みし、
関数呼び出しや定義位置に hover すると `Possible Sentinel Errors` が追記されます。

表示されない場合の確認:

```bash
command -v sentinel-lsp-proxy
command -v sentinelfind
```

## 3. 呼び出し側 hover 再現ケース（catalogservice -> catalogrepo）

- `catalogrepo/find_product.go`
  - `FindProduct` が内部で interface invoke を実行
  - `ErrProductNotFound`, `ErrProductArchived` を返しうる
- `catalogservice/resolve_product.go`
  - `ProductService` が `ProductFinder` を DI で保持
  - `(*ProductService).ResolveProductFromCatalog` から `catalogrepo.FindProduct(...)` を呼び出す

`catalogservice/resolve_product.go` の
`catalogrepo.FindProduct(s.finder, id)` 呼び出し位置で hover し、
`Possible Sentinel Errors` に次が表示されることを確認できます。

- `catalogrepo.ErrProductNotFound`
- `catalogrepo.ErrProductArchived`
