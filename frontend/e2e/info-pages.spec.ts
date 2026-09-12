import { expect, test } from "@playwright/test";

import { infoPages, type InfoPageKey } from "./seed";

/**
 * Covers what a stubbed component test cannot: that the server really renders and
 * sanitises the Markdown. Run anonymously, as these pages are read before signing in.
 */

const names = Object.keys(infoPages) as InfoPageKey[];

test.describe("info pages", () => {
  for (const name of names) {
    test(`/${name} renders for a visitor who is not signed in`, async ({ page }) => {
      await page.goto(`/${name}`);

      // The Markdown's own heading, so the content arrived and rendered.
      await expect(
        page.getByRole("heading", { name: infoPages[name].heading, level: 1 }),
      ).toBeVisible();

      // The logo-only header the server-rendered page had: no search, no user menu.
      await expect(page.locator("#logo")).toBeVisible();
      await expect(page.locator("#user-context")).toHaveCount(0);
    });
  }

  test("the content is sanitised before it reaches the page", async ({ request }) => {
    // The fixture carries a script tag and an inline handler; v-html means both must
    // be gone before the client sees them.
    const response = await request.get("/api/v2/info-pages/privacy");
    expect(response.status()).toBe(200);

    const body = (await response.json()) as { name: string; content: string };
    expect(body.name).toBe("privacy");
    expect(body.content).toContain("<h1>");
    expect(body.content).not.toContain("<script");
    expect(body.content).not.toContain("onerror");
  });

  test("is keyed on the route, not on the editable title", async ({ request }) => {
    // The fixture's titles differ from their routes, so a rename cannot move a page.
    const response = await request.get("/api/v2/info-pages/privacy");
    const body = (await response.json()) as { content: string };
    expect(body.content).not.toContain(infoPages.privacy.title);
  });

  test("an unknown page is refused rather than looked up", async ({ request }) => {
    const response = await request.get("/api/v2/info-pages/terms");
    expect(response.status()).toBe(404);
  });

  test("needs no token", async ({ request }) => {
    const response = await request.get("/api/v2/info-pages/about");
    expect(response.status()).toBe(200);
  });
});
