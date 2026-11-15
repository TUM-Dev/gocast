import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * Recording a lecture in the browser, with Chromium's fake camera, microphone and
 * screen. The test ends where the server has both versions finalized. With
 * E2E_RUNNER=1, for a server with a runner connected, it waits for the runner to
 * convert them onto the lecture as well; CI has none.
 *
 * On Praktikum: Golang (course 3), as create-lecture.spec.ts, for the same reasons. A
 * recording's lecture is dated when it starts, so the test moves it to 1234, the test
 * year, as soon as it exists, and deletes it in a finally.
 */

const PREFIX = "E2E-Recording";
const COURSE = 3;
const lecturesPage = `/admin/courses/${COURSE}/lectures`;

test.use({
  permissions: ["camera", "microphone"],
  launchOptions: {
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
      "--auto-select-desktop-capture-source=Entire screen",
    ],
  },
});

interface Listed {
  id: number;
  name: string;
  vodVersions?: string[];
}

async function listed(api: APIRequestContext): Promise<Listed[]> {
  const response = await api.get(`/api/v2/courses/${COURSE}/lectures/admin`);
  expect(response.status()).toBe(200);
  return (await response.json()).lectures ?? [];
}

async function lectureNamed(api: APIRequestContext, name: string): Promise<Listed> {
  let found: Listed | undefined;
  await expect.poll(async () => (found = (await listed(api)).find((l) => l.name === name))).toBeTruthy();
  return found!;
}

/** Out of every list by date that another file could read while this one runs. */
async function moveToTestYear(api: APIRequestContext, id: number): Promise<void> {
  const response = await api.patch(`/api/v2/courses/${COURSE}/streams/${id}`, {
    data: { start: "1234-05-06T10:00:00Z", end: "1234-05-06T11:00:00Z" },
  });
  expect(response.status()).toBe(200);
}

async function cleanUp(api: APIRequestContext, title: string): Promise<void> {
  const ids = (await listed(api)).filter((l) => l.name.startsWith(title)).map((l) => l.id);
  if (!ids.length) return;
  await api.post(`/api/v2/courses/${COURSE}/streams/delete`, { data: { streamIds: ids } });
}

const wizard = (page: Page) => page.getByRole("region", { name: "Create a lecture" });
const recordingStatus = (w: Locator) => w.getByRole("status").filter({ hasText: /Recording \d+:\d\d:\d\d/ });

/** Opens the wizard on the recorder for a lecture of that title, both sources on. */
async function prepare(page: Page, title: string): Promise<void> {
  await login(page, users.prof1, lecturesPage);
  await page.getByRole("button", { name: "Create lecture" }).click();
  const w = wizard(page);
  await w.getByText("Record here").click();
  await w.getByRole("button", { name: "Continue" }).click();
  await w.getByLabel("Title").fill(title);
  await w.getByRole("button", { name: "Continue" }).click();
  await w.getByRole("button", { name: "Share screen" }).click();
  await w.getByRole("button", { name: "Turn on camera" }).click();
  // The fake devices are slow to start the first time.
  await expect(w.getByRole("button", { name: "Share another screen" })).toBeVisible({ timeout: 20_000 });
  await expect(w.getByRole("button", { name: "Choose another camera" })).toBeVisible({ timeout: 20_000 });
}

interface Recording {
  version: string;
  finalized?: boolean;
  handedOff?: boolean;
  received?: number[];
}

/** Both versions arrived: finalized with chunks, and converted when a runner is there to. */
async function waitForVersions(api: APIRequestContext, name: string): Promise<void> {
  const lecture = await lectureNamed(api, name);
  if (process.env.E2E_RUNNER) {
    await expect
      .poll(async () => (await lectureNamed(api, name)).vodVersions?.slice().sort() ?? [], { timeout: 90_000 })
      .toEqual(["CAM", "PRES"]);
    return;
  }
  // Without a runner the hand-off waits, and the server keeps what arrived.
  await expect
    .poll(async () => {
      const response = await api.get(`/api/v2/courses/${COURSE}/streams/${lecture.id}/browser-recordings`);
      const recordings: Recording[] = (await response.json()).recordings ?? [];
      return recordings
        .filter((r) => r.finalized && (r.received ?? []).some((n) => n > 0))
        .map((r) => r.version)
        .sort();
    })
    .toEqual(["CAM", "PRES"]);
}

