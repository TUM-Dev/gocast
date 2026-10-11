import { timestampDate } from "@bufbuild/protobuf/wkt";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { LectureCreationKind } from "@/gen/server/apiv2_pb";
import { ApiError } from "./api";
import {
  MAX_LECTURES,
  adHocEnd,
  buildCreatePlan,
  clampCount,
  createErrorMessage,
  detailsProblems,
  durationMinutes,
  emptyForm,
  filesProblems,
  formatMinutes,
  lectureCount,
  mediaUploadErrorMessage,
  seriesDate,
  seriesRows,
  type CreateLectureForm,
} from "./create-lecture";
import { fromDateTimeLocal, toDateTimeLocal } from "./datetime-local";

/* In a zone with daylight saving, so the DST cases mean something. */
let previousTz: string | undefined;
/** Set once the zone is, since a Date made from local fields depends on it. */
let NOW: Date;

beforeAll(() => {
  previousTz = process.env.TZ;
  process.env.TZ = "Europe/Berlin";
  NOW = new Date(2026, 9, 5, 12, 0);
});

afterAll(() => {
  if (previousTz === undefined) delete process.env.TZ;
  else process.env.TZ = previousTz;
});

const local = (d: Date) => toDateTimeLocal(d);

function form(overrides: Partial<CreateLectureForm>): CreateLectureForm {
  return { ...emptyForm(true), title: "Bierkunde", ...overrides };
}

function series(start: string, count: number, overrides: Partial<CreateLectureForm> = {}): CreateLectureForm {
  const f = form({ start, end: start.replace(/T10:00$/, "T11:30"), recurring: true, count, ...overrides });
  f.rows = seriesRows(fromDateTimeLocal(start)!, f.interval, count);
  return f;
}

describe("seriesDate", () => {
  it("keeps the time of day across the autumn daylight-saving change", () => {
    // Berlin leaves summer time on 25 October 2026.
    const first = new Date(2026, 9, 20, 10, 0);
    const next = seriesDate(first, "weekly", 1);
    expect(local(next)).toBe("2026-10-27T10:00");
    // Which is 7 days and one hour later in absolute time.
    expect(next.getTime() - first.getTime()).toBe((7 * 24 + 1) * 3_600_000);
  });

  it("keeps the time of day across the spring change, day by day", () => {
    const first = new Date(2026, 2, 28, 8, 15);
    expect([0, 1, 2].map((n) => local(seriesDate(first, "daily", n)))).toEqual([
      "2026-03-28T08:15",
      "2026-03-29T08:15",
      "2026-03-30T08:15",
    ]);
  });

  it("steps weekly over a month and a year end", () => {
    const first = new Date(2026, 11, 22, 14, 0);
    expect(local(seriesDate(first, "weekly", 2))).toBe("2027-01-05T14:00");
  });

  it("puts a monthly series on the 31st on the last day of shorter months, and back", () => {
    const first = new Date(2026, 0, 31, 9, 0);
    expect([1, 2, 3, 4].map((n) => local(seriesDate(first, "monthly", n)))).toEqual([
      "2026-02-28T09:00",
      "2026-03-31T09:00",
      "2026-04-30T09:00",
      "2026-05-31T09:00",
    ]);
  });

  it("finds 29 February in a leap year", () => {
    expect(local(seriesDate(new Date(2028, 0, 30, 9, 0), "monthly", 1))).toBe("2028-02-29T09:00");
  });
});

