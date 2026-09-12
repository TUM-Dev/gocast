# AGENTS.md

GoCast (TUM-Live) is a lecture streaming platform: a Go monolith that serves the web
app and API, plus separate services that capture, transcode and deliver video.

## Layout

Five Go modules, joined by `go.work`:

| Path | What |
|---|---|
| `.` | The main server — `cmd/tumlive` is the entrypoint |
| `worker/` | Captures and transcodes streams |
| `worker/edge/` | Edge delivery node |
| `runner/` | Newer replacement for the worker |
| `vod-service/` | Video-on-demand serving |

Inside the main module: `model/` (GORM models), `dao/` (data access), `api/` (the REST
API, v1), `apiv2/` (Protobuf/Connect API), `web/` (server-rendered pages), `tools/`
(shared helpers), `mock_dao/` + `mock_tools/` (generated). `ingest/`, `rtmp-proxy/` and
`voice-service/` are supporting services; `docs_v2/` is the Docusaurus site.

## The two frontends — read this before touching UI

The app is **mid-migration from server-rendered templates to a Vue SPA**, page by page.
Both are live at once, and which one serves a given URL is a deliberate choice:

- **`web/`** — Go templates (`web/template/*.gohtml`) with Alpine.js and TypeScript in
  `web/ts/`, bundled by webpack. The legacy side.
- **`frontend/`** — Vue 3 + Vite SPA, with Pinia and vue-router. Builds into `web/spa/`,
  which is embedded into the Go binary.

`spaRoutes` in [`web/router.go`](web/router.go) is the source of truth for which paths
the SPA owns. Moving a page across means adding its path **both** there and in
`frontend/src/router/index.ts` — the shell renders nothing if the client router doesn't
know the route. `web/spa_test.go` covers the Go half of that contract.

Consequence worth internalising: **if you don't build the SPA, every migrated route
silently falls back to its old template.** You will be looking at the frontend being
replaced and think it works.

## Build and test

```bash
make test                            # go test -race ./... + frontend unit tests
make lint                            # golangci-lint + web eslint + frontend typecheck
npm --prefix frontend test           # vitest
npm --prefix frontend run typecheck  # vue-tsc
npm --prefix web run lint            # eslint (flat config; lints web/ts only, on purpose)
```

`npm ci` in `web/` is required to **compile the Go server at all** — `web/router.go`
has `//go:embed node_modules`. It is not just about icons rendering.

For running the app, browser testing, and visual regression, use the
**`local-testing` skill** (`.claude/skills/local-testing/`). It has the database
seeding, the asset build order, the fixture accounts and the screenshot-diff scripts.
Don't reconstruct that from scratch.

## Generated code is committed

Regenerate deliberately; don't hand-edit the output.

```bash
make mocks                 # go generate ./... -> mock_dao/, mock_tools/
./apiv2/generate.sh        # apiv2.proto -> Go server + docs
make proto_es              # apiv2.proto -> frontend/src/gen (TypeScript client)
go run cmd/modelGen/modelGen.go <ModelName>   # scaffolds a model + dao
```

The proto is one file, `apiv2/server/apiv2.proto`, and both sides generate from it.
Change it and run both generators, or the Go and TypeScript halves drift.

New API work belongs in `apiv2/` (Protobuf/Connect), not `api/`.

## v1 is being deleted — spend the effort on v2

The migration's endpoint is v2 replacing v1, so refactoring v1 is work that gets
thrown away. Keep edits to the legacy v1 API (`api/`, and the `web/` gohtml handlers)
to the minimum the task needs: fix bugs, and do what a v2 change forces — no
opportunistic cleanup. The higher bar applies on the v2 side instead: interceptors,
declarative authorization, tests.

When a shared package (`model/`, `dao/`, `tools/`) needs to change, prefer adding the
new thing over rewriting v1's use of the old one.

## Database

GORM `AutoMigrate` runs on boot from `cmd/tumlive/main.go`, so adding a field to a model
is usually enough. Anything `AutoMigrate` can't express — backfills, renames, data
fixes — goes in `dao/migrations/` as a dated file.

`tum-live-starter.sql` is the development fixture, and the browser tests assert on its
contents. Treat its users, courses and lectures as an API.

## Conventions

- **Conventional commits**, with a scope: `feat(apiv2):`, `fix(runner):`,
  `perf(frontend):`, `chore(web):`. Branch names follow the same shape
  (`fix/vod-after-early-end`, `feat/speedup-public-courses-rpc`).
- `dev` is the default branch and what PRs target; there is no `main`. Follow
  `.github/pull_request_template.md` — motivation, description, **steps for testing**,
  and screenshots for UI changes.
- Comments should explain *why*, especially where the code looks odd. This codebase
  does that well; match it. Several comments exist specifically to stop someone
  "fixing" a deliberate choice — leave those in place.
- Dependencies are Renovate's job. `renovate.json` carries version holds with the
  reason and the condition for removing each one; read it before pinning something by
  hand.

## Traps

- **Tailwind 4 utilities are unlayered on purpose.** `web/assets/css/{main,home}.css`
  compose the Tailwind entry by hand instead of `@import "tailwindcss"`, because
  layered utilities lose to unlayered vendor CSS (video.js, fullcalendar, flatpickr,
  nouislider) regardless of specificity or order. Restoring the plain import silently
  breaks layout wherever a utility meets a vendor stylesheet. See the `local-testing`
  skill.
- **`go run cmd/tumlive/main.go` leaves a process `pkill -f` won't match** — it runs a
  compiled binary from a temp path. Kill it by port.
- The `ldap:` block in `config.yaml` stays commented out in development: with
  `useForLogin`, every rejected password falls back to LDAP and hangs if the host
  doesn't resolve.
- Meilisearch connection errors on boot are expected locally; only search is affected.
- `web/spa/` holds a tracked marker so `go:embed` resolves on a fresh checkout, which
  is why the Vite build deliberately does **not** empty it; the `prebuild` script
  clears stale hashed assets instead.
