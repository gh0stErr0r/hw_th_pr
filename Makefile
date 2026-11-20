TEST_OVERLAY := 05-structural-testing/overlay.json

.PHONY: build test vet fmt fuzz

build:
	CGO_ENABLED=0 go build -o genc ./04-coding/cmd/genc

test:
	go test -overlay=$(TEST_OVERLAY) -race ./...
	GOEXPERIMENT=runtimesecret go test -overlay=$(TEST_OVERLAY) ./...

vet:
	go vet -overlay=$(TEST_OVERLAY) ./...

fmt:
	gofmt -w 04-coding 05-structural-testing/testdata

fuzz:
	go test -overlay=$(TEST_OVERLAY) ./04-coding/internal/core -run '^$$' -fuzz '^FuzzParseContainer$$' -fuzztime 10s
	go test -overlay=$(TEST_OVERLAY) ./04-coding/internal/core -run '^$$' -fuzz '^FuzzRoundTrip$$' -fuzztime 10s

.PHONY: structural functional

structural:
	sh 05-structural-testing/run.sh

functional: build
	python3 06-functional-testing/run.py ./genc
