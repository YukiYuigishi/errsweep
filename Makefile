BIN        := errsweep
PROXY_BIN  := errsweep-lsp-proxy
LSP_BIN    := errsweep-lsp
GOBIN      := $(shell go env GOPATH)/bin

.PHONY: all build install dev-setup dev-tools check-tools setup-hooks test test-all test-v test-analyzer test-flags test-neovim-compat test-editor-nvim test-editor-vscode test-editor lint lint-go lint-fix clean demo demo-example bench-cache-pattern bench-cache-pattern-moby

all: build

build:
	go build -o $(BIN) ./cmd/errsweep
	go build -o $(PROXY_BIN) ./cmd/errsweep-lsp-proxy
	go build -o $(LSP_BIN) ./cmd/errsweep-lsp

install:
	go install ./cmd/errsweep
	go install ./cmd/errsweep-lsp-proxy
	go install ./cmd/errsweep-lsp
	@echo "installed to $(GOBIN)"

dev-tools:
	go mod download
	go install golang.org/x/tools/gopls@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install ./cmd/errsweep
	go install ./cmd/errsweep-lsp-proxy
	go install ./cmd/errsweep-lsp
	@echo "dev tools installed to $(GOBIN)"

check-tools:
	@command -v go >/dev/null || (echo "go not found"; exit 1)
	@command -v nvim >/dev/null || (echo "nvim not found"; exit 1)
	@command -v code >/dev/null || (echo "code not found"; exit 1)
	@command -v gopls >/dev/null || (echo "gopls not found. run: make dev-tools"; exit 1)
	@command -v golangci-lint >/dev/null || (echo "golangci-lint not found. run: make dev-tools"; exit 1)
	@command -v errsweep >/dev/null || (echo "errsweep not found. run: make dev-tools"; exit 1)
	@command -v errsweep-lsp-proxy >/dev/null || (echo "errsweep-lsp-proxy not found. run: make dev-tools"; exit 1)
	@echo "tool check: OK"

dev-setup: dev-tools check-tools build
	@chmod +x scripts/editor-test-nvim.sh scripts/editor-test-vscode.sh
	@chmod +x .githooks/pre-commit
	@git config core.hooksPath .githooks
	@echo "development setup complete"

setup-hooks:
	@chmod +x .githooks/pre-commit
	@git config core.hooksPath .githooks
	@echo "git hooks configured (.githooks)"

test:
	go test ./...

test-all: check-tools test test-editor

lint-go:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

test-v:
	go test ./analyzer/... -v

test-analyzer:
	go test ./analyzer/ -run TestAnalyzer -v

test-flags:
	go test ./cmd/errsweep/ -v

test-neovim-compat:
	go test ./proxy -run 'TestCache_ParseJSON_(WrappedDiagnosticsObject|SingleDiagnosticObject)|TestProxy_Hover' -v
	go test ./cmd/errsweep-lsp-proxy -run TestE2E_ -v

test-editor-nvim: build
	./scripts/editor-test-nvim.sh

test-editor-vscode: build
	./scripts/editor-test-vscode.sh

test-editor: test-editor-nvim test-editor-vscode

lint: build
	$(MAKE) lint-go
	./$(BIN) ./analyzer/...

demo: build
	-./$(BIN) ./analyzer/testdata/src/...

demo-example: build
	-cd example && ../$(BIN) ./...

bench-cache-pattern: build
	./scripts/bench-cache-pattern.sh

bench-cache-pattern-moby: build
	CACHE_BENCH_REPO=$(PWD)/tmp/moby CACHE_BENCH_PRESET=moby ./scripts/bench-cache-pattern.sh

bench-cache-pattern-check: build
	CACHE_BENCH_MAX_AVG_REAL=2.0 CACHE_BENCH_MAX_AVG_EXIT=0.0 ./scripts/bench-cache-pattern.sh

clean:
	rm -f $(BIN) $(PROXY_BIN) $(LSP_BIN)
