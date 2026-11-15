import { execFileSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

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

/**
 * Where the server keeps the chunks: its paths.mass, plus browser-recordings. The tests
 * that look at what arrived skip without it, as on a server that is not local.
 */
const chunkRoot = resolve(process.env.E2E_MASS_STORAGE ?? "../runner/storage", "browser-recordings");
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
async function recordingsOf(api: APIRequestContext, lectureId: number): Promise<Array<{ id: number; version: string }>> {
  const response = await api.get(`/api/v2/courses/${COURSE}/streams/${lectureId}/browser-recordings`);
  return (await response.json()).recordings ?? [];
}

/** A segment's chunks as the server has them, joined into a file. */
function segmentFile(recordingId: number, segment: number): string {
  const dir = join(chunkRoot, String(recordingId), String(segment).padStart(6, "0"));
  const chunks = readdirSync(dir)
    .filter((n) => /^\d+$/.test(n))
    .sort();
  const file = join(tmpdir(), `e2e-recording-${recordingId}-${segment}.webm`);
  writeFileSync(file, Buffer.concat(chunks.map((n) => readFileSync(join(dir, n)))));
  return file;
}

/** Whether ffprobe is there to look into a recording; CI's browser job has no ffmpeg. */
function hasFfprobe(): boolean {
  try {
    execFileSync("ffprobe", ["-version"], { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

function streamTypes(file: string): string[] {
  return execFileSync("ffprobe", ["-v", "error", "-show_entries", "stream=codec_type", "-of", "csv=p=0", file])
    .toString()
    .trim()
    .split("\n");
}

/** What the browser's "Stop sharing" does to the screen: the shared track ends. */
async function stopSharing(page: Page): Promise<void> {
  await page.evaluate(() => {
    const video = [...document.querySelectorAll("video")].find((v) => v.srcObject instanceof MediaStream);
    const track = (video?.srcObject as MediaStream | undefined)?.getVideoTracks()[0];
    // stop() alone fires no ended event; the browser's own button does.
    track?.stop();
    track?.dispatchEvent(new Event("ended"));
  });
}

async function prepare(page: Page, title: string, camera = true): Promise<void> {
  await login(page, users.prof1, lecturesPage);
  await page.getByRole("button", { name: "Create lecture" }).click();
  const w = wizard(page);
  await w.getByText("Record here").click();
  await w.getByRole("button", { name: "Continue" }).click();
  await w.getByLabel("Title").fill(title);
  await w.getByRole("button", { name: "Continue" }).click();
  await w.getByRole("button", { name: "Share screen" }).click();
  // The fake devices are slow to start the first time.
  await expect(w.getByRole("button", { name: "Share another screen" })).toBeVisible({ timeout: 20_000 });
  if (!camera) return;
  await w.getByRole("button", { name: "Turn on camera" }).click();
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

  test("records sound with only the screen shared", async ({ page, playwright }) => {
    test.skip(!existsSync(chunkRoot), "the server's chunks are not reachable from here");
    test.skip(!hasFfprobe(), "no ffprobe to look into the recording");
    const prof1 = await apiAs(playwright, users.prof1);
    const title = `${PREFIX} Nur Bildschirm`;
    try {
      await prepare(page, title, false);
      const w = wizard(page);
      await w.getByRole("button", { name: "Start recording" }).click();
      await expect(recordingStatus(w)).toBeVisible({ timeout: 15_000 });
      const lecture = await lectureNamed(prof1, title);
      await moveToTestYear(prof1, lecture.id);
      await page.waitForTimeout(5000);
      await w.getByRole("button", { name: "Stop and upload" }).click();
      await expect(page.getByRole("status").filter({ hasText: "Created 1 lecture." })).toBeVisible({ timeout: 60_000 });

      const [recording] = await recordingsOf(prof1, lecture.id);
      expect(recording.version).toBe("PRES");
      expect(streamTypes(segmentFile(recording.id, 0))).toEqual(expect.arrayContaining(["video", "audio"]));
    } finally {
      await cleanUp(prof1, title);
    }
  });

  test("keeps both versions on one timeline when sharing stops for a while", async ({ page, playwright }) => {
    test.skip(!existsSync(chunkRoot), "the server's chunks are not reachable from here");
    const prof1 = await apiAs(playwright, users.prof1);
    const title = `${PREFIX} Unterbrochen`;
    try {
      await prepare(page, title);
      const w = wizard(page);
      await w.getByRole("button", { name: "Start recording" }).click();
      await expect(recordingStatus(w)).toBeVisible({ timeout: 15_000 });
      const lecture = await lectureNamed(prof1, title);
      await moveToTestYear(prof1, lecture.id);
      await page.waitForTimeout(3000);

      await stopSharing(page);
      await expect(w.getByRole("button", { name: "Share screen again" })).toBeVisible();
      // Still recording the camera, and still offering to stop.
      await expect(w.getByRole("button", { name: "Stop and upload" })).toBeVisible();
      await page.waitForTimeout(4000);
      await w.getByRole("button", { name: "Share screen again" }).click();
      await expect(w.getByRole("button", { name: "Share screen again" })).toBeHidden({ timeout: 20_000 });
      await page.waitForTimeout(3000);
      await w.getByRole("button", { name: "Stop and upload" }).click();
      await expect(page.getByRole("status").filter({ hasText: "Created 1 lecture." })).toBeVisible({ timeout: 60_000 });

      const pres = (await recordingsOf(prof1, lecture.id)).find((r) => r.version === "PRES")!;
      const gap = Number(readFileSync(join(chunkRoot, String(pres.id), "000001", "gap-ms"), "utf8"));
      // About the four seconds waited, give or take the time to share again.
      expect(gap).toBeGreaterThan(3500);
      expect(gap).toBeLessThan(10_000);
    } finally {
      await cleanUp(prof1, title);
    }
  });

  test("pads the screen version when sharing stops for good", async ({ page, playwright }) => {
    test.skip(!existsSync(chunkRoot), "the server's chunks are not reachable from here");
    const prof1 = await apiAs(playwright, users.prof1);
    const title = `${PREFIX} Nicht wieder geteilt`;
    try {
      await prepare(page, title);
      const w = wizard(page);
      await w.getByRole("button", { name: "Start recording" }).click();
      await expect(recordingStatus(w)).toBeVisible({ timeout: 15_000 });
      const lecture = await lectureNamed(prof1, title);
      await moveToTestYear(prof1, lecture.id);
      await page.waitForTimeout(3000);

      await stopSharing(page);
      await expect(w.getByRole("button", { name: "Share screen again" })).toBeVisible();
      await page.waitForTimeout(4000);
      await w.getByRole("button", { name: "Stop and upload" }).click();
      await expect(page.getByRole("status").filter({ hasText: "Created 1 lecture." })).toBeVisible({ timeout: 60_000 });

      // The time after the screen stopped is kept as the gap of a segment after the last.
      const pres = (await recordingsOf(prof1, lecture.id)).find((r) => r.version === "PRES")!;
      const gap = Number(readFileSync(join(chunkRoot, String(pres.id), "000001", "gap-ms"), "utf8"));
      expect(gap).toBeGreaterThan(3500);
      expect(gap).toBeLessThan(10_000);
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