test.describe("recording a lecture in the browser", () => {
  test.describe.configure({ timeout: 180_000 });

  test("records both versions through an outage", async ({ page, playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const title = `${PREFIX} Ausfall`;
    try {
      await prepare(page, title);
      const w = wizard(page);
      await w.getByRole("button", { name: "Start recording" }).click();
      await expect(recordingStatus(w)).toBeVisible({ timeout: 15_000 });
      await moveToTestYear(prof1, (await lectureNamed(prof1, title)).id);
      // Closing would hide the recording's controls.
      await expect(w.getByRole("button", { name: "Close" })).toBeDisabled();

      await page.waitForTimeout(4000);

      // The server going away as the browser sees it: Traefik's plain-text 404 while
      // the container is down, then a 503 while it starts.
      let outage: "down" | "starting" | "up" = "down";
      await page.route("**/api/v2/browser-recordings/**", (route) =>
        outage === "down"
          ? route.fulfill({ status: 404, contentType: "text/plain", body: "404 page not found" })
          : outage === "starting"
            ? route.fulfill({ status: 503, body: "" })
            : route.fallback(),
      );
      await expect(w.getByText("The server can't be reached")).toBeVisible({ timeout: 15_000 });
      await page.waitForTimeout(4000);
      outage = "starting";
      await page.waitForTimeout(3000);
      outage = "up";
      await expect(w.getByText("The server can't be reached")).toBeHidden({ timeout: 30_000 });

      await page.waitForTimeout(2000);
      await w.getByRole("button", { name: "Stop and upload" }).click();
      await expect(page.getByRole("status").filter({ hasText: "Created 1 lecture." })).toBeVisible({ timeout: 60_000 });
      await expect(wizard(page)).toBeHidden();

      await waitForVersions(prof1, title);
    } finally {
      await cleanUp(prof1, title);
    }
  });

  test("throws a recording away and lets the wizard go", async ({ page, playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const title = `${PREFIX} Verworfen`;
    try {
      await prepare(page, title);
      const w = wizard(page);
      await w.getByRole("button", { name: "Start recording" }).click();
      await expect(recordingStatus(w)).toBeVisible({ timeout: 15_000 });
      const lecture = await lectureNamed(prof1, title);
      await moveToTestYear(prof1, lecture.id);
      await page.waitForTimeout(3000);

      page.on("dialog", (dialog) => void dialog.accept());
      await w.getByRole("button", { name: "Throw away" }).click();
      await expect(page.getByRole("alert").filter({ hasText: "The recording was thrown away." })).toBeVisible();
      await expect(wizard(page)).toBeHidden();
      await expect(page.getByRole("button", { name: "Create lecture" })).toBeEnabled();

      const response = await prof1.get(`/api/v2/courses/${COURSE}/streams/${lecture.id}/browser-recordings`);
      expect((await response.json()).recordings ?? []).toEqual([]);

      // Nothing of it is left in the browser to offer again, not even its last chunk.
      await page.reload();
      await expect(page.getByRole("heading", { name: title })).toBeVisible();
      await expect(page.getByRole("region", { name: "Unfinished recordings in this browser" })).toBeHidden();

      // And the next wizard starts afresh.
      await page.getByRole("button", { name: "Create lecture" }).click();
      await wizard(page).getByText("Record here").click();
      await wizard(page).getByRole("button", { name: "Continue" }).click();
      await wizard(page).getByLabel("Title").fill(`${title} 2`);
      await wizard(page).getByRole("button", { name: "Continue" }).click();
      await expect(wizard(page).getByRole("button", { name: "Start recording" })).toBeVisible();
    } finally {
      await cleanUp(prof1, title);
    }
  });

  test("finishes a recording after the page was reloaded during it", async ({ page, playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const title = `${PREFIX} Neu geladen`;
    try {
      await prepare(page, title);
      const w = wizard(page);
      await w.getByRole("button", { name: "Start recording" }).click();
      await expect(recordingStatus(w)).toBeVisible({ timeout: 15_000 });
      await moveToTestYear(prof1, (await lectureNamed(prof1, title)).id);

      // Chunks pile up in the browser while the server can't take them, so the reload
      // leaves something behind to recover.
      await page.route("**/api/v2/browser-recordings/**", (route) => route.fulfill({ status: 503, body: "" }));
      await page.waitForTimeout(6000);

      page.on("dialog", (dialog) => void dialog.accept());
      await page.unroute("**/api/v2/browser-recordings/**");
      await page.reload();

      const recovery = page.getByRole("region", { name: "Unfinished recordings in this browser" });
      await expect(recovery).toBeVisible();
      await expect(recovery.getByText(title)).toHaveCount(2);
      for (const button of await recovery.getByRole("button", { name: "Upload and finish" }).all()) {
        await button.click();
      }
      await expect(recovery).toBeHidden({ timeout: 60_000 });

      await waitForVersions(prof1, title);
    } finally {
      await cleanUp(prof1, title);
    }
  });
});
