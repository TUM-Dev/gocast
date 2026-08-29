# GoCast frontend

The Vite single-page app that is gradually replacing the server-rendered templates in
`web/template`. Both frontends run side by side: `web/router.go` decides per route
which one answers, so pages move over one at a time and can be moved back just as
easily.

Currently migrated: **`/settings`**, **`/login`**, and the start page — `/`,
`/courses/mine`, `/courses/public` and `/course/:year/:term/:slug`.

Two things the start page had and this one does not, both waiting on the v1 API:

- **The viewer count on live cards.** The number lives in the chat session map that
  `api/chat.go` keeps in memory, which `GetLiveCourses` has no way to reach.
- **The search typeahead.** `AppHeader` submits to `/search` instead, because the
  Meilisearch endpoints behind the dropdown exist only on v1.

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
make e2e_db                      # load tum-live-starter.sql, dropping what was there
go run cmd/tumlive/main.go       # in another terminal
cd frontend && npm run test:e2e  # or `make test_e2e` from the repo root
```

Playwright, against a running server — deliberately not part of `npm test`, which must
stay runnable with nothing else up. These cover what the unit tests structurally
cannot: which frontend answers a given path, the session cookie surviving login and
redirects, the bearer token minted from that cookie, and what each kind of caller is
actually shown.

**The fixture is `tum-live-starter.sql` and nothing else.** The suite creates no
accounts and inserts no rows; it signs in as the six users that dump defines, all of
which have the password `password`. `e2e/seed.ts` restates the dump — who administers
what, who is enrolled in what, which lecture is live — so the expectations can be
checked against the SQL by eye, and every spec reads its expectations from there rather
than repeating course names.

The dump carries a fixture section at the end, added so that every rule the start page
applies has a case: a course of each of the four visibilities, a private lecture, a
live lecture in a hidden course, watch progress, a pin on a course its owner may no
longer see, scheduled lectures, and a server notification of each kind.

**Some of its lectures are dated relative to when it is loaded**, because "today" and
"starting in half an hour" cannot be written as fixed dates. Reload with `make e2e_db`
before a run; lectures left over from a load days ago have gone stale, and the tests
that depend on them will fail. The one for "today" is dated late in the evening so it
stays ahead of the clock — after 23:45 that test skips itself rather than failing.

- **`login.spec.ts`** — the round trip from a protected page to `/login` and back, a
  rejected password, one session spanning a migrated and a legacy page, and that
  credentials never reach the API.
- **`settings.spec.ts`** — that a saved setting is read back after a reload, and that
  the greeting is stored as the bytes its Go getter reads. It once had to look at the
  server-rendered start page for that, because a client that encodes on write and
  decodes on read hides the fault from itself; that page is the SPA now, so it asserts
  the stored value over the wire instead.
- **`migration.spec.ts`** — the shell served for migrated paths and not for others, and
  the cookie-to-bearer bridge.
- **`start-page.spec.ts`** — that the page fills for a signed-in caller *and* for an
  anonymous one, and that the old `?view=` URLs still land somewhere. The anonymous
  case is there because it once broke outright: the page loads the semester list before
  anything else, and asking for that with a bearer token failed for a visitor with no
  session, leaving the whole page blank. Every unit test still passed.
- **`visibility.spec.ts`** — the matrix: for each of the six users and for an anonymous
  visitor, which courses are listed, which are theirs, which they may open by URL,
  which live lectures they are shown, which lectures a course page lists, whether a
  private one is among them, and who is offered the admin link. Asserted against the
  rendered page, because a listing the server filters correctly and the page then
  renders from the wrong array is exactly as wrong — and neither the Go tests nor the
  component tests would notice.

  The case worth knowing is `hidden`, where being listed and being reachable come
  apart: the hidden course is in nobody's public listing, its live lecture reaches only
  the lecturer administering it — not even the student enrolled in it — and yet a
  direct link to the course opens for anyone, including a visitor who is not signed in.
  That is the difference between unlisted and private, and it is easy to get wrong in
  either direction.

Two things to know before adding to them:

- **The accounts outlive the run.** `settings.spec.ts` writes to `studi2`, and changes
  each setting away from whatever is currently stored rather than to a fixed value; a
  test written against a fixed starting value passes once and then fails on the state
  its predecessor left behind. `make e2e_db` puts the fixture back. For the same reason
  the suite runs with one worker.
- **The visibility tests only read**, so they neither depend on nor disturb that. Add
  new expectations to `e2e/seed.ts`, not to the spec.
- **Add cases to the dump, not to the tests.** A rule with no data behind it cannot be
  covered here; the fixture section at the end of `tum-live-starter.sql` is where a new
  one goes, with its expectation in `e2e/seed.ts`.
- **A rejected password is slow when LDAP is configured.** The server falls back to
  LDAP before giving up, so `login.spec.ts` takes as long as that server takes to
  answer — and hangs until the test times out if the configured host does not exist.

The build is embedded into the binary, so a change here reaches the browser only after
`npm run build` **and** a server restart. `npm run dev` avoids both.

The remaining gap is SAML, which needs an identity provider this suite has no way to
stand up.

## Start page URLs

The server-rendered start page was one path with a `view` query parameter — `/?year=&
term=&slug=&view=3` — driven by an Alpine state machine. Those URLs are in bookmarks
and in browser history, so `legacyStartPageRedirect` in `src/router/index.ts`
translates each of them into the route that replaced it.

The replacements keep the shapes the server already had. `/course/:year/:term/:slug`
and `/semester/:year/:term` are the targets `web/router.go` redirects to today, so
links to them keep working unchanged; everything else carries the semester as
`?year=&term=`, absent meaning whichever semester is current.

There is deliberately no `/course/:slug` for the current semester, although the API
would support it: gin panics when two parameters with different names share a
position, so it could never be registered beside `/course/:year/:term/:slug`.

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
   it belongs in the API. A hook may also answer the request itself by calling
   `c.Abort()`, which is how `/` still renders the onboarding page on a deployment
   that has no users yet.

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

## The start page's shell

Everything the start page renders lives in `src/components/start-page/`; the six
components left directly in `src/components/` are the application shell, reachable from
every page. Nothing in the shell imports from the folder, which is what keeps the split
meaningful rather than decorative.

`start-page/StartPageLayout.vue` is the frame the four routes share: the sidebar, and
the column beside it that each view fills through a slot. `start-page/StartPageSidenav.vue`
is the sidebar itself, ported from `home.gohtml` with its classes copied verbatim.

Two pieces of shared state sit behind it:

- **`stores/courses.ts`** holds the three listings for one semester at a time. The
  sidebar and the view beside it show the same listings, and the server-rendered page
  fetched each of them twice for exactly that reason. Only the semester being looked at
  is kept — caching every semester a user visits grows without bound for the sake of a
  back button that reloads in well under a second. `togglePin` tells the server before
  it touches any listing, so a pin that failed to save is never left looking saved.
- **`stores/sidenav.ts`** is whether the sidebar is showing on a narrow screen. It is
  shared because the button that opens it is in the header and the sidebar is in the
  page — the same split the `navigation` toggle in `web/ts/views/home.ts` had.

The layout waits for `getSemesters` before loading any listing. Which semester "no
semester in the URL" means is the server's answer, and fetching for `undefined` and
then again for the resolved semester would double every request on a first visit.

## What a start page load costs

Around a dozen API calls, which matters less than how deep they stack — they share one
HTTP/2 connection and add up to some 15ms of server time. What a visitor waits for is
the number of *rounds*:

1. `/config`, `/semesters`, `/server-notifications` and the token mint, all at once
2. `/users/me` alongside the four course listings
3. `/progress` and the thumbnails, which need stream ids from the listings

Three rounds, about 220ms at 50ms of latency. Two things keep it there, and both are
easy to undo by accident:

- **The course store asks `hasSession()`, not the auth store.** It only needs to know
  whether to call the endpoints that require a user, and minting a token answers that;
  waiting for `/users/me` put all four listings a round behind it for the same answer.
- **`fetchConfig` is shared and cached.** The footer mounts twice on the start page,
  once per layout, and both ask.

Anything new that mounts on this page and fetches should join round 1 or 2 rather than
adding a fourth.

## Three ways to call the API

Which request helper in `src/lib/api.ts` a module uses follows from the endpoint's
policy in `apiv2/server/services.go`, and getting it wrong fails quietly:

- **`apiGetMessage`** — for `authenticated` endpoints (`/courses/enrolled`,
  `/courses/pinned`, `/progress`). Mints a bearer token and refreshes it once on a 401.
- **`apiGetMessagePublic`** — for `public` endpoints reached *before* login
  (`/login-options`). Sends no token, because minting one from a session that does not
  exist fails for the wrong reason.
- **`apiGetMessageOptionalAuth`** — for `public` endpoints that show a signed-in caller
  **more**: `/courses` gains the logged-in-only courses, and `/courses/live`,
  `/courses/{slug}` and the thumbnails gain whatever the caller administers. Sends the
  token when a session can produce one and falls back to an anonymous request when it
  cannot. Neither of the other two is safe here — the first fails for a visitor who is
  simply not signed in, and the second shows a signed-in user the logged-out view of
  the site.

The "no session" answer is remembered for the life of the page, so an anonymous visitor
makes one doomed token request rather than one per listing.

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
