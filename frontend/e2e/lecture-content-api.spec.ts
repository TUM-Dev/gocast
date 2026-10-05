import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { type APIRequestContext, expect, test } from "@playwright/test";

import { apiAs } from "./helpers";
import { users } from "./seed";

/**
 * The lecture content RPCs and the two uploads beside them, API only: no page uses
 * them yet.
 *
 * Everything runs on Einführung Brauereiwesen (course 1, administered by prof1) and
 * its "VL 1: Was ist Bier?" (stream 1), which has no sections, attachments or custom
 * thumbnail in the fixture; every test takes away what it adds, in a finally.
 * Spieleentwicklung's stream 7 is the other course's lecture the refusals aim at.
 *
 * The upload tests look for the file on disk, under config.yaml's mass storage
 * (runner/storage), so they assume the server runs from this checkout, as the
 * playwright.config.ts server does.
 */

const lecture = 1;
const base = `/api/v2/courses/1/streams/${lecture}`;
const foreign = `/api/v2/courses/1/streams/7`;
const massStorage = fileURLToPath(new URL("../../runner/storage", import.meta.url));

interface Section {
  id: number;
  description: string;
  startHours?: number;
  startMinutes?: number;
  startSeconds?: number;
}

/**
 * The lecture's sections, from the administration list: the viewers' read carries no
 * section IDs.
 */
async function sections(context: APIRequestContext): Promise<Section[]> {
  const response = await context.get("/api/v2/courses/1/lectures/admin");
  expect(response.status()).toBe(200);
  const lectures: { id: number; videoSections?: Section[] }[] = (await response.json()).lectures ?? [];
  return lectures.find((l) => l.id === lecture)?.videoSections ?? [];
}

/** Every file below dir whose content is marker. */
function filesHolding(dir: string, marker: string): string[] {
  let entries: string[];
  try {
    entries = readdirSync(dir);
  } catch {
    return [];
  }
  return entries.flatMap((name) => {
    const path = join(dir, name);
    const stat = statSync(path);
    if (stat.isDirectory()) {
      return filesHolding(path, marker);
    }
    // The size first: recordings live here too.
    return stat.size === Buffer.byteLength(marker) && readFileSync(path, "utf8") === marker ? [path] : [];
  });
}

test.describe("a lecture's sections", () => {
  test("are created, moved to 0:00:00 and deleted", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const before = await sections(prof1);

    const created = await prof1.post(`${base}/sections`, {
      data: { sections: [{ description: "E2E chapter", startMinutes: 4, startSeconds: 2 }] },
    });
    expect(created.status()).toBe(200);
    const section = ((await created.json()).sections as Section[]).find((s) => s.description === "E2E chapter");
    expect(section, "the new section is not answered").toBeTruthy();

    try {
      expect((await sections(prof1)).map((s) => s.id)).toContain(section!.id);

      // v1 could not do this: its update skipped zero values.
      const updated = await prof1.put(`${base}/sections/${section!.id}`, {
        data: { description: "E2E chapter, moved", startHours: 0, startMinutes: 0, startSeconds: 0 },
      });
      expect(updated.status()).toBe(200);
      const moved = (await sections(prof1)).find((s) => s.id === section!.id);
      expect(moved?.description).toBe("E2E chapter, moved");
      expect(moved?.startMinutes ?? 0).toBe(0);
      expect(moved?.startSeconds ?? 0).toBe(0);
    } finally {
      expect((await prof1.delete(`${base}/sections/${section!.id}`)).status()).toBe(200);
    }

    expect((await sections(prof1)).map((s) => s.id)).toEqual(before.map((s) => s.id));
  });

  test("refuse minutes past 59 and a blank description", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const before = await sections(prof1);

    for (const bad of [{ description: "x", startMinutes: 60 }, { description: " " }]) {
      expect((await prof1.post(`${base}/sections`, { data: { sections: [bad] } })).status()).toBe(400);
    }
    expect((await sections(prof1)).length).toBe(before.length);
  });
});

test.describe("a lecture's attachments", () => {
  test("are uploaded, land on disk and are deleted with their file", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const marker = `e2e attachment ${Date.now()}`;

    const uploaded = await prof1.post(`${base}/attachments`, {
      multipart: { file: { name: "e2e-slides.txt", mimeType: "text/plain", buffer: Buffer.from(marker) } },
    });
    expect(uploaded.status()).toBe(200);
    const file = await uploaded.json();
    expect(file.friendlyName).toBe("e2e-slides.txt");
    expect(file.type).toBe(2);

    try {
      expect(filesHolding(massStorage, marker)).toHaveLength(1);
    } finally {
      expect((await prof1.delete(`${base}/attachments/${file.id}`)).status()).toBe(200);
    }
    expect(filesHolding(massStorage, marker)).toHaveLength(0);
    // Gone, so a second delete has nothing to find.
    expect((await prof1.delete(`${base}/attachments/${file.id}`)).status()).toBe(404);
  });
});

