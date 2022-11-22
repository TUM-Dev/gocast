.PHONY: all
all: build

VERSION := $(shell git rev-parse --short origin/HEAD)

.PHONY: protoGen
protoGen:
	cd api; \
	protoc ./api.proto --go-grpc_out=../.. --go_out=../..

.PHONY: build
build: deps
	go build -o main -ldflags="-X 'main.VersionTag=$(VERSION)'" cmd/worker/worker.go

.PHONY: deps
deps:
	go get ./...

.PHONY: install
install:
	mv main /bin/worker

.PHONY: clean
clean:
	rm -f main

.PHONY: test
test:
	go test -race ./...

.PHONY: run
run:
	go run cmd/worker/worker.go

