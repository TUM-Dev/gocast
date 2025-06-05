.PHONY: all
all: npm_dependencies go_dependencies bundle

VERSION := $(shell git rev-parse --short origin/HEAD)

.PHONY: npm_dependencies
npm_dependencies:
	cd web; \
	npm i --no-dev

.PHONY: go_dependencies
go_dependencies:
	go get ./...

.PHONY: bundle
bundle:
	go build -o main -ldflags="-X 'main.VersionTag=$(VERSION)'" cmd/tumlive/tumlive.go

.PHONY: clean
clean:
	rm -fr web/node_modules

.PHONY: install
install:
	mv main /bin/tum-live

.PHONY: mocks
mocks:
	go generate ./...

.PHONY: run_web
run_web:
	cd web; \
	npm i --include=dev

.PHONY: run
run:
	go run cmd/tumlive/tumlive.go

.PHONY: test
test:
	go test -race ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: protoVoice
protoVoice:
	cd voice-service; \
	protoc ./subtitles.proto --go-grpc_out=../. --go_out=../.

