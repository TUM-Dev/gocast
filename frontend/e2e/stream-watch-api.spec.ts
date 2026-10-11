import { expect, test } from "@playwright/test";

import { apiAs } from "./helpers";
import { users } from "./seed";

/**
 * The watch page's RPCs, API only: the page that calls them is built on top.
 *
 * Read-only on purpose, so the file can run beside the others: what writes (a
 * reaction, a seek, a report to the alert room, ending a stream) is covered by the Go
 * tests with mocked backends, and the fixture has no alert room, no durations and no
 * runner to write to. Streams: 1 is a recording in the public brauereiwesen (no lecture
 * hall), 11 a private one there, 9 a recording in the enrolled-only bierkunde (studi1
 * is enrolled, studi2 is not), 10 is live in a hall in the hidden course geheim (prof2
 * administers it).
 */

const watch = (id: number, slug: string) => `/api/v2/streams/${slug}/${id}/watch`;

test.describe("getStreamWatch", () => {
  test("answers an anonymous viewer of a public recording without anything personal", async ({ playwright }) => {
    const response = await (await apiAs(playwright)).get(watch(1, "brauereiwesen"));
    expect(response.status()).toBe(200);
    const page = await response.json();

    expect(page.stream).toMatchObject({ id: 1, displayName: "VL 1: Was ist Bier?", recording: true, liveNow: false });
    expect(page.course).toMatchObject({ slug: "brauereiwesen", year: 2022, term: "S" });
    expect(page.views.map((v: { kind: string }) => v.kind)).toEqual([
      "WATCH_VIEW_KIND_COMBINED",
      "WATCH_VIEW_KIND_PRESENTATION",
      "WATCH_VIEW_KIND_CAMERA",
    ]);
    expect(page.views[0].playlistUrl).toContain("playlist.m3u8");
    expect(page.splitAvailable).toBe(true);
    expect(page.defaultLayout).toBe("WATCH_LAYOUT_COMBINED");
    expect(page.viewer.signedIn ?? false).toBe(false);
    expect(page.viewer.canReportIssue ?? false).toBe(false);
    for (const absent of ["progress", "admin", "startOffset", "endOffset", "firstSilenceEndProgress"]) {
      expect(page[absent]).toBeUndefined();
    }
    // The gateway writes an empty list as [].
    expect(page.downloads ?? []).toEqual([]);
  });

  test("refuses a slug that is not the stream's course's", async ({ playwright }) => {
    expect((await (await apiAs(playwright)).get(watch(1, "games101"))).status()).toBe(400);
  });

  test("keeps a private lecture from everyone but its course's administrators", async ({ playwright }) => {
    expect((await (await apiAs(playwright)).get(watch(11, "brauereiwesen"))).status()).toBe(403);
    expect((await (await apiAs(playwright, users.studi1)).get(watch(11, "brauereiwesen"))).status()).toBe(403);

    const response = await (await apiAs(playwright, users.prof1)).get(watch(11, "brauereiwesen"));
    expect(response.status()).toBe(200);
    expect((await response.json()).stream.isPrivate).toBe(true);
  });

  test("follows the enrolment of an enrolled-only course, and offers downloads to a signed-in viewer", async ({
    playwright,
  }) => {
    expect((await (await apiAs(playwright)).get(watch(9, "bierkunde"))).status()).toBe(403);
    expect((await (await apiAs(playwright, users.studi2)).get(watch(9, "bierkunde"))).status()).toBe(403);

    const response = await (await apiAs(playwright, users.studi1)).get(watch(9, "bierkunde"));
    expect(response.status()).toBe(200);
    const page = await response.json();
    expect(page.viewer.signedIn).toBe(true);
    expect(page.viewer.isCourseAdmin ?? false).toBe(false);
    // The course allows downloads.
    expect(page.downloads.map((d: { friendlyName: string }) => d.friendlyName)).toEqual([
      "Combined",
      "Presentation",
      "Camera",
    ]);
    expect(page.downloads[0].url).toContain("&download=1");
    // A signed-in viewer of a recording has a progress, even before watching.
    expect(page.progress).toMatchObject({ streamId: 9 });
    expect(page.admin).toBeUndefined();
  });

  test("tells a course's administrator what they can do to a live stream", async ({ playwright }) => {
    const response = await (await apiAs(playwright, users.prof2)).get(watch(10, "geheim"));
    expect(response.status()).toBe(200);
    const page = await response.json();
    expect(page.stream.liveNow).toBe(true);
    expect(page.stream.lectureHallName).toBeTruthy();
    expect(page.viewer.isCourseAdmin).toBe(true);
    expect(page.admin).toMatchObject({ canEndStream: true });
    expect(page.admin.lectureHallId).toBeGreaterThan(0);
    expect(page.downloads ?? []).toEqual([]);
    expect(page.silences ?? []).toEqual([]);

    // The hidden course may be watched by anyone signed in who has its link, without
    // the controls.
    const student = await (await apiAs(playwright, users.studi1)).get(watch(10, "geheim"));
    expect(student.status()).toBe(200);
    const asStudent = await student.json();
    expect(asStudent.admin).toBeUndefined();
    expect(asStudent.stream.liveNow).toBe(true);
  });
});

