package proxy

import (
	"testing"
)

const sampleJSON = `{
	"example.com/myapp/repository": {
		"sentinelfind": [
			{
				"posn": "/workspace/repository/user.go:8:6",
				"end":  "/workspace/repository/user.go:8:6",
				"message": "FindByID returns sentinels: repository.ErrNotFound"
			},
			{
				"posn": "/workspace/repository/user.go:15:6",
				"end":  "/workspace/repository/user.go:15:6",
				"message": "Create returns sentinels: repository.ErrDuplicate"
			}
		]
	},
	"example.com/myapp/usecase": {
		"sentinelfind": [
			{
				"posn": "/workspace/usecase/user.go:9:6",
				"end":  "/workspace/usecase/user.go:9:6",
				"message": "GetUser returns sentinels: repository.ErrNotFound"
			}
		]
	}
}`

func TestCache_ParseJSON(t *testing.T) {
	c, err := parseSentinelfindJSON([]byte(sampleJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.byLocation) == 0 {
		t.Fatal("cache is empty")
	}
}

func TestCache_LookupByFile(t *testing.T) {
	c, err := parseSentinelfindJSON([]byte(sampleJSON))
	if err != nil {
		t.Fatal(err)
	}

	// ファイルパスと行番号で検索
	entry, ok := c.lookup("/workspace/repository/user.go", 8)
	if !ok {
		t.Fatal("entry not found for repository/user.go:8")
	}
	if entry.FuncName != "FindByID" {
		t.Errorf("FuncName = %q, want %q", entry.FuncName, "FindByID")
	}
	if len(entry.Sentinels) != 1 || entry.Sentinels[0] != "repository.ErrNotFound" {
		t.Errorf("Sentinels = %v, want [repository.ErrNotFound]", entry.Sentinels)
	}
}

func TestCache_LookupMiss(t *testing.T) {
	c, err := parseSentinelfindJSON([]byte(sampleJSON))
	if err != nil {
		t.Fatal(err)
	}

	_, ok := c.lookup("/workspace/repository/user.go", 99)
	if ok {
		t.Error("expected miss for non-existent line")
	}
}

func TestCache_LookupByFuncName(t *testing.T) {
	c, err := parseSentinelfindJSON([]byte(sampleJSON))
	if err != nil {
		t.Fatal(err)
	}

	// 関数名でも引けること
	entry, ok := c.lookupByFuncName("FindByID")
	if !ok {
		t.Fatal("entry not found by func name FindByID")
	}
	if len(entry.Sentinels) == 0 || entry.Sentinels[0] != "repository.ErrNotFound" {
		t.Errorf("unexpected sentinels: %v", entry.Sentinels)
	}

	// 存在しない関数名はミス
	_, ok = c.lookupByFuncName("NoSuchFunc")
	if ok {
		t.Error("expected miss for unknown func name")
	}
}

func TestCache_MultipleSentinels(t *testing.T) {
	const multiJSON = `{
		"pkg": {
			"sentinelfind": [
				{
					"posn": "/src/foo.go:5:6",
					"end":  "/src/foo.go:5:6",
					"message": "Fetch returns sentinels: pkg.ErrA, pkg.ErrB"
				}
			]
		}
	}`
	c, err := parseSentinelfindJSON([]byte(multiJSON))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.lookup("/src/foo.go", 5)
	if !ok {
		t.Fatal("entry not found")
	}
	if len(entry.Sentinels) != 2 {
		t.Errorf("want 2 sentinels, got %d: %v", len(entry.Sentinels), entry.Sentinels)
	}
}

func TestCache_MarkdownFormat(t *testing.T) {
	entry := &CacheEntry{
		FuncName:  "FindByID",
		Sentinels: []string{"repository.ErrNotFound", "io.EOF"},
	}
	md := entry.markdown()
	if md == "" {
		t.Error("markdown is empty")
	}
	// 各 sentinel 名が含まれること
	for _, s := range entry.Sentinels {
		if !contains(md, s) {
			t.Errorf("markdown missing sentinel %q:\n%s", s, md)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
