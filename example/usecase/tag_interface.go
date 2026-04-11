// tag_interface.go はインターフェース DI パターンの usecase 実装例。
//
// 関数パラメータとして interface を受け取ることでテスト時はモックに差し替えられる。
// アナライザは compile-time assertion から具象型を解決し、
// インターフェースメソッド呼び出し越しに Sentinel を検出する。
package usecase

import (
	"fmt"

	"example.com/myapp/repository"
)

// TagRepository はタグ操作のインターフェース。
type TagRepository interface {
	FindTagByID(id int) (repository.Tag, error)
	CreateTag(name string) (int64, error)
	DeleteTag(id int) error
}

// compile-time assertion: 具象型 *repository.TagRepository が
// インターフェース TagRepository を満たすことを宣言する。
// アナライザはこの宣言から interface → concrete の対応を抽出する。
var _ TagRepository = (*repository.TagRepository)(nil)

// GetTag はインターフェース経由でタグを取得する。
// tagRepo.FindTagByID は SSA 上では Invoke 命令となり、
// アナライザは ifaceImpls から *repository.TagRepository を引き、
// その FindTagByID の Fact を参照して Sentinel を解決する。
//
// 検出される Sentinel:
//   - repository.ErrNotFound : id が 0 以下
//   - database/sql.ErrNoRows : DB に存在しない
func GetTag(tagRepo TagRepository, id int) (repository.Tag, error) {
	t, err := tagRepo.FindTagByID(id)
	if err != nil {
		return repository.Tag{}, fmt.Errorf("GetTag: %w", err)
	}
	return t, nil
}

// CreateTag はインターフェース経由でタグを登録する。
func CreateTag(tagRepo TagRepository, name string) (int64, error) {
	id, err := tagRepo.CreateTag(name)
	if err != nil {
		return 0, fmt.Errorf("CreateTag: %w", err)
	}
	return id, nil
}

// DeleteTag はインターフェース経由でタグを削除する。
//
// 検出される Sentinel:
//   - repository.ErrNotFound : タグが存在しない
func DeleteTag(tagRepo TagRepository, id int) error {
	if err := tagRepo.DeleteTag(id); err != nil {
		return fmt.Errorf("DeleteTag: %w", err)
	}
	return nil
}
