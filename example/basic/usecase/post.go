package usecase

import (
	"database/sql"
	"fmt"

	"example.com/myapp/repository"
)

// GetPost は投稿を取得する。
//
// 検出される Sentinel:
//   - repository.ErrPostNotFound : id が 0 以下またはDBに存在しない
//   - database/sql.ErrNoRows     : DB に存在しない
func GetPost(db *sql.DB, id int) (repository.Post, error) {
	p, err := repository.FindPostByID(db, id)
	if err != nil {
		return repository.Post{}, fmt.Errorf("GetPost: %w", err)
	}
	return p, nil
}

// RemovePost は投稿を削除する。
// クロスパッケージ Fact 伝播により repository.DeletePost の Sentinel をすべて引き継ぐ。
//
// 検出される Sentinel:
//   - repository.ErrPostNotFound : 投稿が存在しない
//   - repository.ErrForbidden    : 操作者が投稿者でない
//   - database/sql.ErrNoRows     : DB に存在しない
func RemovePost(db *sql.DB, postID, requesterID int) error {
	if err := repository.DeletePost(db, postID, requesterID); err != nil {
		return fmt.Errorf("RemovePost: %w", err)
	}
	return nil
}
