import { expect, test } from "@playwright/test";

import { apiAs } from "./helpers";
import { users } from "./seed";

/**
 * GET /api/v2/streams/{stream}/files/{file}, API only: no page uses it yet.
 *
 * The fixture has no files, so each test uploads its own through the lecturers'
 * attachment endpoint (unique content, deleted in a finally) and reads it back as
 * different callers. Stream 1 is on the public Einführung Brauereiwesen (prof1), stream
 * 7 on the logged-in-only Spieleentwicklung (prof2). Stream 1's course has downloads on,
 * which is why the attachment cases name that they do not depend on it.
 */

async function withAttachment(
  playwright: Parameters<typeof apiAs>[0],
  lecturer: typeof users.prof1,
  courseID: number,
  streamID: number,
  body: (file: { id: number }, marker: string) => Promise<void>,
): Promise<void> {
  const admin = await apiAs(playwright, lecturer);
  const marker = `e2e stream file ${Date.now()} ${Math.random()}`;
  const uploaded = await admin.post(`/api/v2/courses/${courseID}/streams/${streamID}/attachments`, {
    multipart: { file: { name: "e2e-stream-file.txt", mimeType: "text/plain", buffer: Buffer.from(marker) } },
  });
  expect(uploaded.status()).toBe(200);
  const file = await uploaded.json();
  try {
    await body(file, marker);
  } finally {
    expect((await admin.delete(`/api/v2/courses/${courseID}/streams/${streamID}/attachments/${file.id}`)).status()).toBe(
      200,
    );
  }
}

test.describe("a stream's files", () => {
  test("an attachment of a public course is served to anyone, as a download for text", async ({ playwright }) => {
    await withAttachment(playwright, users.prof1, 1, 1, async (file, marker) => {
      const anonymous = await apiAs(playwright);
      const response = await anonymous.get(`/api/v2/streams/1/files/${file.id}`);
      expect(response.status()).toBe(200);
      expect(await response.text()).toBe(marker);
      // text/plain is not on the inline list: uploads are never rendered on our origin.
      expect(response.headers()["content-disposition"]).toContain("attachment");
      expect(response.headers()["x-content-type-options"]).toBe("nosniff");

      const partial = await anonymous.get(`/api/v2/streams/1/files/${file.id}`, { headers: { Range: "bytes=0-2" } });
      expect(partial.status()).toBe(206);
      expect(await partial.text()).toBe(marker.slice(0, 3));
    });
  });

  test("is found only through its own stream", async ({ playwright }) => {
    await withAttachment(playwright, users.prof1, 1, 1, async (file) => {
      const studi = await apiAs(playwright, users.studi1);
      // Stream 2 is in the same course; the file is stream 1's.
      expect((await studi.get(`/api/v2/streams/2/files/${file.id}`)).status()).toBe(404);
      expect((await studi.get(`/api/v2/streams/1/files/99999999`)).status()).toBe(404);
    });
  });

  test("of a logged-in-only course need a login", async ({ playwright }) => {
    await withAttachment(playwright, users.prof2, 2, 7, async (file, marker) => {
      const anonymous = await apiAs(playwright);
      expect((await anonymous.get(`/api/v2/streams/7/files/${file.id}`)).status()).toBe(401);

      const studi = await apiAs(playwright, users.studi1);
      const response = await studi.get(`/api/v2/streams/7/files/${file.id}`);
      expect(response.status()).toBe(200);
      expect(await response.text()).toBe(marker);
    });
  });

  test("of an enrolled-only course need the enrolment", async ({ playwright }) => {
    // Fortgeschrittene Bierkunde (course 4, prof1): studi1 is enrolled, studi2 is not.
    await withAttachment(playwright, users.prof1, 4, 9, async (file, marker) => {
      expect((await (await apiAs(playwright)).get(`/api/v2/streams/9/files/${file.id}`)).status()).toBe(401);
      // Not 403: whether the file exists is not for them to learn.
      expect((await (await apiAs(playwright, users.studi2)).get(`/api/v2/streams/9/files/${file.id}`)).status()).toBe(404);

      const enrolled = await (await apiAs(playwright, users.studi1)).get(`/api/v2/streams/9/files/${file.id}`);
      expect(enrolled.status()).toBe(200);
      expect(await enrolled.text()).toBe(marker);
    });
  });
});
