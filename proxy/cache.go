package proxy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// CacheEntry は1関数分の解析結果。
type CacheEntry struct {
	FuncName  string
	Sentinels []string // e.g. ["io.EOF", "sql.ErrNoRows"]
}

// markdown は hover に追記する Markdown テキストを返す。
func (e *CacheEntry) markdown() string {
	var sb strings.Builder
	sb.WriteString("---\n**Possible Sentinel Errors:**\n")
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

// Cache はファイル位置 → CacheEntry のマッピング。
type Cache map[cacheKey]*CacheEntry

// lookup は指定ファイル・行番号の CacheEntry を返す。
func (c Cache) lookup(file string, line int) (*CacheEntry, bool) {
	entry, ok := c[cacheKey{file: file, line: line}]
	return entry, ok
}

// sentinelfindOutput は `sentinelfind -json` の出力形式。
// map[pkgPath]map["sentinelfind"][]diagnostic
type sentinelfindOutput map[string]map[string][]struct {
	Posn    string `json:"posn"`
	Message string `json:"message"`
}

// ParseSentinelfindJSON は `sentinelfind -json` の出力を Cache に変換する（公開 API）。
func ParseSentinelfindJSON(data []byte) (Cache, error) { return parseSentinelfindJSON(data) }

// parseSentinelfindJSON は `sentinelfind -json` の出力を Cache に変換する。
func parseSentinelfindJSON(data []byte) (Cache, error) {
	var out sentinelfindOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parseSentinelfindJSON: %w", err)
	}

	cache := make(Cache)
	for _, checks := range out {
		for _, diags := range checks {
			for _, d := range diags {
				file, line, err := parsePosn(d.Posn)
				if err != nil {
					continue
				}
				funcName, sentinels, err := parseDiagMessage(d.Message)
				if err != nil {
					continue
				}
				cache[cacheKey{file: file, line: line}] = &CacheEntry{
					FuncName:  funcName,
					Sentinels: sentinels,
				}
			}
		}
	}
	return cache, nil
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

// parseDiagMessage は "FuncName returns sentinels: a, b, c" を分解する。
func parseDiagMessage(msg string) (funcName string, sentinels []string, err error) {
	const marker = " returns sentinels: "
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return "", nil, fmt.Errorf("parseMessage: marker not found in %q", msg)
	}
	funcName = msg[:idx]
	rest := msg[idx+len(marker):]
	for _, s := range strings.Split(rest, ", ") {
		s = strings.TrimSpace(s)
		if s != "" {
			sentinels = append(sentinels, s)
		}
	}
	return funcName, sentinels, nil
}
