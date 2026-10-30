# AGENTS.md

GoCast (TUM-Live) is a lecture streaming platform: a Go monolith that serves the web
app and API, plus separate services that capture, transcode and deliver video.

## Layout

Four Go modules, joined by `go.work`:

| Path | What |
|---|---|
| `.` | The main server — `cmd/tumlive` is the entrypoint |
| `edge` | Edge delivery node |
| `runner/` | Captures and transcodes streams (replaced the old `worker/`, which is gone) |
| `vod-service/` | Video-on-demand serving |

Inside the main module: `model/` (GORM models), `dao/` (data access), `apiv2/` (Protobuf/Connect API,
plus the few plain-HTTP routes that scripts and webhooks call), `web/` (Go side of the SPA: routing, session pages, branding files), `tools/`
(shared helpers), `mock_dao/` + `mock_tools/` (generated). `ingest/`, `rtmp-proxy/` and
`voice-service/` are supporting services; `docs_v2/` is the Docusaurus site.

## The frontend

The only frontend is the **Vue 3 + Vite SPA in `frontend/`** (Pinia, vue-router). It builds
into `web/spa/`, which is embedded into the Go binary. There are no server-rendered
pages: Go answers a page request with the SPA shell, and `web/` is just routing, the
session/SAML handlers and the default branding files.

`spaRoutes` in [`web/router.go`](web/router.go) lists the paths the shell is served for.
Adding a page means adding its path **both** there and in `frontend/src/router/index.ts`
— the shell renders nothing if the client router doesn't know the route.
`web/spa_test.go` and `spa-routes.test.ts` cover the contract.

**The SPA build is mandatory.** The server panics at startup when `web/spa/index.html`
is missing, and the Docker build runs the Vite build. Run `make spa` (or
`pnpm --dir frontend run build`) before `go run`, `go build` for shipping, or `go test
./web`, which skips its router tests without a build.

Errors on page requests (403, 404, ...) are the shell with that status plus meta tags
the client renders (`tools.RenderErrorPage` → `web.serveErrorPage`); under `/api/` they
are plain text.

## Build and test

```bash
make test                            # go test -race ./... + frontend unit tests
make lint                            # golangci-lint + frontend typecheck
pnpm --dir frontend test              # vitest
pnpm --dir frontend run typecheck     # vue-tsc
```

For running the app, browser testing, and visual regression, use the
**`local-testing` skill** (`.claude/skills/local-testing/`). It has the database
seeding, the asset build order, the fixture accounts and the screenshot-diff scripts.
Don't reconstruct that from scratch.

## Generated code is committed

Regenerate deliberately; don't hand-edit the output.

```bash
make mocks                 # go generate ./... -> mock_dao/, mock_tools/
./apiv2/generate.sh        # server/*.proto -> Go server + docs
make proto_es              # server/*.proto -> frontend/src/gen (TypeScript client)
go run cmd/modelGen/modelGen.go <ModelName>   # scaffolds a model + dao
```

The protos live in `apiv2/server/`, and both sides generate from them.
Change one and run both generators, or the Go and TypeScript halves drift.

New API work belongs in `apiv2/` (Protobuf/Connect). The v1 `api/` package is gone; its
three external URLs (`/api/selfstream/onPublish`, `/api/token/proxy/:token`,
`/api/schedule.ics`) are served from `apiv2/server/legacy_routes.go`, each also at a
`/api/v2/...` address.

## v1 is deleted — spend the effort on v2

The migration's endpoint was v2 replacing v1, and the REST API (`api/`) is gone. What
remains of v1 is the `web/` gohtml handlers: keep edits to them
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
- **Releases are a tag push.** Everything downstream — images, Matrix notice, and the
  GitHub release notes rendered from the commit log — hangs off it. Use the
  **`release` skill** (`.claude/skills/release/`); don't hand-write a release body,
  it gets overwritten on the next run.
- Dependencies are Renovate's job. `renovate.json` carries version holds with the
  reason and the condition for removing each one; read it before pinning something by
  hand.

## Traps

- **Tailwind 4 utilities are unlayered on purpose.** `frontend/src/styles/main.css`
  composes the Tailwind entry by hand instead of `@import "tailwindcss"`, because
  layered utilities lose to unlayered vendor CSS (video.js, fullcalendar) regardless of
  specificity or order. Restoring the plain import silently breaks layout wherever a
  utility meets a vendor stylesheet. See the `local-testing` skill.
- **`go run cmd/tumlive/main.go` leaves a process `pkill -f` won't match** — it runs a
  compiled binary from a temp path. Kill it by port.
- The `ldap:` block in `config.yaml` stays commented out in development: with
  `useForLogin`, every rejected password falls back to LDAP and hangs if the host
  doesn't resolve.
- Meilisearch connection errors on boot are expected locally; only search is affected.
- `web/spa/` holds a tracked marker so `go:embed` resolves on a fresh checkout, which
  is why the Vite build deliberately does **not** empty it; the `prebuild` script
  clears stale hashed assets instead.
