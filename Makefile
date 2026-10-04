.PHONY: run test vet build

run:
	go run ./cmd/agent-cli

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o bin/agent-cli ./cmd/agent-cli
