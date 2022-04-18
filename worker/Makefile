all: build

VERSION := $(shell git rev-parse --short origin/HEAD)

protoGen:
	cd api; \
	protoc ./api.proto --go-grpc_out=../.. --go_out=../..

build: deps
	go build -ldflags="-X 'main.VersionTag=$(VERSION)'" app/main/main.go;

deps:
	go get ./...;

install:
	mv main /bin/worker