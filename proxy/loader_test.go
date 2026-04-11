package proxy

import "testing"

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