test.describe("reactions", () => {
  test("list for whoever may watch, and need a login to add", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    const list = await anonymous.get("/api/v2/streams/1/reactions/allowed");
    expect(list.status()).toBe(200);
    expect((await list.json()).reactions ?? []).toEqual(expect.any(Array));
    expect((await (await apiAs(playwright, users.studi3)).get("/api/v2/streams/9/reactions/allowed")).status()).toBe(403);

    expect((await anonymous.post("/api/v2/streams/1/reactions", { data: { reaction: "👍" } })).status()).toBe(401);
  });

  test("refuse an emoji the server does not list", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    expect((await studi1.post("/api/v2/streams/1/reactions", { data: { reaction: "not-an-emoji" } })).status()).toBe(400);
  });
});

test.describe("problem reports", () => {
  // Never sent for real here: the development config may name a live alert room. The
  // refusals below all come before anything is sent.
  test("are for signed-in viewers and need a description", async ({ playwright }) => {
    const body = { comment: "no sound", categories: ["STREAM_ISSUE_CATEGORY_AUDIO"] };
    expect((await (await apiAs(playwright)).post("/api/v2/streams/1/issues", { data: body })).status()).toBe(401);

    const studi1 = await apiAs(playwright, users.studi1);
    expect((await studi1.post("/api/v2/streams/1/issues", { data: { ...body, comment: "  " } })).status()).toBe(400);
    expect((await (await apiAs(playwright, users.studi3)).post("/api/v2/streams/9/issues", { data: body })).status()).toBe(403);
  });
});

test.describe("seeks", () => {
  test("refuse a position the recording does not have", async ({ playwright }) => {
    const studi1 = await apiAs(playwright, users.studi1);
    // The fixture's recordings have no processed duration.
    expect((await studi1.post("/api/v2/streams/1/seeks", { data: { position: 5 } })).status()).toBe(400);
    expect((await studi1.post("/api/v2/streams/1/seeks", { data: { position: -1 } })).status()).toBe(400);
    expect((await (await apiAs(playwright, users.studi3)).post("/api/v2/streams/9/seeks", { data: { position: 1 } })).status()).toBe(
      403,
    );
  });

  test("have an empty heatmap until there are enough of them", async ({ playwright }) => {
    const response = await (await apiAs(playwright)).get("/api/v2/streams/1/seeks/heatmap");
    expect(response.status()).toBe(200);
    const heatmap = await response.json();
    expect(heatmap.bucketCount).toBe(150);
    expect(heatmap.buckets ?? []).toEqual([]);
    expect((await (await apiAs(playwright)).get("/api/v2/streams/11/seeks/heatmap")).status()).toBe(403);
  });
});

test.describe("subtitle search", () => {
  test("needs three characters and tolerates Meilisearch being away", async ({ playwright }) => {
    const anonymous = await apiAs(playwright);
    expect((await anonymous.get("/api/v2/streams/1/subtitles/search", { params: { query: "ab" } })).status()).toBe(400);
    // 503 where Meilisearch is unreachable, as it is locally.
    expect([200, 503]).toContain(
      (await anonymous.get("/api/v2/streams/1/subtitles/search", { params: { query: "bier" } })).status(),
    );
    expect(
      (await anonymous.get("/api/v2/streams/11/subtitles/search", { params: { query: "bier" } })).status(),
    ).toBe(403);
  });
});

test.describe("endStream", () => {
  test("is for the course's administrators, and for its live streams", async ({ playwright }) => {
    const end = { data: {} };
    expect((await (await apiAs(playwright)).post("/api/v2/courses/1/streams/1/end", end)).status()).toBe(401);
    expect((await (await apiAs(playwright, users.studi1)).post("/api/v2/courses/1/streams/1/end", end)).status()).toBe(404);

    const prof1 = await apiAs(playwright, users.prof1);
    // A recording is not live; the live stream 10 belongs to another course.
    expect((await prof1.post("/api/v2/courses/1/streams/1/end", end)).status()).toBe(400);
    expect((await prof1.post("/api/v2/courses/1/streams/10/end", end)).status()).toBe(404);
  });
});
