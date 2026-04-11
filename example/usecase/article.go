package usecase

import (
	"database/sql"
	"fmt"

	"example.com/myapp/repository"
)

// GetArticle はクロスパッケージ Fact 伝播により
// repository.FindArticleByID が返すカスタム型エラーを引き継ぐ。
//
// 検出される Sentinel:
//   - *repository.ArticleNotFoundError : 記事が存在しない
//   - sql.ErrNoRows                    : DB に該当行なし
func GetArticle(db *sql.DB, id int) (repository.Article, error) {
	a, err := repository.FindArticleByID(db, id)
	if err != nil {
		return repository.Article{}, fmt.Errorf("GetArticle: %w", err)
	}
	return a, nil
}

// AddArticle は repository.CreateArticle を呼ぶ。
//
// 検出される Sentinel:
//   - repository.ArticleValidationError : title バリデーションエラー
func AddArticle(db *sql.DB, title string) (int64, error) {
	id, err := repository.CreateArticle(db, title)
	if err != nil {
		return 0, fmt.Errorf("AddArticle: %w", err)
	}
	return id, nil
}
