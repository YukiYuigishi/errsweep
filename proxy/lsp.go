package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadMessage は LSP の Content-Length フレームを読み込み、本文を返す（公開 API）。
func ReadMessage(r io.Reader) ([]byte, error) { return readMessage(r) }

// WriteMessage は LSP の Content-Length フレームに包んで書き込む（公開 API）。
func WriteMessage(w io.Writer, body []byte) error { return writeMessage(w, body) }

// readMessage は LSP の Content-Length フレームを読み込み、本文を返す。
func readMessage(r io.Reader) ([]byte, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}

	var contentLength int
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("readMessage header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // ヘッダ終端の空行
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			val := strings.TrimPrefix(line, "Content-Length: ")
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("readMessage: invalid Content-Length %q: %w", val, err)
			}
			contentLength = n
		}
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("readMessage: missing or zero Content-Length")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(br, body); err != nil {
		return nil, fmt.Errorf("readMessage body: %w", err)
	}
	return body, nil
}

// writeMessage は LSP の Content-Length フレームに包んで書き込む。
func writeMessage(w io.Writer, body []byte) error {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(w, header); err != nil {
		return fmt.Errorf("writeMessage header: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("writeMessage body: %w", err)
	}
	return nil
}

// Message は JSON-RPC 2.0 メッセージの共通フィールド。
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// isRequest は JSON-RPC リクエスト（id + method）かを判定する。
func (m *Message) isRequest() bool { return m.Method != "" && len(m.ID) > 0 }

// isResponse は JSON-RPC レスポンス（id のみ、method なし）かを判定する。
func (m *Message) isResponse() bool { return m.Method == "" && len(m.ID) > 0 }

// parseMessage は raw JSON を Message にデコードする。
func parseMessage(raw []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, fmt.Errorf("parseMessage: %w", err)
	}
	return &msg, nil
}
