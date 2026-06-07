.PHONY: build test lint run snapshot

build:
	go build -o ac ./cmd/ac

test:
	go test ./...

run:
	go run ./cmd/ac

snapshot:
	go run ./cmd/ac today
