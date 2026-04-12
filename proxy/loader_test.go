package proxy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkComputeSourceHash はソースハッシュ計算のコストを計測する。
// ERRSWEEP_BENCH_WORKSPACE に計測対象のワークスペースを指定する。
// 未指定なら skip。例:
//
//	ERRSWEEP_BENCH_WORKSPACE=$PWD/tmp/moby go test -bench=BenchmarkComputeSourceHash \
//	    -benchtime=5x -run=^$ ./proxy/
func BenchmarkComputeSourceHash(b *testing.B) {
	workspace := os.Getenv("ERRSWEEP_BENCH_WORKSPACE")
	if workspace == "" {
		b.Skip("set ERRSWEEP_BENCH_WORKSPACE to benchmark")
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		b.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(absWorkspace); err != nil {
		b.Fatalf("workspace stat: %v", err)
	}

	// 初回は warm-up（FS キャッシュを温める）
	if _, err := computeSourceHash(absWorkspace); err != nil {
		b.Fatalf("warm-up: %v", err)
	}

	for b.Loop() {
		if _, err := computeSourceHash(absWorkspace); err != nil {
			b.Fatalf("computeSourceHash: %v", err)
		}
	}
}

func TestComputeSourceHash_DetectsChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module p\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	h1, err := computeSourceHash(dir)
	if err != nil {
		t.Fatalf("computeSourceHash: %v", err)
	}
	if h1 == "" {
		t.Fatal("expected non-empty hash")
	}

	later := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n// edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, "a.go"), later, later); err != nil {
		t.Fatal(err)
	}

	h2, err := computeSourceHash(dir)
	if err != nil {
		t.Fatalf("computeSourceHash: %v", err)
	}
	if h1 == h2 {
		t.Fatal("expected hash to change after source edit")
	}
}

func TestComputeSourceHash_SkipsNestedModules(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "tmp", "vendored")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module vendored\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "v.go"), []byte("package v\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	h1, err := computeSourceHash(dir)
	if err != nil {
		t.Fatal(err)
	}

	// nested module 内を編集してもハッシュは変わらないはず
	later := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(filepath.Join(nested, "v.go"), []byte("package v\n// edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(nested, "v.go"), later, later); err != nil {
		t.Fatal(err)
	}
	h2, err := computeSourceHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatal("expected nested module edits to be ignored by source hash")
	}
}

func TestComputeSourceHash_SkipsCacheDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h1, err := computeSourceHash(dir)
	if err != nil {
		t.Fatal(err)
	}

	// .errsweep/cache.gob 内のファイルは hash 対象外であるべき
	if err := os.MkdirAll(filepath.Join(dir, ".errsweep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".errsweep", "noise.go"), []byte("package n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h2, err := computeSourceHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatal("expected .errsweep to be skipped from source hash")
	}
}

func TestSetBuildCachePattern(t *testing.T) {
	prev := buildCachePattern
	t.Cleanup(func() { buildCachePattern = prev })

	SetBuildCachePattern("./daemon/...")
	args := buildCacheArgs()
	if len(args) != 2 || args[0] != "-json" || args[1] != "./daemon/..." {
		t.Fatalf("buildCacheArgs() = %v, want [-json ./daemon/...]", args)
	}

	SetBuildCachePattern("")
	args = buildCacheArgs()
	if len(args) != 2 || args[1] != "./daemon/..." {
		t.Fatalf("empty pattern should keep previous value, got %v", args)
	}
}

func TestResolveCacheFilePath(t *testing.T) {
	prev := buildCacheFilePath
	t.Cleanup(func() { buildCacheFilePath = prev })

	SetBuildCacheFilePath("")
	got := resolveCacheFilePath("/workspace")
	want := filepath.Join("/workspace", ".errsweep", "cache.gob")
	if got != want {
		t.Fatalf("resolveCacheFilePath default = %q, want %q", got, want)
	}

	SetBuildCacheFilePath("/tmp/custom.gob")
	if got := resolveCacheFilePath("/workspace"); got != "/tmp/custom.gob" {
		t.Fatalf("resolveCacheFilePath custom = %q, want %q", got, "/tmp/custom.gob")
	}
}

func TestLoadValidCache_MetadataMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.gob")
	cache := NewCache()
	if err := SaveCacheToFileWithMetadata(cache, path, CacheMetadata{
		FormatVersion: cacheFormatVersion,
		Workspace:     "/workspace-a",
		Pattern:       "./...",
	}); err != nil {
		t.Fatalf("SaveCacheToFileWithMetadata: %v", err)
	}
	if _, err := loadValidCache(path, CacheMetadata{
		FormatVersion: cacheFormatVersion,
		Workspace:     "/workspace-b",
		Pattern:       "./...",
	}); err == nil {
		t.Fatal("expected metadata mismatch error")
	}
}

func TestBuildCache_FallbackUsesMatchingMetadata(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.gob")
	prevPattern := buildCachePattern
	prevPath := buildCacheFilePath
	prevTimeout := buildCacheTimeout
	t.Cleanup(func() {
		buildCachePattern = prevPattern
		buildCacheFilePath = prevPath
		buildCacheTimeout = prevTimeout
	})

	SetBuildCachePattern("./pkg/...")
	SetBuildCacheFilePath(cachePath)
	SetBuildCacheTimeout(200 * time.Millisecond)

	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G306 -- テスト用の擬似 errsweep バイナリ。exec するため 0o755 が必須。
	if err := os.WriteFile(filepath.Join(dir, "sleep.sh"), []byte("#!/bin/sh\nsleep 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourceHash, err := computeSourceHash(absDir)
	if err != nil {
		t.Fatalf("computeSourceHash: %v", err)
	}
	cached := NewCache()
	cached.addEntry(cacheKey{file: "/src/a.go", line: 10}, &CacheEntry{
		FuncName:  "A",
		Sentinels: []string{"pkg.ErrA"},
	})
	if err := SaveCacheToFileWithMetadata(cached, cachePath, CacheMetadata{
		FormatVersion: cacheFormatVersion,
		Workspace:     absDir,
		Pattern:       "./pkg/...",
		SourceHash:    sourceHash,
	}); err != nil {
		t.Fatalf("SaveCacheToFileWithMetadata: %v", err)
	}

	got, err := BuildCache(filepath.Join(dir, "sleep.sh"), dir)
	if err != nil {
		t.Fatalf("BuildCache: %v", err)
	}
	if _, ok := got.Lookup("/src/a.go", 10); !ok {
		t.Fatal("expected fallback cache entry")
	}
}
