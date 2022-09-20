all: build

VERSION := $(shell git rev-parse --short origin/HEAD)

protoGen:
	cd api; \
	protoc ./api.proto --go-grpc_out=../.. --go_out=../..

protoVoice:
	cd voice-service; \
	protoc ./subtitles.proto --go-grpc_out=../.. --go_out=../..

build: deps
	go build -o main -ldflags="-X 'main.VersionTag=$(VERSION)'" cmd/worker/worker.go;

deps:
	go get ./...;

install:
	mv main /bin/worker