BIN := sentinelfind

.PHONY: all build test test-v test-analyzer lint clean demo

all: build

build:
	go build -o $(BIN) ./cmd/sentinelfind

test:
	go test ./...

test-v:
	go test ./analyzer/... -v

test-analyzer:
	go test ./analyzer/ -run TestAnalyzer -v

lint: build
	./$(BIN) ./analyzer/...

demo: build
	-./$(BIN) ./analyzer/testdata/src/...

clean:
	rm -f $(BIN)