describe("seriesRows", () => {
  it("makes count rows, the first being the start", () => {
    const rows = seriesRows(new Date(2026, 9, 6, 10, 0), "weekly", 3);
    expect(rows.map((r) => local(r.start))).toEqual(["2026-10-06T10:00", "2026-10-13T10:00", "2026-10-20T10:00"]);
    expect(rows.every((r) => r.enabled && r.title === "")).toBe(true);
  });

  it("keeps what was typed into a row when the list is rebuilt", () => {
    const before = seriesRows(new Date(2026, 9, 6, 10, 0), "weekly", 3);
    before[1].title = "Exkursion";
    before[2].enabled = false;
    const after = seriesRows(new Date(2026, 9, 7, 10, 0), "weekly", 4, before);
    expect(after.map((r) => [local(r.start), r.title, r.enabled])).toEqual([
      ["2026-10-07T10:00", "", true],
      ["2026-10-14T10:00", "Exkursion", true],
      ["2026-10-21T10:00", "", false],
      ["2026-10-28T10:00", "", true],
    ]);
  });

  it("never disables the first lecture", () => {
    const before = seriesRows(new Date(2026, 9, 6, 10, 0), "weekly", 2);
    before[0].enabled = false;
    expect(seriesRows(new Date(2026, 9, 6, 10, 0), "weekly", 2, before)[0].enabled).toBe(true);
  });

  it("is bounded by the server's limit", () => {
    expect(seriesRows(new Date(2026, 9, 6, 10, 0), "daily", 1000)).toHaveLength(MAX_LECTURES);
    expect(clampCount(0)).toBe(1);
    expect(clampCount(Number.NaN)).toBe(1);
    expect(clampCount(3.7)).toBe(3);
  });
});

describe("adHocEnd and durationMinutes", () => {
  it("ends today, or tomorrow once the time has passed", () => {
    expect(local(adHocEnd("13:30", NOW)!)).toBe("2026-10-05T13:30");
    expect(local(adHocEnd("11:00", NOW)!)).toBe("2026-10-06T11:00");
    expect(adHocEnd("25:00", NOW)).toBeNull();
    expect(adHocEnd("", NOW)).toBeNull();
  });

  it("counts an ad-hoc livestream from when the server starts it", () => {
    expect(durationMinutes(form({ mode: "adhoc", endTime: "13:30" }), NOW)).toBe(88);
  });

  it("is the gap between start and end for a scheduled livestream", () => {
    expect(durationMinutes(form({ start: "2026-10-06T10:00", end: "2026-10-06T11:30" }), NOW)).toBe(90);
    expect(durationMinutes(form({ start: "2026-10-06T10:00", end: "" }), NOW)).toBeNull();
  });

  it("is 0 for a VOD upload, the server's placeholder", () => {
    expect(durationMinutes(form({ mode: "vod", start: "2026-10-06T10:00" }), NOW)).toBe(0);
  });

  it("formats", () => {
    expect(formatMinutes(90)).toBe("1h 30min");
    expect(formatMinutes(45)).toBe("45min");
    expect(formatMinutes(120)).toBe("2h");
    expect(formatMinutes(0)).toBe("");
  });
});

describe("detailsProblems", () => {
  it("wants a title, a start and an end after it", () => {
    expect(detailsProblems(form({ title: " " }), NOW)).toEqual(["Enter a title.", "Enter a start.", "Enter an end."]);
    expect(detailsProblems(form({ start: "2026-10-06T10:00", end: "2026-10-06T10:00" }), NOW)).toEqual([
      "The end must be after the start.",
    ]);
    expect(detailsProblems(form({ start: "2026-10-06T10:00", end: "2026-10-06T11:00" }), NOW)).toEqual([]);
  });

  it("wants only a start for a VOD upload", () => {
    expect(detailsProblems(form({ mode: "vod", start: "2026-10-06T10:00" }), NOW)).toEqual([]);
  });

  it("wants an end not right now for an ad-hoc livestream", () => {
    expect(detailsProblems(form({ mode: "adhoc" }), NOW)).toEqual(["Enter when the livestream ends."]);
    expect(detailsProblems(form({ mode: "adhoc", endTime: "12:01" }), NOW)).toEqual([
      "The end must be a few minutes from now.",
    ]);
    expect(detailsProblems(form({ mode: "adhoc", endTime: "13:00" }), NOW)).toEqual([]);
  });
});

describe("filesProblems", () => {
  const mp4 = new File(["x"], "lecture.mp4", { type: "video/mp4" });
  const mov = new File(["x"], "lecture.mov", { type: "video/quicktime" });

  it("wants at least one mp4", () => {
    expect(filesProblems(form({ mode: "vod" }))).toEqual(["Choose at least one video."]);
    expect(filesProblems(form({ mode: "vod", files: { PRES: mp4 } }))).toEqual([]);
    expect(filesProblems(form({ mode: "vod", files: { COMB: mp4, CAM: mov } }))).toEqual([
      "The camera video must be an mp4 file.",
    ]);
  });
});

