.PHONY: build test race lint fmt vet ci run snapshot

build:
	go build -o ac ./cmd/ac

test:
	go test ./...

race:
	go test -race ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...

# ci mirrors the GitHub CI gates: format/vet, then the full test suite.
ci: lint test

run:
	go run ./cmd/ac

snapshot:
	go run ./cmd/ac today
