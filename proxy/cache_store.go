package proxy

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

type cacheFile struct {
	Entries []cacheFileEntry
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
	if path == "" {
		return fmt.Errorf("SaveCacheToFile: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("SaveCacheToFile: mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("SaveCacheToFile: create: %w", err)
	}
	defer f.Close()

	payload := cacheToFile(c)
	if err := gob.NewEncoder(f).Encode(payload); err != nil {
		return fmt.Errorf("SaveCacheToFile: encode: %w", err)
	}
	return nil
}

// LoadCacheFromFile は gob 形式のキャッシュを読み込む。
func LoadCacheFromFile(path string) (Cache, error) {
	if path == "" {
		return NewCache(), fmt.Errorf("LoadCacheFromFile: empty path")
	}
	f, err := os.Open(path)
	if err != nil {
		return NewCache(), fmt.Errorf("LoadCacheFromFile: open: %w", err)
	}
	defer f.Close()

	var payload cacheFile
	if err := gob.NewDecoder(f).Decode(&payload); err != nil {
		return NewCache(), fmt.Errorf("LoadCacheFromFile: decode: %w", err)
	}
	return fileToCache(payload), nil
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
