BIN        := sentinelfind
PROXY_BIN  := sentinel-lsp-proxy
LSP_BIN    := sentinel-lsp
GOBIN      := $(shell go env GOPATH)/bin

.PHONY: all build install dev-setup dev-tools check-tools test test-all test-v test-analyzer test-flags test-neovim-compat test-editor-nvim test-editor-vscode test-editor lint clean demo demo-example

all: build

build:
	go build -o $(BIN) ./cmd/sentinelfind
	go build -o $(PROXY_BIN) ./cmd/sentinel-lsp-proxy
	go build -o $(LSP_BIN) ./cmd/sentinel-lsp

install:
	go install ./cmd/sentinelfind
	go install ./cmd/sentinel-lsp-proxy
	go install ./cmd/sentinel-lsp
	@echo "installed to $(GOBIN)"

dev-tools:
	go mod download
	go install golang.org/x/tools/gopls@latest
	go install ./cmd/sentinelfind
	go install ./cmd/sentinel-lsp-proxy
	go install ./cmd/sentinel-lsp
	@echo "dev tools installed to $(GOBIN)"

check-tools:
	@command -v go >/dev/null || (echo "go not found"; exit 1)
	@command -v nvim >/dev/null || (echo "nvim not found"; exit 1)
	@command -v code >/dev/null || (echo "code not found"; exit 1)
	@command -v gopls >/dev/null || (echo "gopls not found. run: make dev-tools"; exit 1)
	@command -v sentinelfind >/dev/null || (echo "sentinelfind not found. run: make dev-tools"; exit 1)
	@command -v sentinel-lsp-proxy >/dev/null || (echo "sentinel-lsp-proxy not found. run: make dev-tools"; exit 1)
	@echo "tool check: OK"

dev-setup: dev-tools check-tools build
	@chmod +x scripts/editor-test-nvim.sh scripts/editor-test-vscode.sh
	@echo "development setup complete"

test:
	go test ./...

test-all: check-tools test test-editor

test-v:
	go test ./analyzer/... -v

test-analyzer:
	go test ./analyzer/ -run TestAnalyzer -v

test-flags:
	go test ./cmd/sentinelfind/ -v

test-neovim-compat:
	go test ./proxy -run 'TestCache_ParseJSON_(WrappedDiagnosticsObject|SingleDiagnosticObject)|TestProxy_Hover' -v
	go test ./cmd/sentinel-lsp-proxy -run TestE2E_ -v

test-editor-nvim: build
	./scripts/editor-test-nvim.sh

test-editor-vscode: build
	./scripts/editor-test-vscode.sh

test-editor: test-editor-nvim test-editor-vscode

lint: build
	./$(BIN) ./analyzer/...

demo: build
	-./$(BIN) ./analyzer/testdata/src/...

demo-example: build
	-cd example && ../$(BIN) ./...

clean:
	rm -f $(BIN) $(PROXY_BIN) $(LSP_BIN)
