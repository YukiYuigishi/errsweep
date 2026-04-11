package proxy

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// CacheEntry は1関数分の解析結果。
type CacheEntry struct {
	FuncName  string
	Sentinels []string // union: e.g. ["io.EOF", "sql.ErrNoRows"]
	// ByConcrete は多 concrete DI の内訳。複数 concrete が検出された場合のみ非 nil。
	// キーは concrete 型名（例: "*repository.TagRepository"）、値はその concrete が返す sentinel 群。
	ByConcrete map[string][]string
}

// Markdown は markdown の公開 API。
func (e *CacheEntry) Markdown() string { return e.markdown() }

// markdown は hover に追記する Markdown テキストを返す。
// 多 concrete の場合は concrete ごとにグルーピングし、そうでなければ平坦なリストで表示する。
func (e *CacheEntry) markdown() string {
	var sb strings.Builder
	sb.WriteString("---\n**Possible Sentinel Errors:**\n")
	if len(e.ByConcrete) > 1 {
		concretes := make([]string, 0, len(e.ByConcrete))
		for c := range e.ByConcrete {
			concretes = append(concretes, c)
		}
		sort.Strings(concretes)
		for _, c := range concretes {
			sb.WriteString("- via `")
			sb.WriteString(c)
			sb.WriteString("`:\n")
			for _, s := range e.ByConcrete[c] {
				sb.WriteString("  - `")
				sb.WriteString(s)
				sb.WriteString("`\n")
			}
		}
		return sb.String()
	}
	for _, s := range e.Sentinels {
		sb.WriteString("- `")
		sb.WriteString(s)
		sb.WriteString("`\n")
	}
	return sb.String()
}

// cacheKey はファイルパスと行番号のペア。
type cacheKey struct {
	file string
	line int
}

// Cache はファイル位置 → CacheEntry および関数名 → CacheEntry の二重インデックス。
// byFuncName は呼び出し側でのホバー時（定義行でないホバー）に使う。
// 同名関数が複数パッケージに存在する場合は最後に登録されたエントリが勝つ。
type Cache struct {
	byLocation map[cacheKey]*CacheEntry
	byFuncName map[string][]*CacheEntry // FuncName/別名 → entries
}

// NewCache は空の Cache を生成する。
func NewCache() Cache {
	return Cache{
		byLocation: make(map[cacheKey]*CacheEntry),
		byFuncName: make(map[string][]*CacheEntry),
	}
}

// Len はロケーションインデックスのエントリ数を返す。
func (c Cache) Len() int { return len(c.byLocation) }

// addEntry はエントリを両方のインデックスに登録する。
func (c *Cache) addEntry(key cacheKey, entry *CacheEntry) {
	c.byLocation[key] = entry
	c.byFuncName[entry.FuncName] = appendUniqueEntry(c.byFuncName[entry.FuncName], entry)
	if simple := simpleFuncName(entry.FuncName); simple != "" && simple != entry.FuncName {
		c.byFuncName[simple] = appendUniqueEntry(c.byFuncName[simple], entry)
	}
}

func appendUniqueEntry(entries []*CacheEntry, entry *CacheEntry) []*CacheEntry {
	for _, e := range entries {
		if e == entry {
			return entries
		}
	}
	return append(entries, entry)
}

