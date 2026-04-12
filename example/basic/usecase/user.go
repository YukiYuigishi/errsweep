package usecase

import (
	"database/sql"
	"fmt"

	"example.com/myapp/repository"
)

// GetUser はユーザーを取得する。
// クロスパッケージ Fact 伝播により repository.FindUserByID の Sentinel を引き継ぐ。
//
// 検出される Sentinel:
//   - repository.ErrNotFound  : id が 0 以下
//   - database/sql.ErrNoRows  : DB に存在しない
func GetUser(db *sql.DB, id int) (repository.User, error) {
	u, err := repository.FindUserByID(db, id)
	if err != nil {
		return repository.User{}, fmt.Errorf("GetUser: %w", err)
	}
	return u, nil
}

// CreateUser はユーザーを登録する。
//
// 検出される Sentinel:
//   - repository.ErrDuplicate : email が既に存在する
//   - database/sql.ErrNoRows  : 重複チェック時の DB エラー
func CreateUser(db *sql.DB, name, email string) (int64, error) {
	id, err := repository.CreateUser(db, name, email)
	if err != nil {
		return 0, fmt.Errorf("CreateUser: %w", err)
	}
	return id, nil
}

// DeleteUser はユーザーを削除する。
//
// 検出される Sentinel:
//   - repository.ErrNotFound : id が 0 以下またはユーザーが存在しない
//   - database/sql.ErrNoRows : DB に存在しない
func DeleteUser(db *sql.DB, id int) error {
	if err := repository.DeleteUser(db, id); err != nil {
		return fmt.Errorf("DeleteUser: %w", err)
	}
	return nil
}
