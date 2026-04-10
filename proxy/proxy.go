package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// pendingRequest は hover リクエストの位置情報を保持する。
type pendingRequest struct {
	file string
	line int // 1-indexed
}

// Proxy は LSP メッセージをインターセプトして Sentinel 情報を付加する。
type Proxy struct {
	cache    Cache
	mu       sync.Mutex
	pending  map[string]pendingRequest // JSON-RPC id (文字列化) → リクエスト情報
}

// NewProxy は新しい Proxy を生成する。
func NewProxy(cache Cache) *Proxy {
	return &Proxy{
		cache:   cache,
		pending: make(map[string]pendingRequest),
	}
}

// TrackRequest は公開 API。
func (p *Proxy) TrackRequest(raw []byte) error { return p.trackRequest(raw) }

// ProcessServerMessage は公開 API。
func (p *Proxy) ProcessServerMessage(raw []byte, w io.Writer) error {
	return p.processServerMessage(raw, w)
}

// trackRequest はクライアントからのリクエストを記録する。
// textDocument/hover のリクエスト位置を後でレスポンスと突き合わせるために使う。
func (p *Proxy) trackRequest(raw []byte) error {
	msg, err := parseMessage(raw)
	if err != nil {
		return err
	}
	if msg.Method != "textDocument/hover" || !msg.isRequest() {
		return nil
	}

	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position struct {
			Line      int `json:"line"` // 0-indexed
			Character int `json:"character"`
		} `json:"position"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return fmt.Errorf("trackRequest: %w", err)
	}

	file := uriToPath(params.TextDocument.URI)
	line := params.Position.Line + 1 // 0-indexed → 1-indexed

	p.mu.Lock()
	p.pending[idKey(msg.ID)] = pendingRequest{file: file, line: line}
	p.mu.Unlock()
	return nil
}

// processServerMessage はサーバー（gopls）からのメッセージを処理し、必要なら改変して w に書く。
func (p *Proxy) processServerMessage(raw []byte, w io.Writer) error {
	msg, err := parseMessage(raw)
	if err != nil {
		return writeMessage(w, raw)
	}

	// hover レスポンスのみインターセプト
	if !msg.isResponse() || len(msg.Result) == 0 {
		return writeMessage(w, raw)
	}

	p.mu.Lock()
	req, ok := p.pending[idKey(msg.ID)]
	if ok {
		delete(p.pending, idKey(msg.ID))
	}
	p.mu.Unlock()

	if !ok {
		return writeMessage(w, raw)
	}

	// 1. 定義行による検索（定義ファイルでのホバー）
	entry, hit := p.cache.lookup(req.file, req.line)

	// 2. 関数名による検索（呼び出し側でのホバー）
	// gopls のホバーレスポンスには関数シグネチャが含まれるため、
	// そこから関数名を抽出してキャッシュを引く。
	if !hit {
		if name := extractFuncNameFromResult(msg.Result); name != "" {
			entry, hit = p.cache.lookupByFuncName(name)
		}
	}

	if !hit {
		return writeMessage(w, raw)
	}

	modified, err := appendSentinelToHover(raw, entry)
	if err != nil {
		// 改変に失敗しても元のレスポンスを返す
		return writeMessage(w, raw)
	}
	return writeMessage(w, modified)
}

// appendSentinelToHover は hover レスポンスの contents に Sentinel 情報を追記する。
func appendSentinelToHover(raw []byte, entry *CacheEntry) ([]byte, error) {
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	resultRaw, ok := resp["result"]
	if !ok {
		return nil, fmt.Errorf("no result field")
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return nil, err
	}

	contentsRaw, ok := result["contents"]
	if !ok {
		return nil, fmt.Errorf("no contents field")
	}

	addition := entry.markdown()

	// MarkupContent {"kind":"markdown","value":"..."} または文字列
	var markup struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(contentsRaw, &markup); err == nil && markup.Kind != "" {
		// MarkupContent 形式
		markup.Value += "\n" + addition
		newContents, err := json.Marshal(markup)
		if err != nil {
			return nil, err
		}
		result["contents"] = newContents
	} else {
		// 文字列形式
		var s string
		if err := json.Unmarshal(contentsRaw, &s); err != nil {
			return nil, err
		}
		s += "\n" + addition
		newContents, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		result["contents"] = newContents
	}

	newResult, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	resp["result"] = newResult

	return json.Marshal(resp)
}

// funcNameRe は gopls のホバーレスポンスから関数名を抽出する正規表現。
// "func FindUser(" や "func (r *Repo) FindUser(" に対応する。
var funcNameRe = regexp.MustCompile(`\bfunc\s+(?:\([^)]*\)\s+)?(\w+)\s*[(\[]`)

// extractFuncNameFromResult は hover レスポンスの result フィールドから関数名を抽出する。
func extractFuncNameFromResult(result json.RawMessage) string {
	var r struct {
		Contents json.RawMessage `json:"contents"`
	}
	if err := json.Unmarshal(result, &r); err != nil || len(r.Contents) == 0 {
		return ""
	}

	var text string
	var markup struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(r.Contents, &markup); err == nil && markup.Kind != "" {
		text = markup.Value
	} else {
		_ = json.Unmarshal(r.Contents, &text)
	}

	m := funcNameRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// idKey は json.RawMessage の id を map のキーに使える文字列に変換する。
func idKey(id json.RawMessage) string {
	return string(id)
}

// uriToPath は file:// URI をファイルパスに変換する。
func uriToPath(uri string) string {
	const prefix = "file://"
	if strings.HasPrefix(uri, prefix) {
		path := uri[len(prefix):]
		// パーセントエンコードの簡易デコード（%20 など）
		path = percentDecode(path)
		return path
	}
	return uri
}

// percentDecode は URL パーセントエンコードを簡易デコードする。
func percentDecode(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			b, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
			if err == nil {
				sb.WriteByte(byte(b))
				i += 3
				continue
			}
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}
