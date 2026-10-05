import { expect, type Page } from "@playwright/test";

import { login } from "./helpers";
import { semesters, users, type UserKey } from "./seed";

/**
 * Shared by visibility-start-page.spec.ts and visibility-course-page.spec.ts, the two
 * halves of the visibility matrix. They are two files rather than one so they run on
 * two workers: as one file they were the suite's critical path, half its wall time.
 */

/** A sidebar group, addressed by its heading. */
export function sidebarGroup(page: Page, heading: string) {
  return page
    .locator("#side-navigation article")
    .filter({ has: page.locator("header", { hasText: heading }) });
}

/** The course slugs a set of course links points at, in the order they are rendered. */
export async function linkedSlugs(scope: { locator: Page["locator"] }): Promise<string[]> {
  const links = scope.locator('a[href^="/course/"]');
  const hrefs = await links.evaluateAll((nodes) =>
    nodes.map((node) => (node as HTMLAnchorElement).getAttribute("href") ?? ""),
  );
  return hrefs.map((href) => href.split("/").pop() ?? "");
}

export async function expectSlugs(scope: { locator: Page["locator"] }, want: readonly string[]) {
  expect(new Set(await linkedSlugs(scope))).toEqual(new Set(want));
}

/** Opens the start page for a semester, signed in as the given user or as nobody. */
export async function startPage(page: Page, user: UserKey | null, semester: keyof typeof semesters) {
  const path = `/${semesters[semester].query}`;
  if (user) await login(page, users[user], path);
  else await page.goto(path);
  // The listings arrive after the shell, so wait for the one group always present.
  await expect(sidebarGroup(page, "Public Courses")).toBeVisible();
}
