package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestReadMessage_Single(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)

	msg, err := readMessage(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != body {
		t.Errorf("got %q, want %q", msg, body)
	}
}

func TestReadMessage_ExtraHeaders(t *testing.T) {
	body := `{"jsonrpc":"2.0","method":"initialized"}`
	input := fmt.Sprintf("Content-Type: application/vscode-jsonrpc; charset=utf-8\r\nContent-Length: %d\r\n\r\n%s", len(body), body)

	msg, err := readMessage(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if string(msg) != body {
		t.Errorf("got %q, want %q", msg, body)
	}
}

func TestReadMessage_Multiple(t *testing.T) {
	body1 := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	body2 := `{"jsonrpc":"2.0","id":2,"method":"shutdown"}`
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%sContent-Length: %d\r\n\r\n%s",
		len(body1), body1, len(body2), body2)

	// 同一の bufio.Reader を共有して連続読み込みをテスト
	r := bufio.NewReader(strings.NewReader(input))
	msg1, err := readMessage(r)
	if err != nil {
		t.Fatal(err)
	}
	msg2, err := readMessage(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(msg1) != body1 {
		t.Errorf("msg1: got %q, want %q", msg1, body1)
	}
	if string(msg2) != body2 {
		t.Errorf("msg2: got %q, want %q", msg2, body2)
	}
}

func TestWriteMessage(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)
	var buf bytes.Buffer
	if err := writeMessage(&buf, body); err != nil {
		t.Fatal(err)
	}
	want := "Content-Length: 36\r\n\r\n" + string(body)
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestReadMessage_InvalidLength(t *testing.T) {
	input := "Content-Length: abc\r\n\r\n{}"
	_, err := readMessage(strings.NewReader(input))
	if err == nil {
		t.Error("expected error for invalid Content-Length")
	}
}
