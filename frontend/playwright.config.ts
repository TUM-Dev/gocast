import { defineConfig, devices } from "@playwright/test";

/**
 * End-to-end configuration. Run with `make test_e2e`, which reloads the fixture in
 * tum-live-starter.sql (see e2e/seed.ts) before the server below starts.
 *
 * E2E_BASE_URL points at a server that is already up and suppresses the one started
 * here. Whatever it names has to hold the same fixture.
 */

export const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:8081";

export default defineConfig({
  testDir: "./e2e",
  // One shared database account, so parallel workers would fight over its settings.
  workers: 1,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],

  /*
   * `reuseExistingServer` is off on purpose: reusing one would reuse a server that
   * booted before the fixture was reloaded, which is what this exists to prevent.
   * The timeout is generous because a cold `go run` compiles first.
   */
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        command: "go run cmd/tumlive/main.go",
        cwd: "..",
        url: `${baseURL}/api/v2/status`,
        reuseExistingServer: false,
        timeout: 180_000,
        // The access log is one line per request and buries the results.
        stdout: "ignore",
        stderr: "pipe",
      },
});
