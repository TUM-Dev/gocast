import { defineConfig, devices } from "@playwright/test";

/**
 * End-to-end configuration.
 *
 * These drive a real browser against a running server, covering what no unit test
 * reaches: gin choosing between the SPA shell and a template, the session cookie
 * surviving login and redirects, and the bearer token minted from it.
 *
 * The server needs a database, so bring it up and seed the account first:
 *
 *   go run ./frontend/e2e/seeduser
 *   go run cmd/tumlive/main.go
 *   cd frontend && npm run test:e2e
 *
 * Point E2E_BASE_URL elsewhere to run against a deployed instance.
 */

export const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:8081";
export const username = process.env.E2E_USERNAME ?? "e2e@localhost";
export const password = process.env.E2E_PASSWORD ?? "e2e-password";

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
