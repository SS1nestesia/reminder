.PHONY: build run test fmt clean

BINARY_NAME=bot

build:
	go build -o $(BINARY_NAME) ./cmd/bot

run:
	go run ./cmd/bot

test:
	go test ./... -v -count=1

fmt:
	go fmt ./...

clean:
	rm -f $(BINARY_NAME)
	rm -f bot_binary