test.describe("a lecture's custom thumbnail", () => {
  test("is uploaded and deleted with its file", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const marker = `e2e thumbnail ${Date.now()}`;

    const uploaded = await prof1.post(`${base}/thumbnail`, {
      multipart: { file: { name: "e2e-cover.png", mimeType: "image/png", buffer: Buffer.from(marker) } },
    });
    expect(uploaded.status()).toBe(200);
    expect((await uploaded.json()).id).toBeGreaterThan(0);

    try {
      expect(filesHolding(massStorage, marker)).toHaveLength(1);
      // Not an attachment, so not deletable as one.
      const id = (await uploaded.json()).id;
      expect((await prof1.delete(`${base}/attachments/${id}`)).status()).toBe(404);
    } finally {
      expect((await prof1.delete(`${base}/thumbnail`)).status()).toBe(200);
    }
    expect(filesHolding(massStorage, marker)).toHaveLength(0);
    expect((await prof1.delete(`${base}/thumbnail`)).status()).toBe(404);
  });

  test("must be an image", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const response = await prof1.post(`${base}/thumbnail`, {
      multipart: { file: { name: "e2e-cover.pdf", mimeType: "application/pdf", buffer: Buffer.from("x") } },
    });
    expect(response.status()).toBe(400);
  });
});

test.describe("subtitles and transcoding progress", () => {
  test("the progress of a finished lecture lists nothing still transcoding", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const response = await prof1.get(`${base}/transcoding-progress`);
    expect(response.status()).toBe(200);
    expect((await response.json()).progresses ?? []).toEqual([]);
  });

  test("a subtitle request answers 503 with the voice service down", async ({ playwright }) => {
    // config.yaml names a voice service on localhost that nothing serves.
    const prof1 = await apiAs(playwright, users.prof1);
    expect((await prof1.post(`${base}/subtitles`, { data: { language: "de" } })).status()).toBe(503);
    expect((await prof1.post(`${base}/subtitles`, { data: { language: "german" } })).status()).toBe(400);
  });
});

test.describe("the lecture content endpoints", () => {
  test("refuse another course's lecture through one's own", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);

    expect(
      (await prof1.post(`${foreign}/sections`, { data: { sections: [{ description: "Hijacked" }] } })).status(),
    ).toBe(404);
    expect((await prof1.put(`${foreign}/sections/1`, { data: { description: "Hijacked" } })).status()).toBe(404);
    expect((await prof1.delete(`${foreign}/sections/1`)).status()).toBe(404);
    expect((await prof1.delete(`${foreign}/attachments/1`)).status()).toBe(404);
    expect((await prof1.delete(`${foreign}/thumbnail`)).status()).toBe(404);
    expect((await prof1.post(`${foreign}/subtitles`, { data: { language: "de" } })).status()).toBe(404);
    expect((await prof1.get(`${foreign}/transcoding-progress`)).status()).toBe(404);
    for (const upload of ["attachments", "thumbnail"]) {
      const response = await prof1.post(`${foreign}/${upload}`, {
        multipart: { file: { name: "hijack.png", mimeType: "image/png", buffer: Buffer.from("hijack") } },
      });
      expect(response.status()).toBe(404);
    }
    expect(filesHolding(massStorage, "hijack")).toHaveLength(0);

    const prof2 = await apiAs(playwright, users.prof2);
    const stream7 = await prof2.get("/api/v2/streams/games101/7/sections");
    expect(stream7.status()).toBe(200);
    expect(((await stream7.json()).sections ?? []).map((s: Section) => s.description)).not.toContain("Hijacked");
  });

  test("answer 404 to a student", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    expect((await studi1.post(`${base}/sections`, { data: { sections: [{ description: "x" }] } })).status()).toBe(
      404,
    );
    expect((await studi1.get(`${base}/transcoding-progress`)).status()).toBe(404);
    const upload = await studi1.post(`${base}/attachments`, {
      multipart: { file: { name: "x.txt", mimeType: "text/plain", buffer: Buffer.from("studi upload") } },
    });
    expect(upload.status()).toBe(404);
    expect(filesHolding(massStorage, "studi upload")).toHaveLength(0);
  });

  test("answer 401 to an anonymous caller", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    expect((await anonymous.get(`${base}/transcoding-progress`)).status()).toBe(401);
    expect((await anonymous.delete(`${base}/thumbnail`)).status()).toBe(401);
    const upload = await anonymous.post(`${base}/attachments`, {
      multipart: { file: { name: "x.txt", mimeType: "text/plain", buffer: Buffer.from("anonymous upload") } },
    });
    expect(upload.status()).toBe(401);
  });
});
