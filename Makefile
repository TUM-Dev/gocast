.PHONY: all
all: npm_dependencies spa go_dependencies bundle

VERSION := $(shell git rev-parse --short origin/HEAD)

.PHONY: npm_dependencies
npm_dependencies:
	cd web; \
	npm i --no-dev

# Builds the single-page frontend into web/spa, where it is embedded into the binary.
# Skipping this target leaves every page served by its template handler.
.PHONY: spa
spa:
	cd frontend; \
	npm ci; \
	npm run build

# Regenerates the TypeScript client in frontend/src/gen from apiv2/server/apiv2.proto.
# The output is committed, so this only needs running when the proto changes; use it
# together with apiv2/generate.sh, which regenerates the Go side from the same file.
.PHONY: proto_es
proto_es:
	cd frontend; \
	npm ci; \
	npm run proto

.PHONY: go_dependencies
go_dependencies:
	go get ./...

.PHONY: bundle
bundle:
	go build -o main -ldflags="-X 'main.VersionTag=$(VERSION)'" cmd/tumlive/main.go

.PHONY: clean
clean:
	rm -fr web/node_modules
	rm -fr frontend/node_modules
	rm -fr web/spa/assets web/spa/index.html

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
	go run cmd/tumlive/main.go

.PHONY: test
test:
	go test -race ./...
	cd frontend; npm test

# Loads tum-live-starter.sql into the development database, dropping whatever was
# there. That dump is the fixture the browser tests assert against — its users, courses
# and lectures — so they need it as written, not as some earlier run left it. The
# server migrates the 2022 schema forward on boot.
#
# Runs the client inside the database container so its version always matches the
# server's and nothing depends on what is installed on the host. docker-compose.yml
# names the container mariadb_container; override for a differently named one:
#
#   make e2e_db DB_CONTAINER=mariadb_container
DB_CONTAINER ?= mariadb-tumlive

.PHONY: e2e_db
e2e_db:
	@docker inspect -f . $(DB_CONTAINER) >/dev/null 2>&1 || { \
		echo "no container named $(DB_CONTAINER); pass DB_CONTAINER=<name>"; exit 1; }
	docker exec -i $(DB_CONTAINER) mariadb -uroot -pexample \
		-e "DROP DATABASE IF EXISTS tumlive;"
	docker exec -i $(DB_CONTAINER) mariadb -uroot -pexample < tum-live-starter.sql

# Browser tests. Not part of `test`: they need the database container up, and they
# drive a real browser.
#
#   make test_e2e
#
# The fixture is reloaded first, every time, rather than left to whoever remembers.
# Some of these tests write — settings, and the runner the delete test consumes, which
# nothing can put back — so a second run against a fixture the first one changed fails
# for reasons that have nothing to do with the code.
#
# The SPA is built first for the same reason: without web/spa the migrated pages fall
# back to their templates, and the tests that assert a page is served the shell fail
# saying so, which reads as a routing bug rather than a missing build.
#
# The server is started by playwright.config.ts once the reload has finished, and
# stopped afterwards. The order is the point: the dump is the 2022 schema and the
# server migrates it forward on boot, creating tables the dump does not contain, so a
# reload underneath a running server takes those away until it is restarted.
#
# To run them against a server of your own instead — the `npm run dev` loop, say —
# name it and reload the fixture yourself:
#
#   make e2e_db && make run          # in another terminal
#   cd frontend && E2E_BASE_URL=http://localhost:8081 npm run test:e2e
.PHONY: test_e2e
test_e2e: spa e2e_db
	cd frontend; \
	npx playwright install --with-deps chromium; \
	npm run test:e2e

.PHONY: lint
lint:
	golangci-lint run
	cd web; npm run lint
	cd frontend; npm run typecheck

.PHONY: protoVoice
protoVoice:
	cd voice-service; \
	protoc ./subtitles.proto --go-grpc_out=../. --go_out=../.

