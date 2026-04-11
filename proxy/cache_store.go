package proxy

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

const cacheFormatVersion = 2

// CacheMetadata は永続キャッシュの互換性・無効化判定に使うメタ情報。
type CacheMetadata struct {
	FormatVersion int
	Workspace     string
	Pattern       string
	SourceHash    string
}

type cacheFile struct {
	Metadata CacheMetadata
	Entries  []cacheFileEntry
}

type cacheFileEntry struct {
	File       string
	Line       int
	FuncName   string
	Sentinels  []string
	ByConcrete map[string][]string
}

// SaveCacheToFile は Cache を gob 形式で保存する。
func SaveCacheToFile(c Cache, path string) error {
	return SaveCacheToFileWithMetadata(c, path, CacheMetadata{FormatVersion: cacheFormatVersion})
}

// SaveCacheToFileWithMetadata は Cache をメタ情報付きで gob 形式保存する。
// tmp ファイル → fsync → rename の順で書き込むため、複数プロセスが同時に保存しても
// torn write や途中書きのファイルが残ることはない。
func SaveCacheToFileWithMetadata(c Cache, path string, metadata CacheMetadata) error {
	if path == "" {
		return fmt.Errorf("SaveCacheToFileWithMetadata: empty path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("SaveCacheToFileWithMetadata: mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "cache-*.gob.tmp")
	if err != nil {
		return fmt.Errorf("SaveCacheToFileWithMetadata: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	metadata = normalizeMetadata(metadata)
	payload := cacheToFile(c)
	payload.Metadata = metadata
	if err := gob.NewEncoder(tmp).Encode(payload); err != nil {
		cleanup()
		return fmt.Errorf("SaveCacheToFileWithMetadata: encode: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("SaveCacheToFileWithMetadata: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("SaveCacheToFileWithMetadata: close: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("SaveCacheToFileWithMetadata: rename: %w", err)
	}
	return nil
}

// LoadCacheFromFile は gob 形式のキャッシュを読み込む。
func LoadCacheFromFile(path string) (Cache, error) {
	cache, _, err := LoadCacheFromFileWithMetadata(path)
	return cache, err
}

// LoadCacheFromFileWithMetadata は gob 形式キャッシュをメタ情報付きで読み込む。
func LoadCacheFromFileWithMetadata(path string) (Cache, CacheMetadata, error) {
	if path == "" {
		return NewCache(), CacheMetadata{}, fmt.Errorf("LoadCacheFromFileWithMetadata: empty path")
	}
	f, err := os.Open(path)
	if err != nil {
		return NewCache(), CacheMetadata{}, fmt.Errorf("LoadCacheFromFileWithMetadata: open: %w", err)
	}
	defer f.Close()

	var payload cacheFile
	if err := gob.NewDecoder(f).Decode(&payload); err != nil {
		return NewCache(), CacheMetadata{}, fmt.Errorf("LoadCacheFromFileWithMetadata: decode: %w", err)
	}
	return fileToCache(payload), normalizeMetadata(payload.Metadata), nil
}

func cacheToFile(c Cache) cacheFile {
	out := cacheFile{Entries: make([]cacheFileEntry, 0, len(c.byLocation))}
	for key, entry := range c.byLocation {
		if entry == nil {
			continue
		}
		out.Entries = append(out.Entries, cacheFileEntry{
			File:       key.file,
			Line:       key.line,
			FuncName:   entry.FuncName,
			Sentinels:  append([]string(nil), entry.Sentinels...),
			ByConcrete: cloneByConcrete(entry.ByConcrete),
		})
	}
	return out
}

func fileToCache(cf cacheFile) Cache {
	c := NewCache()
	for _, e := range cf.Entries {
		entry := &CacheEntry{
			FuncName:   e.FuncName,
			Sentinels:  append([]string(nil), e.Sentinels...),
			ByConcrete: cloneByConcrete(e.ByConcrete),
		}
		c.addEntry(cacheKey{file: e.File, line: e.Line}, entry)
	}
	return c
}

func cloneByConcrete(src map[string][]string) map[string][]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string][]string, len(src))
	for k, v := range src {
		dst[k] = append([]string(nil), v...)
	}
	return dst
}

func normalizeMetadata(m CacheMetadata) CacheMetadata {
	if m.FormatVersion == 0 {
		m.FormatVersion = cacheFormatVersion
	}
	if m.Workspace != "" {
		m.Workspace = filepath.Clean(m.Workspace)
	}
	return m
}

func metadataMatches(actual, expected CacheMetadata) bool {
	a := normalizeMetadata(actual)
	e := normalizeMetadata(expected)
	return a.FormatVersion == e.FormatVersion &&
		a.Workspace == e.Workspace &&
		a.Pattern == e.Pattern &&
		a.SourceHash == e.SourceHash
}
