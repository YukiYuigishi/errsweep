BIN       := sentinelfind
PROXY_BIN := sentinel-lsp-proxy

.PHONY: all build test test-v test-analyzer test-flags lint clean demo demo-example

all: build

build:
	go build -o $(BIN) ./cmd/sentinelfind
	go build -o $(PROXY_BIN) ./cmd/sentinel-lsp-proxy

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
	rm -f $(BIN) $(PROXY_BIN)
