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

# The dump dates its lectures off the clock -- NOW() and CURDATE() -- which MariaDB
# evaluates in the session's time zone. The server then reads those columns back as
# local time (loc=Local, cmd/tumlive/main.go) and the browser judges "today" in its
# own, so all three have to agree on what day it is. They do not by default: the
# database usually runs UTC inside its container while the host does not, and every
# fixture date lands off by that offset -- far enough that the evening lecture the
# "Today" test wants is dated to yesterday, and the waiting-room lecture meant to
# start in 25 minutes has already ended. Pin the seeding session to this host's
# offset instead, which puts the whole fixture back in the frame the tests read it in.
#
# date(1) prints +0200; MariaDB wants +02:00. Done with sed rather than date's %:z,
# which GNU date has and BSD date does not.
HOST_TZ_OFFSET = $(shell date +%z | sed 's/..$$/:&/')

.PHONY: e2e_db
e2e_db:
	@docker inspect -f . $(DB_CONTAINER) >/dev/null 2>&1 || { \
		echo "no container named $(DB_CONTAINER); pass DB_CONTAINER=<name>"; exit 1; }
	docker exec -i $(DB_CONTAINER) mariadb -uroot -pexample \
		-e "DROP DATABASE IF EXISTS tumlive;"
	{ echo "SET time_zone = '$(HOST_TZ_OFFSET)';"; cat tum-live-starter.sql; } | \
		docker exec -i $(DB_CONTAINER) mariadb -uroot -pexample

# Browser tests. Not part of `test`: they need the database container and a browser.
#
# Some of these write, so the fixture is reloaded every run — once per run, not per
# file, so a test that changes a seeded account breaks later files. The SPA is built
# first, or the migrated pages fall back to their templates and fail confusingly.
#
# playwright.config.ts starts the server, after the reload: the dump is the 2022
# schema and the server migrates it on boot, so reloading under a running one takes
# away the tables it created.
#
# To use a server of your own instead:
#
#   make e2e_db && make run          # in another terminal
#   cd frontend && E2E_BASE_URL=http://localhost:8081 npm run test:e2e
#
# The browser install is deliberately not `--with-deps`: that shells out to apt-get
# under sudo, which prompts for a password and then fails on any distribution without
# it. The CI workflow asks for the system libraries there, where apt-get exists.
.PHONY: test_e2e
test_e2e: spa e2e_db
	cd frontend && \
	npx playwright install chromium && \
	npm run test:e2e

# Coverage for ./apiv2 measured from the browser tests, which are the only thing that
# exercises the API through the gateway rather than by calling a handler.
#
# Two things about the build are load-bearing. cmd/tumlive is instrumented alongside
# apiv2 because the exit hook that writes the counters is only registered when the main
# package is covered — with apiv2 alone the run produces nothing at all. And the
# counters are written as the server exits, so playwright.config.ts stops it with
# SIGTERM; killed outright it writes nothing either. `-pkg` keeps the report to apiv2.
E2E_COVER_PKG ?= github.com/TUM-Dev/gocast/apiv2/...
E2E_COVER_DIR ?= cov/e2e

.PHONY: test_e2e_cover
test_e2e_cover: spa e2e_db
	rm -rf $(E2E_COVER_DIR)
	mkdir -p $(E2E_COVER_DIR)
	go build -cover -coverpkg=./apiv2/...,./cmd/tumlive -o cov/tumlive ./cmd/tumlive
	cd frontend && \
	npx playwright install chromium && \
	GOCOVERDIR=$(CURDIR)/$(E2E_COVER_DIR) E2E_SERVER_CMD=$(CURDIR)/cov/tumlive \
	npm run test:e2e
	go tool covdata percent -i=$(E2E_COVER_DIR) -pkg=$(E2E_COVER_PKG)
	go tool covdata textfmt -i=$(E2E_COVER_DIR) -pkg=$(E2E_COVER_PKG) -o=$(E2E_COVER_DIR)/coverage.out
	@echo
	@echo "line-by-line: go tool cover -html=$(E2E_COVER_DIR)/coverage.out"

.PHONY: lint
lint:
	golangci-lint run
	cd web; npm run lint
	cd frontend; npm run typecheck

.PHONY: protoVoice
protoVoice:
	cd voice-service; \
	protoc ./subtitles.proto --go-grpc_out=../. --go_out=../.

