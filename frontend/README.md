# GoCast frontend

The Vite single-page app that is gradually replacing the server-rendered templates in
`web/template`. Both frontends run side by side: `web/router.go` decides per route
which one answers, so pages move over one at a time and can be moved back just as
easily.

Currently migrated: **`/settings`**, **`/login`**.

## Running it

```sh
# terminal 1 — the Go server
go run cmd/tumlive/main.go

# terminal 2 — this app, with hot reloading
cd frontend && npm install && npm run dev
```

Then open **http://localhost:5173**, not the Go server's port. Everything the SPA does
not own — the API, static assets, login, and the pages still rendered from templates —
is proxied through to Go by `vite.config.ts`. The proxy leaves the request host alone
so the session cookie survives the hop and you stay logged in across both frontends.

To check the production path instead, build and let Go serve it:

```sh
npm run build                      # writes web/spa/
go run cmd/tumlive/main.go         # /settings now serves the built app
```

## Authentication

The API is called with `Authorization: Bearer`. The token lives in memory only —
never `localStorage`, which any XSS on the origin can read — and is obtained from
`POST /api/v2/auth/token`, which trades the browser's `HttpOnly` session cookie for a
short-lived token. That bridge is what lets a user who signed in on a server-rendered
page use a migrated page without signing in again, and vice versa.

`src/lib/api.ts` attaches the token, refreshes it once on a 401, and shares a single
in-flight refresh between concurrent requests. Callers never see any of that.

## The login page

The page is rendered here, but **credentials are still posted to Go**. The form is a
real `method="post" action="/login"` submission, so the browser handles it and the
server sets the session cookie and honours the stored redirect exactly as before.
Posting through the API instead would mean reimplementing session creation, the LDAP
fallback and the redirect — for no gain.

Three consequences worth knowing:

- **`/login` has a server-side hook.** `spaRouteHooks` in `web/router.go` runs
  `SetLoginRedirectCookie` before serving the shell, because where to return to has to
  be recorded before the user leaves for an external identity provider, which comes
  back without the original query string.
- **A failed attempt redirects to `/login?error`** rather than rendering the template
  inline, which would otherwise replace the SPA mid-flow. The server-rendered page
  reads the same parameter, so the legacy fallback still shows the error.
- **SAML is a full navigation** to `/saml/out`, never a fetch.

`getLoginOptions` reports whether single sign-on is configured and what to call it, so
the page knows which controls to render before anyone has authenticated. It and the
password reset call go through `apiFetchPublic`, which sends no bearer token — using
the authenticated path would try to mint a token from a session that does not exist.

## Tests

```sh
npm test            # once
npm run test:watch  # while working
```

Vitest, with happy-dom. The suite deliberately concentrates on the places where a
mistake is silent rather than loud:

- **`lib/api.test.ts`** — the credential flow: one token for concurrent callers, a
  single refresh-and-retry on 401, and no retry for anything else. Only `fetch` is
  stubbed, so the real client runs.
- **`lib/settings.test.ts`** — that each setting is written in the format its Go getter
  reads. A name encoded as JSON once shipped literal quotes into every page that
  displayed it; these assertions describe the actual bytes on the wire, generated
  schemas and all.
- **`lib/notifications.test.ts`** — read state surviving a refetch, including the known
  cost of keying on content rather than an id.
- **`views/LoginView.test.ts`** — that credentials go to Go as a form post, and which
  controls appear for each login configuration.

- **`stores/auth.test.ts`** — that the shell and the view it renders share one request
  for the current user rather than issuing one each.

`src/test/setup.ts` installs an in-memory `localStorage`: Node ships a built-in one
that is inert without `--localstorage-file` and shadows the DOM environment's.

## Browser tests

```sh
go run ./frontend/e2e/seeduser   # once, and again after the database is reset
go run cmd/tumlive/main.go       # in another terminal
cd frontend && npm run test:e2e  # or `make test_e2e` from the repo root
```

Playwright, against a running server — deliberately not part of `npm test`, which must
stay runnable with nothing else up. These cover what the unit tests structurally
cannot: which frontend answers a given path, the session cookie surviving login and
redirects, and the bearer token minted from that cookie.

- **`login.spec.ts`** — the round trip from a protected page to `/login` and back, a
  rejected password, one session spanning a migrated and a legacy page, and that
  credentials never reach the API.
- **`settings.spec.ts`** — that a saved setting is read back by this page after a
  reload *and* by the server-rendered start page, which is the only place the encoding
  regression was ever visible.
- **`migration.spec.ts`** — the shell served for migrated paths and not for others, and
  the cookie-to-bearer bridge.

Two things to know before adding to them:

- **The account outlives the run.** Every test changes a setting away from whatever is
  currently stored rather than to a fixed value; a test written against a fixed
  starting value passes once and then fails on the state its predecessor left behind.
  For the same reason the suite runs with one worker.
- **A rejected password is slow when LDAP is configured.** The server falls back to
  LDAP before giving up, so `login.spec.ts` takes as long as that server takes to
  answer — and hangs until the test times out if the configured host does not exist.

The build is embedded into the binary, so a change here reaches the browser only after
`npm run build` **and** a server restart. `npm run dev` avoids both.

The remaining gap is SAML, which needs an identity provider this suite has no way to
stand up.

## Migrating another page

