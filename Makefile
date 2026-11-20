.PHONY: build test vet fmt fuzz

build:
	CGO_ENABLED=0 go build -o genc ./04-coding/cmd/genc

test:
	go test -race ./...
	GOEXPERIMENT=runtimesecret go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

fuzz:
	go test ./04-coding/internal/core -run '^$$' -fuzz '^FuzzParseContainer$$' -fuzztime 10s
	go test ./04-coding/internal/core -run '^$$' -fuzz '^FuzzRoundTrip$$' -fuzztime 10s

.PHONY: structural functional

structural:
	sh 05-structural-testing/run.sh

functional: build
	python3 06-functional-testing/run.py ./genc
