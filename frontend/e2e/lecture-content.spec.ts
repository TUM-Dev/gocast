import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";

import { apiAs, login } from "./helpers";
import { users } from "./seed";

/**
 * A lecture's content on its card in the lectures tab: sections, attachments, the
 * custom thumbnail and subtitles, as prof1 on Einführung Brauereiwesen (course 1).
 *
 * Everything happens on "VL 1: Was ist Bier?" (stream 1), a recording with no
 * sections, attachments or custom thumbnail in the fixture. Every test takes away what
 * it adds in a finally, through the API, so a failing page cannot leave the fixture
 * changed; the upload tests also check the file left runner/storage, config.yaml's
 * mass storage, which assumes the server runs from this checkout.
 */

const page1 = "/admin/courses/1/lectures";
const lectureName = "VL 1: Was ist Bier?";
const lecture = 1;
const base = `/api/v2/courses/1/streams/${lecture}`;
const massStorage = fileURLToPath(new URL("../../runner/storage", import.meta.url));

/** A 1×1 transparent PNG, so the thumbnail is a real image the page can show. */
const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
  "base64",
);

interface AdminLecture {
  id: number;
  videoSections?: { id: number; description: string }[];
  files?: { id: number; type: number; friendlyName: string }[];
}

async function adminLecture(context: APIRequestContext): Promise<AdminLecture> {
  const response = await context.get("/api/v2/courses/1/lectures/admin");
  expect(response.status()).toBe(200);
  const lectures: AdminLecture[] = (await response.json()).lectures ?? [];
  return lectures.find((l) => l.id === lecture)!;
}

/** Every file below dir whose content is exactly `content`. */
function filesHolding(dir: string, content: Buffer): string[] {
  let entries: string[];
  try {
    entries = readdirSync(dir);
  } catch {
    return [];
  }
  return entries.flatMap((name) => {
    const path = join(dir, name);
    const stat = statSync(path);
    if (stat.isDirectory()) return filesHolding(path, content);
    // The size first: recordings live here too.
    return stat.size === content.length && readFileSync(path).equals(content) ? [path] : [];
  });
}

/** Opens the lecture's editor and answers every confirm() with OK. */
async function openLecture(page: Page): Promise<Locator> {
  page.on("dialog", (dialog) => void dialog.accept());
  await login(page, users.prof1, page1);
  const card = page
    .locator('li[id^="lecture-"]')
    .filter({ has: page.getByRole("heading", { name: lectureName, exact: true }) });
  await card.getByRole("button", { name: "Edit" }).click();
  return card;
}

