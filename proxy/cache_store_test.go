package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestCacheStore_MetadataRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.gob")
	meta := CacheMetadata{
		FormatVersion: cacheFormatVersion,
		Workspace:     "/workspace",
		Pattern:       "./pkg/...",
	}
	c := NewCache()

	if err := SaveCacheToFileWithMetadata(c, path, meta); err != nil {
		t.Fatalf("SaveCacheToFileWithMetadata: %v", err)
	}
	_, gotMeta, err := LoadCacheFromFileWithMetadata(path)
	if err != nil {
		t.Fatalf("LoadCacheFromFileWithMetadata: %v", err)
	}
	if gotMeta.Workspace != meta.Workspace || gotMeta.Pattern != meta.Pattern || gotMeta.FormatVersion != meta.FormatVersion {
		t.Fatalf("metadata mismatch: got=%+v want=%+v", gotMeta, meta)
	}
}

func TestMetadataMatches(t *testing.T) {
	t.Parallel()
	base := CacheMetadata{
		FormatVersion: cacheFormatVersion,
		Workspace:     "/workspace",
		Pattern:       "./...",
	}
	if !metadataMatches(base, base) {
		t.Fatal("expected equal metadata to match")
	}
	if metadataMatches(base, CacheMetadata{FormatVersion: cacheFormatVersion, Workspace: "/other", Pattern: "./..."}) {
		t.Fatal("workspace mismatch should not match")
	}
	if metadataMatches(base, CacheMetadata{FormatVersion: cacheFormatVersion, Workspace: "/workspace", Pattern: "./pkg/..."}) {
		t.Fatal("pattern mismatch should not match")
	}
}

func TestMetadataMatches_SourceHashMismatch(t *testing.T) {
	t.Parallel()
	base := CacheMetadata{
		FormatVersion: cacheFormatVersion,
		Workspace:     "/workspace",
		Pattern:       "./...",
		SourceHash:    "abc",
	}
	other := base
	other.SourceHash = "def"
	if metadataMatches(base, other) {
		t.Fatal("source hash mismatch should not match")
	}
}

func TestCacheStore_AtomicWrite_NoLeftoverTemp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.gob")
	if err := SaveCacheToFile(NewCache(), path); err != nil {
		t.Fatalf("SaveCacheToFile: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover tmp file after successful save: %s", e.Name())
		}
	}
}

// 複数プロセス想定の並行書き込み。どのゴルーチンが勝っても
// 最終ファイルは完全な gob として読み出せることを確認する。
func TestCacheStore_ConcurrentSaves_NoTornWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.gob")

	build := func(fn string) Cache {
		c := NewCache()
		c.addEntry(cacheKey{file: fn, line: 1}, &CacheEntry{
			FuncName:  "F",
			Sentinels: []string{"pkg.Err"},
		})
		return c
	}

	const writers = 8
	var wg sync.WaitGroup
	wg.Add(writers)
	for range writers {
		go func() {
			defer wg.Done()
			c := build("/src/a.go")
			meta := CacheMetadata{
				FormatVersion: cacheFormatVersion,
				Workspace:     "/workspace",
				Pattern:       "./...",
				SourceHash:    "hash",
			}
			_ = SaveCacheToFileWithMetadata(c, path, meta)
			_, _, _ = LoadCacheFromFileWithMetadata(path)
		}()
	}
	wg.Wait()

	loaded, meta, err := LoadCacheFromFileWithMetadata(path)
	if err != nil {
		t.Fatalf("LoadCacheFromFileWithMetadata after concurrent saves: %v", err)
	}
	if _, ok := loaded.Lookup("/src/a.go", 1); !ok {
		t.Fatal("expected entry after concurrent saves")
	}
	if meta.SourceHash != "hash" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}

	// tmp ファイルが残っていないこと
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover tmp file after concurrent saves: %s", e.Name())
		}
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
