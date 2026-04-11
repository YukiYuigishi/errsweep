package proxy

import (
	"path/filepath"
	"testing"
)

func TestCacheStore_SaveAndLoadRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.gob")

	c, err := ParseSentinelfindJSON([]byte(`{
		"pkg": {
			"sentinelfind": [
				{
					"posn": "/src/tag.go:44:6",
					"message": "CreateTag returns sentinels via *repository.A: pkg.ErrA"
				},
				{
					"posn": "/src/tag.go:44:6",
					"message": "CreateTag returns sentinels via *repository.B: pkg.ErrB"
				}
			]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	if err := SaveCacheToFile(c, path); err != nil {
		t.Fatalf("SaveCacheToFile: %v", err)
	}
	loaded, err := LoadCacheFromFile(path)
	if err != nil {
		t.Fatalf("LoadCacheFromFile: %v", err)
	}

	entry, ok := loaded.Lookup("/src/tag.go", 44)
	if !ok {
		t.Fatal("loaded cache missing /src/tag.go:44")
	}
	if len(entry.Sentinels) != 2 {
		t.Fatalf("want 2 sentinels, got %v", entry.Sentinels)
	}
	if len(entry.ByConcrete) != 2 {
		t.Fatalf("want 2 byConcrete entries, got %v", entry.ByConcrete)
	}
}

func TestCacheStore_EmptyPath(t *testing.T) {
	t.Parallel()
	if err := SaveCacheToFile(NewCache(), ""); err == nil {
		t.Fatal("SaveCacheToFile should fail for empty path")
	}
	if _, err := LoadCacheFromFile(""); err == nil {
		t.Fatal("LoadCacheFromFile should fail for empty path")
	}
}
