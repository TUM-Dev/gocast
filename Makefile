.PHONY: docs

all: npm_dependencies go_dependencies bundle docs

VERSION := $(shell git rev-parse --short origin/HEAD)

npm_dependencies:
	cd web; \
	npm i --no-dev;

go_dependencies:
	go get ./...

bundle:
	go build -ldflags="-X 'main.VersionTag=$(VERSION)'" app/server/main.go

clean:
	rm -fr web/node_modules

install:
	mv main /bin/tum-live

docs:
	cd docs; \
	mkdocs build;