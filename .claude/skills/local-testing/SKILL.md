---
name: local-testing
description: Run GoCast's checks and bring the stack up locally to verify a change in a real browser — Go tests, frontend unit tests, lint, Playwright e2e, and screenshot-based visual regression against a baseline branch. Use when asked to test, run, or visually verify GoCast, when a change touches CSS or the Tailwind config, or when a dependency bump needs proving beyond CI.
---

# Testing GoCast locally

CI covers compile-and-assert. It cannot see layout, colour, or anything that only
appears once the app is in a browser — which is where the expensive regressions
live. This skill covers both halves.

## The fast checks

Run these first; they need no database and catch most things.

```bash
go test -race ./...                  # or: make test  (also runs frontend unit tests)
pnpm --dir frontend test           # 166 vitest tests, ~2s
pnpm --dir frontend run typecheck  # vue-tsc
make lint                            # golangci-lint + typecheck
```

## Bringing up the stack

Four steps, in order. Step 2 is not optional.

**1. Database.** The starter dump is the fixture every browser test asserts on.

```bash
docker run --detach --name mariadb-tumlive \
  --env MARIADB_USER=root --env MARIADB_ROOT_PASSWORD=example \
  -e TZ="$(cat /etc/timezone)" -p 3306:3306 \
  --volume "$PWD"/tum-live-starter.sql:/init.sql \
  mariadb:latest --init-file /init.sql

make e2e_db DB_CONTAINER=mariadb-tumlive   # reseed; rerun after anything mutates settings
```

`make e2e_db` pins the seeding session to the host's UTC offset. The dump dates its
lectures off `NOW()`, the server reads them back as local time, and the browser judges
"today" in its own — all three have to agree or the "today" fixtures land on the wrong
day. Don't seed by piping the dump in directly.

**2. SPA build.** The server panics at startup without `web/spa/index.html`, and a stale
build means you are testing yesterday's frontend.

```bash
pnpm --dir frontend install --frozen-lockfile && pnpm --dir frontend run build
```

**3. Server.** Config comes from `./config.yaml` at the repo root.

```bash
go run cmd/tumlive/main.go > server.log 2>&1 &
until curl -fsS -o /dev/null http://localhost:8081/login; do sleep 2; done
```

First boot compiles and migrates — allow a minute. Meilisearch `connection refused`
warnings are expected and harmless; search just won't work. Leave the `ldap:` block in
`config.yaml` commented out: with `useForLogin` set, every rejected password falls back
to LDAP and hangs if the host doesn't resolve.

**4. Log in.** Accounts are `admin`, `prof1`, `prof2`, `studi1`, `studi2`, `studi3`,
all with password `password`. The login field is the users table's `email` column, not
`name`. Roles: 1 admin, 2 lecturer, 4 student.

Useful URLs: `/`, `/course/2022/S/brauereiwesen`, `/w/brauereiwesen/2`, `/admin`,
`/admin/courses/1/lectures`. Courses in the fixture are
`brauereiwesen`, `games101`, `godev` (2021/W), `bierkunde`, `geheim`.

**There is no `/schedule` route** — `/admin` *is* the schedule page. `/schedule`
renders the SPA's 404 page.

## Driving it in a browser

Three scripts in `scripts/`. They find the repo root themselves, so run them from
anywhere in the tree.

```bash
node .claude/skills/local-testing/scripts/capture.mjs <label> [path...]
node .claude/skills/local-testing/scripts/compare.mjs <baseline> <candidate>
node .claude/skills/local-testing/scripts/matched-rules.mjs <path> <selector> <property>
```

`capture.mjs` logs in, screenshots 14 pages in light and dark into `.shots/`, and
reports two things a screenshot alone won't tell you: horizontal overflow
(`scrollWidth > clientWidth`) and **error pages**, which look like any other page in a
screenshot. Trust no diff from a run that reported either.

Playwright is a dependency of `frontend/` and the package is `@playwright/test` — not
`playwright`, which fails with `ERR_MODULE_NOT_FOUND` even though the browsers are
installed. `lib.mjs` resolves it by path for scripts living outside `frontend/`.