// unionSentinels は a と b を重複排除してソートしたスライスを返す。
func unionSentinels(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// lookup は指定ファイル・行番号の CacheEntry を返す。
func (c Cache) lookup(file string, line int) (*CacheEntry, bool) {
	entry, ok := c.byLocation[cacheKey{file: file, line: line}]
	return entry, ok
}

// Lookup は lookup の公開 API。
func (c Cache) Lookup(file string, line int) (*CacheEntry, bool) {
	return c.lookup(file, line)
}

// lookupByFuncName は関数名で CacheEntry を返す。
// 呼び出し側でのホバー時のフォールバックとして使う。
func (c Cache) lookupByFuncName(name string) (*CacheEntry, bool) {
	entries, ok := c.byFuncName[name]
	if !ok || len(entries) == 0 {
		return nil, false
	}
	if len(entries) == 1 {
		return entries[0], true
	}
	merged := &CacheEntry{
		FuncName:   name,
		ByConcrete: make(map[string][]string),
	}
	for _, e := range entries {
		merged.Sentinels = unionSentinels(merged.Sentinels, e.Sentinels)
		for concrete, sentinels := range e.ByConcrete {
			merged.ByConcrete[concrete] = unionSentinels(merged.ByConcrete[concrete], sentinels)
		}
	}
	if len(merged.ByConcrete) == 0 {
		merged.ByConcrete = nil
	}
	return merged, true
}

// simpleFuncName は "(*T).Method" や "pkg.Func" から "Method"/"Func" を返す。
func simpleFuncName(name string) string {
	if i := strings.LastIndex(name, ")."); i >= 0 && i+2 < len(name) {
		return name[i+2:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 && i+1 < len(name) {
		return name[i+1:]
	}
	return name
}

type diagnostic struct {
	Posn    string `json:"posn"`
	Message string `json:"message"`
}

// ParseSentinelfindJSON は `sentinelfind -json` の出力を Cache に変換する（公開 API）。
func ParseSentinelfindJSON(data []byte) (Cache, error) { return parseSentinelfindJSON(data) }

// parseSentinelfindJSON は `sentinelfind -json` の出力を Cache に変換する。
// 同一 file:line に複数診断がある場合（例: 合算ライン + per-concrete 内訳ライン）は
// エントリを上書きせず、sentinels を union し ByConcrete に内訳を蓄積する。
func parseSentinelfindJSON(data []byte) (Cache, error) {
	var out map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		return NewCache(), fmt.Errorf("parseSentinelfindJSON: %w", err)
	}

	cache := NewCache()
	for _, checks := range out {
		for _, raw := range checks {
			diags, err := decodeDiagnostics(raw)
			if err != nil {
				continue
			}
			for _, d := range diags {
				file, line, err := parsePosn(d.Posn)
				if err != nil {
					continue
				}
				funcName, concrete, sentinels, err := parseDiagMessage(d.Message)
				if err != nil {
					continue
				}
				key := cacheKey{file: file, line: line}
				entry := cache.byLocation[key]
				if entry == nil {
					entry = &CacheEntry{FuncName: funcName}
					cache.addEntry(key, entry)
				}
				entry.Sentinels = unionSentinels(entry.Sentinels, sentinels)
				if concrete != "" {
					if entry.ByConcrete == nil {
						entry.ByConcrete = make(map[string][]string)
					}
					entry.ByConcrete[concrete] = sentinels
				}
			}
		}
	}
	return cache, nil
}

// decodeDiagnostics は analyzer 出力を diagnostics 配列へ正規化する。
// 互換のため、以下の形を受け付ける:
// - []diagnostic
// - diagnostic (単体)
// - {"diagnostics": []diagnostic} / {"diagnostics": diagnostic}
func decodeDiagnostics(raw json.RawMessage) ([]diagnostic, error) {
	var arr []diagnostic
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}

	var one diagnostic
	if err := json.Unmarshal(raw, &one); err == nil && one.Posn != "" {
		return []diagnostic{one}, nil
	}

	var wrapped struct {
		Diagnostics json.RawMessage `json:"diagnostics"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Diagnostics) > 0 {
		if err := json.Unmarshal(wrapped.Diagnostics, &arr); err == nil {
			return arr, nil
		}
		if err := json.Unmarshal(wrapped.Diagnostics, &one); err == nil && one.Posn != "" {
			return []diagnostic{one}, nil
		}
	}

	return nil, fmt.Errorf("decodeDiagnostics: unsupported shape")
}

// parsePosn は "path/to/file.go:8:6" を (file, line, nil) に分解する。
func parsePosn(posn string) (file string, line int, err error) {
	// 最後の2つの ":N" を取り除く
	last := strings.LastIndex(posn, ":")
	if last < 0 {
		return "", 0, fmt.Errorf("parsePosn: no colon in %q", posn)
	}
	rest := posn[:last] // "file.go:8"
	mid := strings.LastIndex(rest, ":")
	if mid < 0 {
		return "", 0, fmt.Errorf("parsePosn: expected file:line:col in %q", posn)
	}
	lineStr := rest[mid+1:]
	n, err := strconv.Atoi(lineStr)
	if err != nil {
		return "", 0, fmt.Errorf("parsePosn: invalid line %q: %w", lineStr, err)
	}
	return rest[:mid], n, nil
}

// parseDiagMessage は analyzer が emit する 2 種類の診断メッセージを分解する:
//
//	"FuncName returns sentinels: a, b, c"                      → concrete=""
//	"FuncName returns sentinels via *pkg.Concrete: a, b, c"    → concrete="*pkg.Concrete"
//
// per-concrete で sentinel が無い場合は "(none)" が入るので空扱いにする。
func parseDiagMessage(msg string) (funcName, concrete string, sentinels []string, err error) {
	const marker = " returns sentinels"
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return "", "", nil, fmt.Errorf("parseMessage: marker not found in %q", msg)
	}
	funcName = msg[:idx]
	rest := msg[idx+len(marker):]
	switch {
	case strings.HasPrefix(rest, " via "):
		rest = rest[len(" via "):]
		colonIdx := strings.Index(rest, ": ")
		if colonIdx < 0 {
			return "", "", nil, fmt.Errorf("parseMessage: missing ':' after via in %q", msg)
		}
		concrete = rest[:colonIdx]
		rest = rest[colonIdx+len(": "):]
	case strings.HasPrefix(rest, ": "):
		rest = rest[len(": "):]
	default:
		return "", "", nil, fmt.Errorf("parseMessage: unexpected format in %q", msg)
	}
	for _, s := range strings.Split(rest, ", ") {
		s = strings.TrimSpace(s)
		if s == "" || s == "(none)" {
			continue
		}
		sentinels = append(sentinels, s)
	}
	return funcName, concrete, sentinels, nil
}
