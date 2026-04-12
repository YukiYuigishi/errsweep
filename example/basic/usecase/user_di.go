// user_di.go は関数変数 DI パターンの usecase 実装例。
//
// user.go の直接呼び出し版との対比:
//
//	user.go:    repository.FindUserByID(db, id)    // 静的呼び出し
//	user_di.go: findUserByID(db, id)               // 関数変数経由
//
// アナライザは var findUserByID = repository.FindUserByID という初期化を
// init 内の SSA Store 命令から辿り、関数変数越しでも Sentinel を検出する。
package usecase

import (
	"database/sql"
	"fmt"

	"example.com/myapp/repository"
)

// パッケージレベルの関数変数（DI パターン）。
// テストやモックでは差し替えることができる。
var (
	findUserByID repository.UserFinder  = repository.FindUserByID
	createUser   repository.UserCreator = repository.CreateUser
	deleteUser   repository.UserDeleter = repository.DeleteUser
)

// GetUserDI は関数変数経由でユーザーを取得する。
// アナライザは findUserByID → repository.FindUserByID と辿り、
// 返しうる Sentinel を検出する。
//
// 検出される Sentinel:
//   - repository.ErrNotFound  : id が 0 以下
//   - database/sql.ErrNoRows  : DB に存在しない
func GetUserDI(db *sql.DB, id int) (repository.User, error) {
	u, err := findUserByID(db, id)
	if err != nil {
		return repository.User{}, fmt.Errorf("GetUserDI: %w", err)
	}
	return u, nil
}

// CreateUserDI は関数変数経由でユーザーを登録する。
//
// 検出される Sentinel:
//   - repository.ErrDuplicate : email が既に存在する
//   - database/sql.ErrNoRows  : 重複チェック時の DB エラー
func CreateUserDI(db *sql.DB, name, email string) (int64, error) {
	id, err := createUser(db, name, email)
	if err != nil {
		return 0, fmt.Errorf("CreateUserDI: %w", err)
	}
	return id, nil
}

// DeleteUserDI は関数変数経由でユーザーを削除する。
//
// 検出される Sentinel:
//   - repository.ErrNotFound : id が 0 以下またはユーザーが存在しない
//   - database/sql.ErrNoRows : DB に存在しない
func DeleteUserDI(db *sql.DB, id int) error {
	if err := deleteUser(db, id); err != nil {
		return fmt.Errorf("DeleteUserDI: %w", err)
	}
	return nil
}
