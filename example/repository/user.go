package repository

import (
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrNotFound  = errors.New("user not found")
	ErrDuplicate = errors.New("user already exists")
)

// User はユーザーエンティティ。
type User struct {
	ID    int
	Name  string
	Email string
}

// FindUserByID はユーザーを ID で取得する。
//
// 検出される Sentinel:
//   - ErrNotFound   : id が 0 以下（直接 return）
//   - sql.ErrNoRows : DB に該当行が存在しない（known map 経由）
//
// Phi ノード: 上記2つが異なるブロックで合流する。
func FindUserByID(db *sql.DB, id int) (User, error) {
	if id <= 0 {
		return User{}, ErrNotFound
	}
	var u User
	err := db.QueryRow(
		"SELECT id, name, email FROM users WHERE id = ?", id,
	).Scan(&u.ID, &u.Name, &u.Email)
	if err != nil {
		return User{}, fmt.Errorf("FindUserByID: %w", err)
	}
	return u, nil
}

// CreateUser はユーザーを新規登録する。
//
// 検出される Sentinel:
//   - ErrDuplicate  : email が既に存在する（直接 return）
//   - sql.ErrNoRows : 重複チェックの Scan が失敗した場合（known map 経由）
func CreateUser(db *sql.DB, name, email string) (int64, error) {
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM users WHERE email = ?", email,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("CreateUser: count: %w", err)
	}
	if count > 0 {
		return 0, ErrDuplicate
	}
	res, err := db.Exec("INSERT INTO users (name, email) VALUES (?, ?)", name, email)
	if err != nil {
		return 0, fmt.Errorf("CreateUser: insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// DeleteUser はユーザーを削除する。
//
// 検出される Sentinel:
//   - ErrNotFound   : FindUserByID 経由（パッケージ内呼び出し trace）
//   - sql.ErrNoRows : FindUserByID 経由（パッケージ内呼び出し trace）
func DeleteUser(db *sql.DB, id int) error {
	if _, err := FindUserByID(db, id); err != nil {
		return fmt.Errorf("DeleteUser: %w", err)
	}
	if _, err := db.Exec("DELETE FROM users WHERE id = ?", id); err != nil {
		return fmt.Errorf("DeleteUser: exec: %w", err)
	}
	return nil
}
