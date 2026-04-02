// dummy-gopls は E2E テスト用の最小 LSP サーバー。
// textDocument/hover リクエストに対して固定の Markdown レスポンスを返す。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/YukiYuigishi/errsweep/proxy"
)

func main() {
	r := bufio.NewReader(os.Stdin)
	for {
		raw, err := proxy.ReadMessage(r)
		if err != nil {
			return
		}

		var msg struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id,omitempty"`
			Method  string          `json:"method,omitempty"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		// 通知（id なし）は無視
		if len(msg.ID) == 0 {
			continue
		}

		var result interface{}
		switch msg.Method {
		case "initialize":
			result = map[string]interface{}{"capabilities": map[string]interface{}{}}
		case "shutdown":
			result = nil
		case "textDocument/hover":
			result = map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": "```go\nfunc GetUser(id int) (string, error)\n```",
				},
			}
		default:
			result = nil
		}

		resp, err := json.Marshal(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      msg.ID,
			"result":  result,
		})
		if err != nil {
			continue
		}
		if err := proxy.WriteMessage(os.Stdout, resp); err != nil {
			fmt.Fprintf(os.Stderr, "dummy-gopls: write: %v\n", err)
			return
		}
	}
}
