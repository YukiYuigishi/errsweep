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

func TestCache_LookupByFuncName_MergeSameNameAcrossPackages(t *testing.T) {
	const multiPkgJSON = `{
		"pkg/a": {
			"sentinelfind": [
				{
					"posn": "/workspace/a/new_hoge.go:10:2",
					"message": "NewHoge returns sentinels: a.ErrA"
				}
			]
		},
		"pkg/b": {
			"sentinelfind": [
				{
					"posn": "/workspace/b/new_hoge.go:20:2",
					"message": "NewHoge returns sentinels: b.ErrB"
				}
			]
		}
	}`
	c, err := parseSentinelfindJSON([]byte(multiPkgJSON))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.lookupByFuncName("NewHoge")
	if !ok {
		t.Fatal("entry not found by func name NewHoge")
	}
	if len(entry.Sentinels) != 2 {
		t.Fatalf("want merged 2 sentinels, got %d: %v", len(entry.Sentinels), entry.Sentinels)
	}
	if entry.Sentinels[0] != "a.ErrA" || entry.Sentinels[1] != "b.ErrB" {
		t.Fatalf("unexpected merged sentinels: %v", entry.Sentinels)
	}
}

func TestCache_LookupByFuncName_MethodFallbackBySimpleName(t *testing.T) {
	const methodJSON = `{
		"pkg/usecase": {
			"sentinelfind": [
				{
					"posn": "/workspace/usecase/service.go:30:2",
					"message": "(*Service).Create returns sentinels: usecase.ErrInvalid"
				}
			]
		}
	}`
	c, err := parseSentinelfindJSON([]byte(methodJSON))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.lookupByFuncName("Create")
	if !ok {
		t.Fatal("entry not found by simple method name Create")
	}
	if len(entry.Sentinels) != 1 || entry.Sentinels[0] != "usecase.ErrInvalid" {
		t.Fatalf("unexpected sentinels: %v", entry.Sentinels)
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

func TestCache_ParseJSON_WrappedDiagnosticsObject(t *testing.T) {
	const wrappedJSON = `{
		"pkg": {
			"sentinelfind": {
				"diagnostics": [
					{
						"posn": "/src/foo.go:5:6",
						"message": "Fetch returns sentinels: pkg.ErrA"
					}
				]
			}
		}
	}`
	c, err := parseSentinelfindJSON([]byte(wrappedJSON))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.lookup("/src/foo.go", 5)
	if !ok {
		t.Fatal("entry not found")
	}
	if len(entry.Sentinels) != 1 || entry.Sentinels[0] != "pkg.ErrA" {
		t.Errorf("unexpected sentinels: %v", entry.Sentinels)
	}
}

func TestCache_ParseJSON_SingleDiagnosticObject(t *testing.T) {
	const singleJSON = `{
		"pkg": {
			"sentinelfind": {
				"posn": "/src/bar.go:7:2",
				"message": "Run returns sentinels: pkg.ErrB"
			}
		}
	}`
	c, err := parseSentinelfindJSON([]byte(singleJSON))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.lookup("/src/bar.go", 7)
	if !ok {
		t.Fatal("entry not found")
	}
	if len(entry.Sentinels) != 1 || entry.Sentinels[0] != "pkg.ErrB" {
		t.Errorf("unexpected sentinels: %v", entry.Sentinels)
	}
}

func TestCache_MultiConcreteUnion(t *testing.T) {
	// 多 concrete DI のケース: 合算ライン + per-concrete 内訳が同一 file:line に並ぶ。
	// 上書きではなく union で merge され、ByConcrete に内訳が蓄積されること。
	const multiConcreteJSON = `{
		"pkg": {
			"sentinelfind": [
				{
					"posn": "/src/tag.go:44:6",
					"end":  "/src/tag.go:44:6",
					"message": "CreateTag returns sentinels: repository.ErrInvalidValue, repository.ErrInvalidValueDummy"
				},
				{
					"posn": "/src/tag.go:44:6",
					"end":  "/src/tag.go:44:6",
					"message": "CreateTag returns sentinels via *repository.TagRepository: repository.ErrInvalidValue"
				},
				{
					"posn": "/src/tag.go:44:6",
					"end":  "/src/tag.go:44:6",
					"message": "CreateTag returns sentinels via *repository.TagRepositoryDummy: repository.ErrInvalidValueDummy"
				}
			]
		}
	}`
	c, err := parseSentinelfindJSON([]byte(multiConcreteJSON))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := c.lookup("/src/tag.go", 44)
	if !ok {
		t.Fatal("entry not found for /src/tag.go:44")
	}
	if entry.FuncName != "CreateTag" {
		t.Errorf("FuncName = %q, want %q", entry.FuncName, "CreateTag")
	}
	if len(entry.Sentinels) != 2 {
		t.Errorf("want 2 union sentinels, got %d: %v", len(entry.Sentinels), entry.Sentinels)
	}
	if len(entry.ByConcrete) != 2 {
		t.Errorf("want 2 concrete breakdowns, got %d: %v", len(entry.ByConcrete), entry.ByConcrete)
	}
	if got := entry.ByConcrete["*repository.TagRepository"]; len(got) != 1 || got[0] != "repository.ErrInvalidValue" {
		t.Errorf("TagRepository breakdown = %v, want [repository.ErrInvalidValue]", got)
	}
	if got := entry.ByConcrete["*repository.TagRepositoryDummy"]; len(got) != 1 || got[0] != "repository.ErrInvalidValueDummy" {
		t.Errorf("TagRepositoryDummy breakdown = %v, want [repository.ErrInvalidValueDummy]", got)
	}

	// per-concrete だけで合算ラインが来ない順序でも union されること。
	const perConcreteOnlyJSON = `{
		"pkg": {
			"sentinelfind": [
				{
					"posn": "/src/get.go:35:6",
					"end":  "/src/get.go:35:6",
					"message": "GetTag returns sentinels via *repository.A: pkg.ErrA"
				},
				{
					"posn": "/src/get.go:35:6",
					"end":  "/src/get.go:35:6",
					"message": "GetTag returns sentinels via *repository.B: pkg.ErrB"
				}
			]
		}
	}`
	c, err = parseSentinelfindJSON([]byte(perConcreteOnlyJSON))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok = c.lookup("/src/get.go", 35)
	if !ok {
		t.Fatal("entry not found for /src/get.go:35")
	}
	if len(entry.Sentinels) != 2 {
		t.Errorf("want 2 union sentinels, got %d: %v", len(entry.Sentinels), entry.Sentinels)
	}
}

func TestCache_MarkdownByConcrete(t *testing.T) {
	entry := &CacheEntry{
		FuncName:  "CreateTag",
		Sentinels: []string{"repository.ErrInvalidValue", "repository.ErrInvalidValueDummy"},
		ByConcrete: map[string][]string{
			"*repository.TagRepository":      {"repository.ErrInvalidValue"},
			"*repository.TagRepositoryDummy": {"repository.ErrInvalidValueDummy"},
		},
	}
	md := entry.markdown()
	for _, want := range []string{
		"*repository.TagRepository",
		"*repository.TagRepositoryDummy",
		"repository.ErrInvalidValue",
		"repository.ErrInvalidValueDummy",
		"via `",
	} {
		if !contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
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
