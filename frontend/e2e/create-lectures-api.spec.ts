import { type APIRequestContext, expect, test } from "@playwright/test";

import { apiAs } from "./helpers";
import { users } from "./seed";

/**
 * createLectures and the VOD media upload beside it, API only: the wizard that will
 * use them comes later.
 *
 * Einführung Brauereiwesen (course 1) is administered by prof1 and prof2; Praktikum:
 * Golang (course 3) by prof1 alone. Every lecture created here is deleted again in a
 * finally, and dated in 1234, the test year, so nothing another spec lists by date
 * can meet one even if a run is cut short.
 */

// Timestamps are written as protojson answers them (no fractional seconds), so the
// answers compare as strings.
interface CreatedLecture {
  id: number;
  courseId: number;
  name: string;
  start: string;
  end: string;
  seriesIdentifier?: string;
  streamKey?: string;
  lectureHallId?: number;
}

async function listed(
  context: APIRequestContext,
  courseId: number,
): Promise<CreatedLecture[]> {
  const response = await context.get(
    `/api/v2/courses/${courseId}/lectures/admin`,
  );
  expect(response.status()).toBe(200);
  return (await response.json()).lectures ?? [];
}

async function create(
  context: APIRequestContext,
  courseId: number,
  data: object,
) {
  return context.post(`/api/v2/courses/${courseId}/streams`, { data });
}

async function deleteAll(
  context: APIRequestContext,
  courseId: number,
  ids: number[],
): Promise<void> {
  if (ids.length === 0) return;
  const response = await context.post(
    `/api/v2/courses/${courseId}/streams/delete`,
    { data: { streamIds: ids } },
  );
  expect(response.status()).toBe(200);
}

test.describe("creating lectures", () => {
  test("creates a single self-streamed lecture", async ({ playwright }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const created: number[] = [];
    try {
      const response = await create(prof1, 1, {
        title: "E2E: Einzeltermin",
        start: "1234-05-02T08:15:00Z",
        durationMinutes: 90,
        chatEnabled: true,
      });
      expect(response.status()).toBe(200);
      const lectures: CreatedLecture[] = (await response.json()).lectures;
      created.push(...lectures.map((l) => l.id));

      expect(lectures).toHaveLength(1);
      expect(lectures[0].name).toBe("E2E: Einzeltermin");
      expect(lectures[0].end).toBe("1234-05-02T09:45:00Z");
      expect(lectures[0].streamKey).toBeTruthy();
      expect(lectures[0].seriesIdentifier ?? "").toBe("");

      const found = (await listed(prof1, 1)).find(
        (l) => l.id === lectures[0].id,
      );
      expect(found?.name).toBe("E2E: Einzeltermin");
      expect(found?.start).toBe("1234-05-02T08:15:00Z");
    } finally {
      await deleteAll(prof1, 1, created);
    }
    expect((await listed(prof1, 1)).map((l) => l.id)).not.toContain(created[0]);
  });

  test("creates a weekly series of three under one series identifier", async ({
    playwright,
  }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const created: number[] = [];
    try {
      const response = await create(prof1, 1, {
        title: "E2E: Reihe",
        start: "1234-05-02T10:00:00Z",
        durationMinutes: 45,
        dateSeries: ["1234-05-09T10:00:00Z", "1234-05-16T10:00:00Z"],
      });
      expect(response.status()).toBe(200);
      const lectures: CreatedLecture[] = (await response.json()).lectures;
      created.push(...lectures.map((l) => l.id));
      expect(lectures).toHaveLength(3);

      const ids = new Set(created);
      const series = (await listed(prof1, 1)).filter((l) => ids.has(l.id));
      expect(series).toHaveLength(3);
      expect(series.map((l) => l.start).sort()).toEqual([
        "1234-05-02T10:00:00Z",
        "1234-05-09T10:00:00Z",
        "1234-05-16T10:00:00Z",
      ]);
      expect(series[0].seriesIdentifier).toBeTruthy();
      expect(new Set(series.map((l) => l.seriesIdentifier)).size).toBe(1);
    } finally {
      await deleteAll(prof1, 1, created);
    }
    const remaining = new Set((await listed(prof1, 1)).map((l) => l.id));
    expect(created.filter((id) => remaining.has(id))).toEqual([]);
  });

  test("refuses a hall for a VOD upload and a missing title", async ({
    playwright,
  }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const before = (await listed(prof1, 1)).length;

    const vodInHall = await create(prof1, 1, {
      title: "E2E: VoD im Hörsaal",
      kind: "LECTURE_CREATION_KIND_VOD_UPLOAD",
      start: "1234-05-02T08:15:00Z",
      lectureHallId: 1,
    });
    expect(vodInHall.status()).toBe(400);
    const untitled = await create(prof1, 1, {
      title: " ",
      start: "1234-05-02T08:15:00Z",
      durationMinutes: 90,
    });
    expect(untitled.status()).toBe(400);

    expect((await listed(prof1, 1)).length).toBe(before);
  });

  test("answers 404 to a student and to a lecturer of other courses", async ({
    playwright,
  }) => {
    const data = {
      title: "E2E: Fremd",
      start: "1234-05-02T08:15:00Z",
      durationMinutes: 90,
    };

    const studi1 = await apiAs(playwright, users.studi1);
    expect((await create(studi1, 1, data)).status()).toBe(404);

    // prof2 administers course 1 too, but not Praktikum: Golang.
    const prof2 = await apiAs(playwright, users.prof2);
    expect((await create(prof2, 3, data)).status()).toBe(404);

    const anonymous = await apiAs(playwright);
    expect((await create(anonymous, 1, data)).status()).toBe(401);

    const prof1 = await apiAs(playwright, users.prof1);
    expect((await listed(prof1, 3)).map((l) => l.name)).not.toContain(
      "E2E: Fremd",
    );
    expect((await listed(prof1, 1)).map((l) => l.name)).not.toContain(
      "E2E: Fremd",
    );
  });
});

test.describe("uploading a VOD's recording", () => {
  // The fixture's workers are dead (no heartbeat), so the upload has nowhere to go.
  test("answers 503 when no worker is alive, and 404 to a non-administrator", async ({
    playwright,
  }) => {
    const prof1 = await apiAs(playwright, users.prof1);
    const created: number[] = [];
    try {
      const response = await create(prof1, 1, {
        title: "E2E: Aufzeichnung",
        kind: "LECTURE_CREATION_KIND_VOD_UPLOAD",
        start: "1234-05-02T08:15:00Z",
      });
      expect(response.status()).toBe(200);
      const [lecture]: CreatedLecture[] = (await response.json()).lectures;
      created.push(lecture.id);
      expect(lecture.streamKey ?? "").toBe("");
      expect(lecture.end).toBe("1234-05-02T09:15:00Z");

      const media = `/api/v2/courses/1/streams/${lecture.id}/media`;
      const file = {
        name: "lecture.mp4",
        mimeType: "video/mp4",
        buffer: Buffer.from("not really a video"),
      };

      const noWorker = await prof1.post(`${media}?type=COMB`, {
        multipart: { file },
      });
      expect(noWorker.status()).toBe(503);
      expect((await noWorker.json()).message).toContain("no worker");

      expect(
        (
          await prof1.post(`${media}?type=SCREEN`, { multipart: { file } })
        ).status(),
      ).toBe(400);

      const studi1 = await apiAs(playwright, users.studi1);
      expect(
        (
          await studi1.post(`${media}?type=COMB`, { multipart: { file } })
        ).status(),
      ).toBe(404);
    } finally {
      await deleteAll(prof1, 1, created);
    }
  });
});
