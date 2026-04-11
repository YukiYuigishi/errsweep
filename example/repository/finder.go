package repository

import "database/sql"

// UserFinder はユーザー検索関数の型。
// パッケージレベルの変数として保持することで、DI（依存性注入）パターンを実現する。
// テストではモック実装に差し替えることができる。
type UserFinder func(db *sql.DB, id int) (User, error)

// UserCreator はユーザー登録関数の型。
type UserCreator func(db *sql.DB, name, email string) (int64, error)

// UserDeleter はユーザー削除関数の型。
type UserDeleter func(db *sql.DB, id int) error
