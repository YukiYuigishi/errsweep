package repository

import (
	"database/sql"
	"fmt"
)

// Article は記事エンティティ。
type Article struct {
	ID    int
	Title string
}

// ArticleNotFoundError は記事が見つからないことを表すカスタムエラー型。
// pointer receiver で error interface を実装する。
//
// 呼び出し側は errors.As で ID 付きのコンテキスト情報を取り出せる。
//
//	var nfe *repository.ArticleNotFoundError
//	if errors.As(err, &nfe) { log.Printf("missing article %d", nfe.ID) }
type ArticleNotFoundError struct {
	ID int
}

func (e *ArticleNotFoundError) Error() string {
	return fmt.Sprintf("article not found: id=%d", e.ID)
}

// ArticleValidationError は値型の受信者で実装したバリデーションエラー。
type ArticleValidationError struct {
	Field string
}

func (e ArticleValidationError) Error() string {
	return "article: invalid field: " + e.Field
}

// FindArticleByID は記事を ID で取得する。
//
// 検出される Sentinel:
//   - *repository.ArticleNotFoundError : id が 0 以下（カスタム型直接 return）
//   - sql.ErrNoRows                    : DB に該当行なし（known map 経由）
func FindArticleByID(db *sql.DB, id int) (Article, error) {
	if id <= 0 {
		return Article{}, &ArticleNotFoundError{ID: id}
	}
	var a Article
	err := db.QueryRow(
		"SELECT id, title FROM articles WHERE id = ?", id,
	).Scan(&a.ID, &a.Title)
	if err != nil {
		return Article{}, fmt.Errorf("FindArticleByID: %w", err)
	}
	return a, nil
}

// CreateArticle は記事を登録する。title が空ならバリデーションエラー。
//
// 検出される Sentinel:
//   - repository.ArticleValidationError : title が空（値型 return）
func CreateArticle(db *sql.DB, title string) (int64, error) {
	if title == "" {
		return 0, ArticleValidationError{Field: "title"}
	}
	res, err := db.Exec("INSERT INTO articles (title) VALUES (?)", title)
	if err != nil {
		return 0, fmt.Errorf("CreateArticle: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}
