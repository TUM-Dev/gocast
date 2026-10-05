import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { users } from "./seed";

/**
 * A course's administration page in the SPA: its frame, the settings tab and the
 * participants tab, and the sidebar's tree of administered courses.
 *
 * Course 1 (Einführung Brauereiwesen, public, Summer 2022) is administered by prof1
 * and prof2, course 2 by prof2 alone. Every change made here is put back, and copying
 * and deleting only touch a course created here in the test semester 1234.
 */

const settings1 = "/admin/courses/1/settings";
const participants1 = "/admin/courses/1/participants";

interface Api {
  context: APIRequestContext;
  headers: Record<string, string>;
}

async function apiAsProf1(playwright: Parameters<typeof apiAs>[0]): Promise<Api> {
  const context = await apiAs(playwright, users.prof1);
  const token = await bearerToken(context);
  return { context, headers: { Authorization: `Bearer ${token}` } };
}

async function courseSettings(api: Api, id: number) {
  return (await api.context.get(`/api/v2/courses/${id}/admin`, { headers: api.headers })).json();
}

const tabs = (page: Page) => page.getByRole("navigation", { name: "Course administration" });

test.describe("the course administration page", () => {
  test("is served the SPA shell for both tabs", async ({ page }) => {
    await login(page, users.prof1);

    for (const path of [settings1, participants1]) {
      const response = await page.goto(path);
      expect(response?.status()).toBe(200);
      expect((await response?.text()) ?? "").toContain("/spa-assets/");
    }
  });

  test("names the course and offers its tabs", async ({ page }) => {
    await login(page, users.prof1, settings1);

    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Einführung Brauereiwesen");
    await expect(page.getByRole("heading", { level: 1 }).getByRole("link")).toHaveAttribute(
      "href",
      "/course/2022/S/brauereiwesen",
    );

    // Lectures is still the server-rendered page.
    await expect(tabs(page).getByRole("link", { name: "Lectures" })).toHaveAttribute("href", "/admin/course/1");
    await expect(tabs(page).getByRole("link", { name: "Settings" })).toHaveAttribute("aria-current", "page");
    await expect(tabs(page).getByRole("link", { name: "Statistics" })).toHaveAttribute(
      "href",
      "/admin/courses/1/stats",
    );

    await tabs(page).getByRole("link", { name: "Participants" }).click();
    await expect(page).toHaveURL(participants1);
    await expect(page.getByRole("heading", { name: "Invited Participants" })).toBeVisible();
  });

  test("is refused by the server to a lecturer of other courses", async ({ page }) => {
    await login(page, users.prof1);

    for (const path of ["/admin/courses/2/settings", "/admin/courses/2/participants"]) {
      const response = await page.goto(path);
      expect(response?.ok()).toBe(false);
      expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
    }
  });

  test("is refused by the server to a student", async ({ page }) => {
    await login(page, users.studi1);

    const response = await page.goto(settings1);
    expect(response?.ok()).toBe(false);
    expect((await response?.text()) ?? "").not.toContain("/spa-assets/");
  });
});

