import { expect, test } from "@playwright/test";

import { dynamicInfoPage, infoPages, type InfoPageKey } from "./seed";

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

      // The full header nav, not the logo-only chrome these pages had before, which
      // made it hard to navigate away. Empty rather than a Login link: these routes
      // are anonymous, so App.vue never calls auth.load() on them.
      await expect(page.locator("#logo")).toBeVisible();
      await expect(page.locator("#user-context")).toHaveCount(1);
      await expect(page.locator("#user-context")).toBeEmpty();
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

  test("the list endpoint names every page, including one added after the built-in three", async ({
    request,
  }) => {
    const response = await request.get("/api/v2/info-pages");
    expect(response.status()).toBe(200);

    const body = (await response.json()) as { pages: { slug: string; name: string }[] };
    const slugs = body.pages.map((page) => page.slug);
    expect(slugs).toEqual(expect.arrayContaining([...names, dynamicInfoPage.slug]));
    // Public, so it must not leak unrendered content ahead of getInfoPage.
    expect(body.pages[0]).not.toHaveProperty("rawContent");
  });

  test("a page added after the built-in three is reachable at its slug with no route of its own", async ({
    page,
  }) => {
    await page.goto(`/${dynamicInfoPage.slug}`);

    await expect(
      page.getByRole("heading", { name: dynamicInfoPage.heading, level: 1 }),
    ).toBeVisible();
  });

  test("a path that is not a real info page still falls through to Go's own 404", async ({
    page,
  }) => {
    // web/course.go's shortLinkOrInfoPage checks the same table before falling back to
    // a course short link, so an unknown single-segment path is not swallowed by the SPA.
    await page.goto("/this-page-does-not-exist");

    await expect(page.getByText("This page does not exist.")).toBeVisible();
  });
});