A migration finishes by **deleting the page it replaces**, in the same pull request.
Running the two versions side by side buys less than it looks like: the build is
embedded in the binary, so moving a page back is a code change and a deploy either way,
and the browser tests are what actually establish that the new page works. Keeping the
old one around instead just leaves two implementations to maintain.

**Build it**

1. Build the page here and add its path to the router in `src/router/index.ts`.
2. Add the same path to `spaRoutes` in `web/router.go`.
3. Keep the route's existing auth middleware. The server still gates the page; the
   client only decides what to offer.
4. If the page needs something server-side that a client cannot do — reading or writing
   a cookie it depends on — add a `spaRouteHooks` entry. Data fetching is not that:
   it belongs in the API.

**Prove it**

5. Walk the old page and list what it does, including the parts nobody uses often:
   every control, every role that sees something different, every error state.
6. Cover the ones that fail *silently* in `e2e/`. Anything that writes needs a test
   that reads the value back — settings whose Go getter returns the stored string
   verbatim have to be written unencoded, and the only place that shows is a page
   outside the SPA.

**Delete it**

7. Remove the template handler and pass `nil` to `registerPage` for the path.
8. Delete the `.gohtml`. Its partials are resolved at render time, not at boot, so a
   partial another page still includes fails only when someone loads that page —
   `grep` each one before removing it. This is the one deletion the compiler cannot
   check for you.
9. Delete the DAO methods and v1 endpoints the page used. The compiler catches a DAO
   method that still has callers; it says nothing about an HTTP route, so check the
   legacy TypeScript in `web/ts` for anything still calling the endpoint.
10. Regenerate mocks if a DAO interface changed: `make mocks`.

**Rolling back** is `git revert` of that pull request. Taking the path out of
`spaRoutes` on its own no longer works once the template is gone, and the server says
so at boot rather than serving a 404 — as it does when the frontend build is missing
for a page that has no template left. `web/spa_test.go` covers both.

Pages where parity cannot be established cheaply — the watch page, course
administration — are the reason `registerPage` still takes a legacy handler at all.
Serve both for those, and keep the entry in `spaRoutes` as the switch.

## Styling

Tailwind, with the theme copied verbatim from `web/tailwind.config.js` so that
migrated and legacy pages resolve identical colours. `src/styles/main.css` carries
over the `tum-live-*` component classes and the `text-1`…`text-9` contrast ladder the
templates use. Keep both in sync until the legacy config is deleted; each page
migration brings its own classes across rather than porting the whole stylesheet up
front.

The inline script in `index.html` applies the theme before first paint, sharing the
`themeMode` localStorage key with `web/assets/init.js` so a choice made on a legacy
page is honoured here and vice versa.

Font Awesome is loaded from `/static/node_modules`, exactly as the templates load it —
same file, same icons, no bundle weight. It moves into the SPA's own assets when the
`node_modules` embed in `web/router.go` goes away.

## The header

`AppHeader.vue` is the shared header for every SPA page, ported from the header in
`web/template/home.gohtml` with its classes copied verbatim so migrated and legacy
pages line up. It carries the logo, global search, the notification bell and the user
menu (admin link for admins and lecturers, theme picker, settings, keyboard shortcuts,
feedback links, logout).

Two deliberate gaps, both waiting on the start page:

- **Search** submits to `/search` rather than showing the live typeahead. The dropdown
  needs the Meilisearch endpoints, which exist only on the v1 API, and is course-context
  aware in a way that belongs with the course page.
- **The mobile sidenav toggle** is behind the `showSidenavToggle` prop, off by default,
  because the navigation it opens is part of the start page. Pass the prop and handle
  `@toggle-sidenav` when that lands.

Notification read state is client-side, kept in localStorage under the same key the
legacy code uses. It keys on a hash of the notification's content because
`protobuf.UserGroupNotification` has no id field; adding one to the proto would make
that exact rather than merely stable.

## The generated API client

Message types live in `src/gen`, generated from `apiv2/server/apiv2.proto` by
`protoc-gen-es`:

```sh
npm run proto     # or `make proto_es` from the repo root
```

The output is **committed**, so building the app needs neither `buf` nor the Go
toolchain — only regenerating does. Run it whenever the proto changes, alongside
`apiv2/generate.sh`, which regenerates the Go server from the same file.

grpc-gateway marshals with protojson and `@bufbuild/protobuf` implements protojson, so
the generated schemas decide how enums, timestamps and 64-bit integers are read rather
than the client guessing. `apiGetMessage` and `apiPatchMessage` in `src/lib/api.ts`
wrap that; they ignore unknown fields, matching the gateway's own `DiscardUnknown`, so
a field added server-side does not break a deployed client.

Two things the generator deliberately does not cover:

- **Paths.** `protoc-gen-es` generates messages, not routes, so the URL and method are
  written by hand from the `google.api.http` annotations. Connect would generate those
  too, but it speaks its own protocol rather than the gateway's REST mapping — adding
  it means registering Connect handlers in Go beside the gateway.
- **Errors.** Gateway failures are `{code, message, details}`, not proto messages.
  `ApiError` in `src/lib/api.ts` handles them.

Endpoints returning `google.api.HttpBody` (subtitles, thumbnails) bypass JSON
entirely — fetch those raw rather than through a generated schema.
