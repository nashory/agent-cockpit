.PHONY: build test race lint fmt vet ci bench run snapshot

build:
	go build -o cockpit ./cmd/cockpit

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

bench:
	go test -run '^$$' -bench=. -benchmem ./internal/cli ./internal/source ./internal/usage

run:
	go run ./cmd/cockpit

snapshot:
	go run ./cmd/cockpit today

# Refresh the vendored LiteLLM pricing table (run before a release).
pricing:
	python3 scripts/gen-pricing.py
