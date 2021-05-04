all: npm_dependencies go_dependencies bundle

VERSION := $(shell git rev-parse --short HEAD)

npm_dependencies:
	cd web; \
	npm i --no-dev;

go_dependencies:
	go get ./...

bundle:
	go build -ldflags="-X 'main.VersionTag=$(origin/HEAD)'" app/server/main.go

clean:
	rm -fr web/node_modules

install:
	mv main /bin/tum-live