describe("buildCreatePlan", () => {
  it("sends a single scheduled livestream with its duration and hall", () => {
    const { request, titles } = buildCreatePlan(
      1,
      form({ start: "2026-10-06T10:00", end: "2026-10-06T11:30", lectureHallId: 3 }),
      NOW,
    );
    expect(request.courseId).toBe(1);
    expect(request.title).toBe("Bierkunde");
    expect(request.kind).toBe(LectureCreationKind.LIVESTREAM);
    expect(request.durationMinutes).toBe(90);
    expect(request.lectureHallId).toBe(3);
    expect(request.chatEnabled).toBe(true);
    expect(request.adHoc).toBe(false);
    expect(timestampDate(request.start!).toISOString()).toBe("2026-10-06T08:00:00.000Z");
    expect(request.dateSeries).toEqual([]);
    expect(titles).toEqual([""]);
  });

  it("puts only the further dates of a series in date_series", () => {
    const { request } = buildCreatePlan(1, series("2026-10-20T10:00", 3), NOW);
    expect(timestampDate(request.start!).toISOString()).toBe("2026-10-20T08:00:00.000Z");
    // After the change to winter time, 10:00 in Berlin is 09:00 UTC.
    expect(request.dateSeries.map((t) => timestampDate(t).toISOString())).toEqual([
      "2026-10-27T09:00:00.000Z",
      "2026-11-03T09:00:00.000Z",
    ]);
  });

  it("leaves out the dates switched off, and keeps each lecture's own title in order", () => {
    const f = series("2026-10-20T10:00", 4);
    f.rows[0].title = "Einführung";
    f.rows[1].enabled = false;
    f.rows[1].title = "Ausgefallen";
    f.rows[3].title = "Exkursion";
    f.rows[2].title = "Bierkunde"; // the series' own title is no title of its own
    const { request, titles } = buildCreatePlan(1, f, NOW);
    expect(request.dateSeries).toHaveLength(2);
    expect(titles).toEqual(["Einführung", "", "Exkursion"]);
    expect(lectureCount(f)).toBe(3);
  });

  it("ignores the series list when not recurring", () => {
    const f = series("2026-10-20T10:00", 4, { recurring: false });
    expect(buildCreatePlan(1, f, NOW).request.dateSeries).toEqual([]);
    expect(lectureCount(f)).toBe(1);
  });

  it("sends an ad-hoc livestream without a start or a series", () => {
    const { request, titles } = buildCreatePlan(1, form({ mode: "adhoc", endTime: "13:30", start: "x" }), NOW);
    expect(request.adHoc).toBe(true);
    expect(request.start).toBeUndefined();
    expect(request.dateSeries).toEqual([]);
    expect(request.durationMinutes).toBe(88);
    expect(titles).toEqual([""]);
  });

  it("sends a VOD upload without a hall or a duration", () => {
    const { request } = buildCreatePlan(
      1,
      form({ mode: "vod", start: "2026-10-06T10:00", end: "2026-10-06T12:00", lectureHallId: 3, recurring: true }),
      NOW,
    );
    expect(request.kind).toBe(LectureCreationKind.VOD_UPLOAD);
    expect(request.lectureHallId).toBe(0);
    expect(request.durationMinutes).toBe(0);
    expect(request.dateSeries).toEqual([]);
  });

  it("trims the title", () => {
    expect(buildCreatePlan(1, form({ title: "  VL 3  ", start: "2026-10-06T10:00", end: "2026-10-06T11:00" }), NOW).request.title).toBe("VL 3");
  });
});

describe("messages", () => {
  it("passes the server's reason for a 400 on", () => {
    expect(createErrorMessage(new ApiError(400, "title is required"))).toBe(
      "The lecture could not be created: title is required.",
    );
    expect(createErrorMessage(new ApiError(404, "not found"))).toMatch(/do not administer/);
  });

  it("says the lecture exists when no worker takes the upload", () => {
    expect(mediaUploadErrorMessage(new ApiError(503, "no worker"), "combined video")).toBe(
      "No worker is available to receive the upload right now. The lecture was created; upload the video later from its card.",
    );
    expect(mediaUploadErrorMessage(new ApiError(409, "the lecture is live"), "camera video")).toBe(
      "The camera video could not be uploaded: the lecture is live. The lecture was created.",
    );
  });
});
