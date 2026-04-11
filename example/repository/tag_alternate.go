package repository

import (
	"database/sql"
	"errors"
	"fmt"
)

// TagRepositoryLegacy は代替実装（複数 concrete の解析デモ用）。
type TagRepositoryLegacy struct {
	db *sql.DB
}

var ErrInvalidValueLegacy = errors.New("legacy repository invalid value")

func (t *TagRepositoryLegacy) CreateTag(name string) (int64, error) {
	if name == "" {
		return 0, ErrInvalidValueLegacy
	}
	res, err := t.db.Exec("INSERT INTO tags (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("CreateTag: insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (t *TagRepositoryLegacy) DeleteTag(id int) error {
	if _, err := t.FindTagByID(id); err != nil {
		return fmt.Errorf("DeleteTag: %w", err)
	}
	if _, err := t.db.Exec("DELETE FROM tags WHERE id = ?", id); err != nil {
		return fmt.Errorf("DeleteTag: exec: %w", err)
	}
	return nil
}

func (t *TagRepositoryLegacy) FindTagByID(id int) (Tag, error) {
	if id <= 0 {
		return Tag{}, ErrNotFound
	}
	var tag Tag
	err := t.db.QueryRow(
		"SELECT id, name, email FROM tags WHERE id = ?", id,
	).Scan(&tag.ID, &tag.Name)
	if err != nil {
		return Tag{}, fmt.Errorf("FindTagByID: %w", err)
	}
	return tag, nil
}

func NewTagRepositoryLegacy(db *sql.DB) *TagRepositoryLegacy {
	return &TagRepositoryLegacy{
		db: db,
	}
}
