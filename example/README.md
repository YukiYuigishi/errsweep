# Example module

`example/` は、sentinelfind が現場コードに近い構造でどう見えるかを確認するためのサンプルです。

## 構成

- `repository/`
  - DB / HTTP など外部I/Oを扱う層
  - sentinel を定義し、`fmt.Errorf("Func: %w", err)` でラップして返す
- `usecase/`
  - 複数 repository を組み合わせる orchestration 層
  - 関数変数 DI / interface DI の両方を含む

## 実行例

```bash
cd example
go test ./...
cd ..
./sentinelfind ./example/...
```

## 実運用寄りケース

- `usecase/integration_story.go`
  - `repository.FetchTagNameFromUpstream`（HTTP + context + io.EOF）
  - `repository.ResolveTagID`（database/sql.ErrNoRows）
  - usecase では依存呼び出しの順序制御とエラー文脈付与に集中

