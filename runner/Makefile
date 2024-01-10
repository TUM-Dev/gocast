.PHONY: all
all: build

VERSION := $(shell git rev-parse --short origin/HEAD)

.PHONY: protoGen
protoGen:
	protoc ./runner.proto --go-grpc_out=.. --go_out=..

.PHONY: build
build: deps
	go build -o main -ldflags="-X 'main.VersionTag=$(VERSION)'" cmd/runner/main.go

.PHONY: deps
deps:
	go get ./...

.PHONY: install
install:
	mv main /bin/runner

.PHONY: clean
clean:
	rm -f main

.PHONY: test
test:
	go test -race ./...

.PHONY: run
run:
	go run cmd/runner/main.go

.PHONY: lint
lint:
	golangci-lint run
