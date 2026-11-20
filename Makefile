.PHONY: build test vet fmt fuzz

build:
	CGO_ENABLED=0 go build -o genc ./cmd/genc

test:
	go test -race ./...
	GOEXPERIMENT=runtimesecret go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

fuzz:
	go test ./internal/core -run '^$$' -fuzz '^FuzzParseContainer$$' -fuzztime 10s
	go test ./internal/core -run '^$$' -fuzz '^FuzzRoundTrip$$' -fuzztime 10s
