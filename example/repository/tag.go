package repository

import (
	"database/sql"
	"fmt"
)

type TagRepository struct {
	db *sql.DB
}

func (t *TagRepository) CreateTag(name string) (int64, error) {
	res, err := t.db.Exec("INSERT INTO tags (name) VALUES (?)", name)
	if err != nil {
		return 0, fmt.Errorf("CreateTag: insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (t *TagRepository) DeleteTag(id int) error {
	if _, err := t.FindTagByID(id); err != nil {
		return fmt.Errorf("DeleteTag: %w", err)
	}
	if _, err := t.db.Exec("DELETE FROM tags WHERE id = ?", id); err != nil {
		return fmt.Errorf("DeleteTag: exec: %w", err)
	}
	return nil
}

// FindTagByID implements [usecase.TagRepository].
func (t *TagRepository) FindTagByID(id int) (Tag, error) {
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

type Tag struct {
	ID   int
	Name string
}

func NewTagRepository(db *sql.DB) *TagRepository {
	return &TagRepository{
		db: db,
	}
}