test.describe("a lecture's sections", () => {
  test("are checked, added, edited and deleted from its card", async ({ page, playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const before = (await adminLecture(prof1)).videoSections ?? [];
    const card = await openLecture(page);
    const sections = card.getByRole("region", { name: "Video sections" });
    const add = sections.getByRole("form", { name: "Add a section" });

    try {
      // Refused here, with the reasons beside the form, before the server is asked.
      await add.getByRole("spinbutton", { name: "Minutes" }).fill("70");
      await add.getByRole("button", { name: "Add section" }).click();
      await expect(add.getByText("Minutes and seconds must be below 60.")).toBeVisible();
      await expect(add.getByText("Give the section a description.")).toBeVisible();

      await add.getByRole("spinbutton", { name: "Minutes" }).fill("4");
      await add.getByRole("spinbutton", { name: "Seconds" }).fill("2");
      await add.getByRole("textbox", { name: "Description", exact: true }).fill("E2E UI chapter");
      await add.getByRole("button", { name: "Add section" }).click();

      const list = sections.getByRole("list", { name: "Video sections" });
      await expect(sections.getByRole("status")).toHaveText('Section "E2E UI chapter" added.');
      const row = list.getByRole("listitem").filter({ hasText: "E2E UI chapter" });
      await expect(row).toContainText("4:02");
      await expect(add.getByRole("textbox", { name: "Description", exact: true })).toHaveValue("");

      // Edited to 0:00, which v1 could not save.
      await row.getByRole("button", { name: "Edit section E2E UI chapter", exact: true }).click();
      const edit = sections.getByRole("form", { name: "Edit section E2E UI chapter" });
      await edit.getByRole("spinbutton", { name: "Minutes" }).fill("0");
      await edit.getByRole("spinbutton", { name: "Seconds" }).fill("0");
      await edit.getByRole("textbox", { name: "Description", exact: true }).fill("E2E UI chapter, moved");
      await edit.getByRole("button", { name: "Save section" }).click();

      const moved = list.getByRole("listitem").filter({ hasText: "E2E UI chapter, moved" });
      await expect(moved).toContainText("0:00");
      const saved = (await adminLecture(prof1)).videoSections?.find((s) => s.description === "E2E UI chapter, moved");
      expect(saved, "the edit did not reach the server").toBeTruthy();

      await moved.getByRole("button", { name: "Delete section E2E UI chapter, moved" }).click();
      await expect(sections.getByRole("status")).toHaveText('Section "E2E UI chapter, moved" deleted.');
      // The status names it too, so look in the list.
      await expect(sections.getByRole("listitem").filter({ hasText: "E2E UI chapter" })).toHaveCount(0);
    } finally {
      for (const s of (await adminLecture(prof1)).videoSections ?? []) {
        if (s.description.startsWith("E2E UI chapter")) await prof1.delete(`${base}/sections/${s.id}`);
      }
    }
    expect(((await adminLecture(prof1)).videoSections ?? []).map((s) => s.id)).toEqual(before.map((s) => s.id));
  });
});

test.describe("a lecture's attachments", () => {
  test("are uploaded from the card and deleted with their file", async ({ page, playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const content = Buffer.from(`e2e ui attachment ${Date.now()}`);
    const path = test.info().outputPath("e2e-ui-handout.txt");
    writeFileSync(path, content);
    const card = await openLecture(page);
    const attachments = card.getByRole("region", { name: "Attachments" });

    try {
      await attachments.getByLabel("Upload attachments").setInputFiles(path);
      await expect(attachments.getByRole("status")).toHaveText("Uploaded e2e-ui-handout.txt.");
      const link = attachments.getByRole("link", { name: "e2e-ui-handout.txt" });
      await expect(link).toHaveAttribute("href", /^\/api\/download\/\d+\?type=download$/);
      expect(filesHolding(massStorage, content)).toHaveLength(1);

      await attachments.getByRole("button", { name: "Delete attachment e2e-ui-handout.txt" }).click();
      await expect(attachments.getByRole("status")).toHaveText("Deleted e2e-ui-handout.txt.");
      await expect(link).toHaveCount(0);
    } finally {
      for (const f of (await adminLecture(prof1)).files ?? []) {
        if (f.friendlyName === "e2e-ui-handout.txt") await prof1.delete(`${base}/attachments/${f.id}`);
      }
    }
    expect(filesHolding(massStorage, content)).toHaveLength(0);
  });
});

test.describe("a lecture's custom thumbnail", () => {
  test("is uploaded from the card, shown and deleted with its file", async ({ page, playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    // Unique bytes, so the file on disk is told apart from any other upload.
    const content = Buffer.concat([PNG, Buffer.from(`e2e ui thumbnail ${Date.now()}`)]);
    const path = test.info().outputPath("e2e-ui-cover.png");
    writeFileSync(path, content);
    const card = await openLecture(page);
    const thumbnail = card.getByRole("region", { name: "Thumbnail" });

    try {
      await expect(thumbnail.getByText("None uploaded")).toBeVisible();
      await thumbnail.getByLabel("Upload thumbnail").setInputFiles(path);
      await expect(thumbnail.getByRole("status")).toHaveText("Thumbnail uploaded.");
      const image = thumbnail.getByRole("img", { name: "Custom thumbnail" });
      await expect(image).toBeVisible();
      // Served, not a broken image.
      await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(1);
      await expect(thumbnail.getByLabel("Replace thumbnail")).toBeAttached();
      expect(filesHolding(massStorage, content)).toHaveLength(1);

      await thumbnail.getByRole("button", { name: "Delete thumbnail" }).click();
      await expect(thumbnail.getByRole("status")).toHaveText("Thumbnail deleted.");
      await expect(thumbnail.getByText("None uploaded")).toBeVisible();
    } finally {
      // 404 when the page already deleted it.
      await prof1.delete(`${base}/thumbnail`);
    }
    expect(filesHolding(massStorage, content)).toHaveLength(0);
  });

  test("refuses a file that is not an image without sending it", async ({ page }) => {
    const path = test.info().outputPath("e2e-ui-cover.pdf");
    writeFileSync(path, "not an image");
    const card = await openLecture(page);
    const thumbnail = card.getByRole("region", { name: "Thumbnail" });

    await thumbnail.getByLabel("Upload thumbnail").setInputFiles(path);
    await expect(thumbnail.getByRole("alert")).toHaveText("The thumbnail must be a JPG, PNG, GIF or WebP image.");
    await expect(thumbnail.getByText("None uploaded")).toBeVisible();
  });
});

test.describe("a lecture's subtitles", () => {
  test("say so when no voice service is there to generate them", async ({ page }) => {
    // config.yaml names a voice service on localhost that nothing serves.
    const card = await openLecture(page);
    const recording = card.getByRole("region", { name: "Recording" });

    await recording.getByRole("button", { name: "German" }).click();
    await expect(recording.getByRole("alert")).toHaveText("Subtitle generation is not available right now.");
  });
});

test.describe("the lecture content", () => {
  test("is not shown to a student", async ({ page }) => {
    await login(page, users.studi1);

    const response = await page.goto(page1);
    expect(response?.ok()).toBe(false);
    await expect(page.getByRole("region", { name: "Video sections" })).toHaveCount(0);
    await expect(page.getByLabel("Upload attachments")).toHaveCount(0);
  });
});
