import { fileURLToPath } from "node:url";

import { defineConfig, devices } from "@playwright/test";

/**
 * End-to-end configuration. Run with `make test_e2e`, which reloads the fixture in
 * tum-live-starter.sql (see e2e/seed.ts) before the server below starts.
 *
 * E2E_BASE_URL points at a server that is already up and suppresses the one started
 * here. Whatever it names has to hold the same fixture.
 */

export const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:8081";

/**
 * Where the `setup` project leaves each seeded account's session for the run. Written
 * fresh every run and ignored by git; see e2e/auth.setup.ts.
 */
export const authStateDir = fileURLToPath(new URL("./e2e/.auth/", import.meta.url));

export default defineConfig({
  testDir: "./e2e",
  /*
   * Files run in parallel, the tests within a file in order on one worker. The
   * default worker count is half the machine's cores; CI gets whatever the runner has.
   *
   * The files share one database, so this is only sound because of how they write:
   *
   * - The destructive tests consume rows seeded for them alone (`runner-beta`,
   *   `worker-beta`, the `consumed` failures in maintenance.spec.ts), or rows they
   *   created themselves (accounts, courses in semester 1234, halls, tokens, info
   *   pages, notifications).
   * - Tests that change a seeded row put it back in a `finally`, and only touch rows
   *   no other file reads while they do: course-admin-api.spec.ts flips course 3's
   *   settings, course-settings.spec.ts course 1's; lecture-admin-api.spec.ts renames
   *   stream 4 ("VL 3: Rückblick", listed nowhere), course-lectures.spec.ts "VL 7:
   *   Malz" and schedule.spec.ts "VL 2: Verkostung", each read by name only in its
   *   own file.
   * - settings.spec.ts flips studi2's settings relative to whatever is stored; nothing
   *   else reads them.
   * - The audit log is the exception, since every write above appends to it:
   *   audits.spec.ts runs in a project of its own, before the rest (see `projects`).
   *
   * A new test that changes something another file asserts on has to follow one of
   * those, or it will fail the other file at random.
   */
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL,
    // Recording a trace costs every test a little; keeping one only for a failed
    // retry costs the green path nothing and still leaves a trace behind a failure
    // that repeats, which is the kind worth debugging from CI.
    trace: process.env.CI ? "on-first-retry" : "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "setup", testMatch: /auth\.setup\.ts/ },
    // The audits page asserts on the log as a whole -- which row comes first, how many
    // pages there are -- and every file that writes appends to that log. So it reads
    // the log alone, before anything else has written to it. Only this file: the
    // ordering costs a few seconds of serial time, and a failure here keeps the
    // dependent project from running at all, which is what a dependency means. A run
    // narrowed to one file runs this project in full as well; it is a few seconds.
    {
      name: "audits",
      testMatch: /audits\.spec\.ts/,
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["setup"],
    },
    {
      name: "chromium",
      testIgnore: /audits\.spec\.ts/,
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["audits"],
    },
  ],

  /*
   * `reuseExistingServer` is off on purpose: reusing one would reuse a server that
   * booted before the fixture was reloaded, which is what this exists to prevent.
   * The timeout is generous because a cold `go run` compiles first.
   */
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        // E2E_SERVER_CMD names a prebuilt binary; `make test_e2e_cover` points it at
        // one built with -cover, and the CI workflow at the one it just built.
        command: process.env.E2E_SERVER_CMD ?? "go run cmd/tumlive/main.go",
        cwd: "..",
        url: `${baseURL}/api/v2/status`,
        reuseExistingServer: false,
        timeout: 300_000,
        // Not piped even on CI: one access-log line per request through the reporter
        // slowed the suite enough to fail it. stderr still carries panics and errors.
        stdout: "ignore",
        stderr: "pipe",
        // A -cover binary writes its counters as it exits, so it has to be asked to
        // rather than killed: the default SIGKILL leaves GOCOVERDIR empty.
        gracefulShutdown: { signal: "SIGTERM", timeout: 30_000 },
      },
});
