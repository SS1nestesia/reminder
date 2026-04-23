.PHONY: build run test test-race cover fmt vet lint tidy clean ci

BINARY_NAME := bot
PKG         := ./...

## build: compile the bot binary into ./$(BINARY_NAME)
build:
	go build -o $(BINARY_NAME) ./cmd/bot

## run: run the bot (requires TOKEN env var)
run:
	go run ./cmd/bot

## test: run all tests once (no cache)
test:
	go test $(PKG) -count=1

## test-race: run all tests with the race detector
test-race:
	go test $(PKG) -count=1 -race

## cover: produce coverage report (coverage.out + HTML)
cover:
	go test $(PKG) -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -n 1
	go tool cover -html=coverage.out -o coverage.html

## fmt: format and organise imports in all Go files
fmt:
	go fmt $(PKG)
	@command -v goimports >/dev/null 2>&1 && goimports -w . || true

## vet: run go vet on all packages
vet:
	go vet $(PKG)

## lint: run golangci-lint (requires it to be installed)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not installed. Install: https://golangci-lint.run/welcome/install/"; \
		exit 1; \
	}
	golangci-lint run

## tidy: keep go.mod/go.sum clean
tidy:
	go mod tidy

## ci: one-shot pipeline suitable for CI/local pre-push (fmt-check, vet, race tests)
ci: vet test-race
	@gofmt -l . | grep . && { echo "gofmt: the files above need formatting"; exit 1; } || echo "gofmt: OK"

## clean: remove build artifacts
clean:
	rm -f $(BINARY_NAME) bot_binary coverage.out coverage.html