test.describe("the settings tab", () => {
  test("shows the course's settings as the fixture has them", async ({ page, playwright }) => {
    const api = await apiAsProf1(playwright);
    const fixture = await courseSettings(api, 1);

    await login(page, users.prof1, settings1);

    await expect(page.getByText("brauereiwesen", { exact: true })).toBeVisible();
    await expect(page.getByRole("radio", { name: /^Public:/ })).toBeChecked();
    // Course 1 is not linked to TUMOnline, so nobody could be enrolled.
    await expect(page.getByRole("radio", { name: /^Enrolled:/ })).toBeDisabled();

    const box = (name: string) => page.getByRole("checkbox", { name, exact: false });
    for (const [label, value] of [
      ["Enable VoD", fixture.vodEnabled],
      ["Enable downloads", fixture.downloadsEnabled],
      ["Enable live chat", fixture.chatEnabled],
      ["Private livestreams", fixture.livePrivate],
    ] as const) {
      if (value) await expect(box(label)).toBeChecked();
      else await expect(box(label)).not.toBeChecked();
    }
  });

  test("saves a change and changes it back", async ({ page, playwright }) => {
    const api = await apiAsProf1(playwright);
    const before = await courseSettings(api, 1);

    try {
      await login(page, users.prof1, settings1);
      const downloads = page.getByRole("checkbox", { name: /Enable downloads/ });
      const save = page.getByRole("button", { name: "Save Settings" });
      await expect(save).toBeDisabled();

      await downloads.click();
      await save.click();
      await expect(page.getByRole("status").filter({ hasText: "Settings saved." })).toBeVisible();
      expect((await courseSettings(api, 1)).downloadsEnabled ?? false).toBe(!(before.downloadsEnabled ?? false));

      await downloads.click();
      await save.click();
      await expect(page.getByRole("status").filter({ hasText: "Settings saved." })).toBeVisible();
    } finally {
      await api.context.patch("/api/v2/courses/1/settings", {
        headers: api.headers,
        data: { downloadsEnabled: before.downloadsEnabled ?? false },
      });
    }

    expect(await courseSettings(api, 1)).toEqual(before);
  });

  test("lists the course's admins and finds users to add", async ({ page }) => {
    await login(page, users.prof1, settings1);

    const table = page.getByRole("table", { name: "Course administrators" });
    await expect(table.getByRole("row", { name: new RegExp(users.prof1.name) })).toBeVisible();
    await expect(table.getByRole("row", { name: new RegExp(users.prof2.name) })).toBeVisible();

    await page.getByLabel("Add an administrator").fill("studi");
    const results = page.getByRole("list", { name: "Search results" });
    await expect(results.getByRole("listitem")).toHaveCount(3);
    await expect(results.getByRole("button", { name: `Add ${users.studi1.name}` })).toBeVisible();
  });

  test("copies a course into another semester and deletes both", async ({ page, playwright }) => {
    const api = await apiAsProf1(playwright);
    const created: number[] = [];

    try {
      const create = await api.context.post("/api/v2/courses", {
        headers: api.headers,
        data: { name: "E2E Settings Kurs", slug: "e2e-settings", year: 1234, term: "S", language: "" },
      });
      expect(create.status()).toBe(200);
      const id = (await create.json()).courseId;
      created.push(id);

      await login(page, users.prof1, `/admin/courses/${id}/settings`);
      await expect(page.getByRole("heading", { level: 1 })).toHaveText("E2E Settings Kurs");
      // A course without lectures has no lecture halls to configure.
      await expect(page.getByRole("heading", { name: "Lecture Hall Settings" })).toHaveCount(0);

      await page.getByRole("button", { name: /Copy course/ }).click();
      await page.getByLabel("Semester").selectOption("W");
      await page.getByLabel("Year").fill("1234");
      await page.getByRole("button", { name: "Copy to Winter 1234/35" }).click();

      await expect(page).toHaveURL(/\/admin\/courses\/(\d+)\/settings\?copied=0$/);
      const copyId = Number(/courses\/(\d+)\//.exec(page.url())?.[1]);
      expect(copyId).not.toBe(id);
      created.push(copyId);
      await expect(page.getByRole("status").filter({ hasText: "The course was copied." })).toBeVisible();
      await expect(page.getByText("Winter 1234/35").first()).toBeVisible();

      page.once("dialog", (dialog) => {
        expect(dialog.message()).toContain('"E2E Settings Kurs"');
        void dialog.accept();
      });
      await page.getByRole("button", { name: /Delete course/ }).click();
      await expect(page).toHaveURL("/admin");
      expect((await api.context.get(`/api/v2/courses/${copyId}/admin`, { headers: api.headers })).status()).toBe(
        404,
      );
    } finally {
      for (const id of created) {
        await api.context.delete(`/api/v2/courses/${id}`, { headers: api.headers });
      }
    }
  });
});

test.describe("the participants tab", () => {
  test("lists the invitations and refuses a malformed one without inviting anyone", async ({
    page,
    playwright,
  }) => {
    const api = await apiAsProf1(playwright);
    const before = await (
      await api.context.get("/api/v2/courses/1/participants", { headers: api.headers })
    ).json();

    await login(page, users.prof1, participants1);
    await expect(page.getByRole("heading", { name: "Invited Participants" })).toBeVisible();
    const count = (before.participants ?? []).length;
    if (count) {
      await expect(page.getByRole("table", { name: "Invited participants" }).getByRole("row")).toHaveCount(count + 1);
    } else {
      await expect(page.getByText("Nobody has been invited yet.")).toBeVisible();
    }

    await page.getByLabel("Name", { exact: true }).fill("Tim");
    await page.getByLabel("Email", { exact: true }).fill("not-an-address");
    await page.getByRole("button", { name: "Invite", exact: true }).click();
    await expect(page.getByRole("alert")).toContainText("no valid email address");

    // A batch line that is not name,email is refused before anything is sent.
    await page.getByLabel(/Invite several at once/).fill("Tim,tim@example.com\njust-an-address@example.com");
    await page.getByRole("button", { name: "Invite all" }).click();
    await expect(page.getByRole("alert")).toContainText("Line 2");

    const after = await (await api.context.get("/api/v2/courses/1/participants", { headers: api.headers })).json();
    expect(after).toEqual(before);
  });
});

test.describe("the sidebar's course tree", () => {
  test("lists every course prof1 administers, by semester", async ({ page }) => {
    await login(page, users.prof1, settings1);

    const nav = page.getByRole("navigation", { name: "Administration" });
    for (const semester of ["Summer 2022", "Winter 2021/22"]) {
      const group = nav.locator("details").filter({ has: page.getByText(semester, { exact: true }) });
      if (!(await group.evaluate((el) => (el as HTMLDetailsElement).open))) {
        await group.locator("summary").click();
      }
    }

    for (const [name, id] of [
      ["Einführung Brauereiwesen", 1],
      ["Praktikum: Golang", 3],
      ["Fortgeschrittene Bierkunde", 4],
    ] as const) {
      await expect(nav.getByRole("link", { name })).toHaveAttribute("href", `/admin/courses/${id}/settings`);
    }
    await expect(nav.getByRole("link", { name: "Spieleentwicklung für Dummies" })).toHaveCount(0);
  });
});
