package proxy

import (
	"path/filepath"
	"testing"
)

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
