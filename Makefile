all: npm_dependencies go_dependencies bundle

VERSION := $(shell git rev-parse --short origin/HEAD)

npm_dependencies:
	cd web; \
	npm i --no-dev;

go_dependencies:
	go get ./...

bundle:
	go build -o main -ldflags="-X 'main.VersionTag=$(VERSION)'" cmd/tumlive/tumlive.go

clean:
	rm -fr web/node_modules

install:
	mv main /bin/tum-live

mocks:
	go generate ./...

protoVoice:
	cd voice-service; \
	protoc ./subtitles.proto --go-grpc_out=../. --go_out=../.
