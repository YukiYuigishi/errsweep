BIN        := sentinelfind
PROXY_BIN  := sentinel-lsp-proxy
LSP_BIN    := sentinel-lsp
GOBIN      := $(shell go env GOPATH)/bin

.PHONY: all build install test test-v test-analyzer test-flags lint clean demo demo-example

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

test:
	go test ./...

test-v:
	go test ./analyzer/... -v

test-analyzer:
	go test ./analyzer/ -run TestAnalyzer -v

test-flags:
	go test ./cmd/sentinelfind/ -v

lint: build
	./$(BIN) ./analyzer/...

demo: build
	-./$(BIN) ./analyzer/testdata/src/...

demo-example: build
	-cd example && ../$(BIN) ./...

clean:
	rm -f $(BIN) $(PROXY_BIN) $(LSP_BIN)
