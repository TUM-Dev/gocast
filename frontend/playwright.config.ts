import { defineConfig, devices } from "@playwright/test";

/**
 * End-to-end configuration.
 *
 * These drive a real browser against a running server, covering what no unit test
 * reaches: gin choosing between the SPA shell and a template, the session cookie
 * surviving login and redirects, and the bearer token minted from it.
 *
 * They run against the database in tum-live-starter.sql and nothing else: the users,
 * courses and lectures they assert on are the ones that dump defines, described in
 * e2e/seed.ts. Load it, then start the server, which migrates it forward:
 *
 *   make e2e_db
 *   go run cmd/tumlive/main.go
 *   cd frontend && npm run test:e2e
 *
 * E2E_BASE_URL points them elsewhere, but only at a deployment holding the same
 * fixture — the visibility assertions name particular courses.
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
});
