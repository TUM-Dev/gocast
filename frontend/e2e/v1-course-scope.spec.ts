import { expect, test } from "@playwright/test";

import { apiAs } from "./helpers";
import { users } from "./seed";

/**
 * The v1 course routes, `/api/course/:courseID/...`, check that the caller administers
 * the course in the URL; the stream a route names has to belong to that course too, or
 * an administrator of one course can edit another's lectures through their own.
 *
 * The fixture rows it leans on:
 *   - prof1 administers course 1 (Einführung Brauereiwesen) but not course 2
 *     (Spieleentwicklung); prof2 administers both;
 *   - stream 7 is course 2's live lecture, "VL 1: Livestream";
 *   - stream 1 is course 1's.
 * Every refused request is refused before anything is written, and the rename test
 * checks that, so the file needs no reload: the start-page specs rely on that name.
 */

const foreignStream = 7;
const foreignStreamName = "VL 1: Livestream";

test.describe("a course's v1 routes and another course's stream", () => {
  test("cannot rename it", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);

    // renameLecture binds with c.Bind, which reads a JSON body by its content type.
    const response = await prof1.post(`/api/course/1/renameLecture/${foreignStream}`, {
      data: { name: "Hijacked" },
    });

    expect(response.status()).toBe(404);
    expect((await response.json()).message).toBe("can not find stream");

    // Read back by an administrator of the stream's own course.
    const prof2 = await apiAs(playwright, users.prof2);
    const lectures = await prof2.get("/api/course/2/lectures");
    expect(lectures.status()).toBe(200);
    const { streams } = (await lectures.json()) as { streams: { lectureId: number; name: string }[] };
    expect(streams.find((s) => s.lectureId === foreignStream)?.name).toBe(foreignStreamName);
  });

  test("cannot read its transcoding progress", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);

    // This one goes through the InitStream middleware rather than the handler.
    const response = await prof1.get(`/api/course/1/stream/${foreignStream}/transcodingProgress`);

    expect(response.status()).toBe(404);
  });

  test("cannot read its statistics", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);

    const response = await prof1.get(`/api/course/1/stats?interval=liveViews&lecture=${foreignStream}`);

    expect(response.status()).toBe(404);
    expect((await response.json()).message).toBe("can not find stream");
  });
});

test.describe("a course's v1 routes and its own stream", () => {
  test("answer statistics to an administrator of the stream's course", async ({ playwright }) => {
    const prof2 = await apiAs(playwright, users.prof2);

    const response = await prof2.get(`/api/course/2/stats?interval=liveViews&lecture=${foreignStream}`);

    expect(response.status()).toBe(200);
  });

  test("answer statistics for a lecture of the course in the URL", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);

    const response = await prof1.get("/api/course/1/stats?interval=liveViews&lecture=1");

    expect(response.status()).toBe(200);
  });
});
