package repository

import (
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrPostNotFound = errors.New("post not found")
	ErrForbidden    = errors.New("forbidden")
)

// Post は投稿エンティティ。
type Post struct {
	ID       int
	AuthorID int
	Title    string
	Body     string
}

// FindPostByID は投稿を ID で取得する。
//
// 検出される Sentinel:
//   - ErrPostNotFound : id が 0 以下（直接 return）
//   - sql.ErrNoRows   : DB に該当行が存在しない（known map 経由）
//
// Phi ノード: 上記2つが異なるブロックで合流する。
func FindPostByID(db *sql.DB, id int) (Post, error) {
	if id <= 0 {
		return Post{}, ErrPostNotFound
	}
	var p Post
	err := db.QueryRow(
		"SELECT id, author_id, title, body FROM posts WHERE id = ?", id,
	).Scan(&p.ID, &p.AuthorID, &p.Title, &p.Body)
	if err != nil {
		return Post{}, fmt.Errorf("FindPostByID: %w", err)
	}
	return p, nil
}

// DeletePost は投稿を削除する。
//
// 検出される Sentinel:
//   - ErrPostNotFound : 投稿が存在しない（FindPostByID 経由 + fmt.Errorf %w ラップ）
//   - sql.ErrNoRows   : 投稿が存在しない（FindPostByID 経由 + fmt.Errorf %w ラップ）
//   - ErrForbidden    : 操作者が投稿者でない（直接 return）
//
// パッケージ内呼び出し + Phi: 3種の Sentinel が合流する。
func DeletePost(db *sql.DB, id, requesterID int) error {
	p, err := FindPostByID(db, id)
	if err != nil {
		return fmt.Errorf("DeletePost: %w", err)
	}
	if p.AuthorID != requesterID {
		return ErrForbidden
	}
	if _, err := db.Exec("DELETE FROM posts WHERE id = ?", id); err != nil {
		return fmt.Errorf("DeletePost: exec: %w", err)
	}
	return nil
}