## Visual regression against a baseline

This is the part CI cannot do, and it is how every bug below was found. Comparing a
change against *itself* proves nothing; compare it against `dev`.

```bash
git switch dev && pnpm --dir frontend install --frozen-lockfile \
  && pnpm --dir frontend run build
# restart the server, then:
node .claude/skills/local-testing/scripts/capture.mjs base

git switch -                                        # back to your branch, rebuild, restart
node .claude/skills/local-testing/scripts/capture.mjs mine
node .claude/skills/local-testing/scripts/compare.mjs base mine
```

Read the output like this:

- **SIZE MISMATCH** — the layout moved. Always a regression; start here.
- **> ~1%** — something structural. Open both images.
- **< ~1% spread evenly** — usually palette or font rounding. Sample a few pixels
  before dismissing it, and check whether the project's *own* configured colours
  moved: if they did, it is not just rounding.

Restart the Go server after rebuilding: the build is embedded in the binary.

## Debugging a layout difference

When an element is the wrong size and the utility class controlling it looks right,
`matched-rules.mjs` prints every rule setting that property **with its cascade layer**:

```
$ node .claude/skills/local-testing/scripts/matched-rules.mjs /w/brauereiwesen/2 video-js.video-js width
  .video-js                width:300px    layer=(none)
  .video-comb-dimensions   width:1920px   layer=(none)
  .w-full                  width:100%     layer=(none)
computed: 1438px
```

The `layer` column answers more questions than specificity does.

## Things CI cannot catch, learned the hard way

These are real regressions that shipped green through every job.

**Cascade layers beat specificity and order.** `@import "tailwindcss"` puts every
utility in `@layer utilities`, and a layered rule loses to an unlayered one no matter
what. Every vendor stylesheet this app links — video.js, silvermine-airplay,
fullcalendar — is unlayered, so *any*
utility on an element they also style loses. It surfaced as video.js sizing its player
1920px wide over `w-full`, but the quiet cases are everywhere the two meet. Hence the
hand-composed entry in `frontend/src/styles/main.css`: theme and preflight layered,
utilities not. **If you ever restore a plain `@import "tailwindcss"`, re-run the visual
diff** — this breakage is invisible to lint, tests and typecheck.

**A class that does nothing is not a class that does nothing forever.**
`min-w-screen` was never a Tailwind 3 utility, so four admin pages rendered as
though it were absent for years. Tailwind 4 defines it, and those pages jumped 320px
past the viewport. When a major version lands, the risk is not only renamed classes —
it is dead ones coming alive. Grep the views for classes that produce no CSS in the
*old* version.

**Default tokens move between majors.** Tailwind 4's default sans stack drops
`ui-sans-serif` and `system-ui`, which on Linux picks a different face and reflows
text everywhere. The v3 stack is pinned in `frontend/tailwind.config.js` with a
comment; deleting it is a deliberate visual decision, not a cleanup.

**Utility scales get reused at new values.** v4 kept the names `shadow-sm`,
`rounded-sm`, `blur-sm` and `outline-none` but changed what they mean. Verify by
diffing compiled CSS between versions rather than trusting a changelog — and note
that `@tailwindcss/upgrade` skips these codemods if the new version is already
installed, which is exactly when you would be running it.

## Playwright e2e

```bash
make e2e_db DB_CONTAINER=mariadb-tumlive   # and again whenever a run leaves settings changed
make run                                    # in another terminal
make test_e2e
```

Not part of `make test`: these need the server and its database up.

## Shell gotchas

- `pkill -f cmd/tumlive/main.go` does **not** stop the server. `go run` executes a
  compiled binary elsewhere; kill it by port:
  `kill "$(ss -lptn 'sport = :8081' | grep -oP 'pid=\K[0-9]+' | head -1)"`
- Under zsh, `kill -0 "$(cat server.pid)"` in a readiness loop floods
  `Too many arguments`. Use an `until curl ...; do sleep 2; done` loop instead.
- `.shots/` is scratch output; keep it out of commits.
