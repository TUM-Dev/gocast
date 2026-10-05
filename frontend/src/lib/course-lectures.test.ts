import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, clearToken } from "./api";
import {
  copyLecture,
  deleteLectureSeries,
  deleteLectures,
  fetchAdministeredCourses,
  fetchCourseLectures,
  formatDuration,
  lectureBadges,
  lectureErrorMessage,
  lectureFromHash,
  lectureState,
  loadSortAscending,
  saveSortAscending,
  seriesSize,
  sortLectures,
  updateLecture,
  updateLectureSeries,
  updateLectureSeriesTime,
  updateLecturesLectureHall,
  type CourseLecture,
} from "./course-lectures";

let fetchMock: ReturnType<typeof vi.fn>;

/** Answers the token request, then the given body for everything else. */
function respondWith(body: unknown, status = 200): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(
        new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 }),
      );
    }
    return Promise.resolve(new Response(JSON.stringify(body), { status }));
  });
}

const lastCall = () => fetchMock.mock.calls.at(-1) as [string, RequestInit];
const sentBody = () => JSON.parse(String(lastCall()[1].body));

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

describe("fetchCourseLectures", () => {
  it("reads the lectures, filling in what protojson leaves out", async () => {
    respondWith({
      lectures: [
        {
          id: 1,
          courseId: 1,
          name: "VL 1: Was ist Bier?",
          start: "2022-04-11T08:00:00Z",
          end: "2022-04-11T10:00:00Z",
          recording: true,
          vodVersions: ["COMB", "PRES"],
          durationSeconds: 5400,
          files: [{ id: 3, type: 2, friendlyName: "slides.pdf" }],
          transcodingProgresses: [{ version: "CAM", progress: 40 }],
          videoSections: [{ id: 1, description: "Intro" }],
          seriesIdentifier: "abc",
        },
      ],
    });

    const [lecture] = await fetchCourseLectures(1);

    expect(lastCall()[0]).toBe("/api/v2/courses/1/lectures/admin");
    expect(lecture.name).toBe("VL 1: Was ist Bier?");
    expect(lecture.start.toISOString()).toBe("2022-04-11T08:00:00.000Z");
    expect(lecture.lectureHallId).toBe(0);
    expect(lecture.lectureHallName).toBe("");
    expect(lecture.chatEnabled).toBe(false);
    expect(lecture.vodVersions).toEqual(["COMB", "PRES"]);
    expect(lecture.files).toEqual([{ id: 3, type: 2, friendlyName: "slides.pdf" }]);
    expect(lecture.transcodingProgresses).toEqual([{ version: "CAM", progress: 40 }]);
    expect(lecture.videoSectionCount).toBe(1);
    expect(lecture.seriesIdentifier).toBe("abc");
  });

  it("reads an empty course as no lectures", async () => {
    respondWith({});
    expect(await fetchCourseLectures(1)).toEqual([]);
  });

  it("surfaces a refusal as an ApiError", async () => {
    respondWith({ code: 5, message: "not found" }, 404);
    await expect(fetchCourseLectures(2)).rejects.toMatchObject({ status: 404 });
  });
});

describe("fetchAdministeredCourses", () => {
  it("lists the administered courses", async () => {
    respondWith({ courses: [{ id: 3, name: "Praktikum: Golang", slug: "godev", year: 2021, term: "W" }] });
    expect(await fetchAdministeredCourses()).toEqual([{ id: 3, name: "Praktikum: Golang", year: 2021, term: "W" }]);
    expect(lastCall()[0]).toBe("/api/v2/courses/administered");
  });
});

describe("updating", () => {
  it("sends only what changed, times as RFC 3339 and a hall of 0", async () => {
    respondWith({});

    await updateLecture(1, 12, {
      start: new Date("2026-10-05T21:45:00Z"),
      end: new Date("2026-10-05T22:30:00Z"),
      lectureHallId: 0,
      name: undefined,
    });

    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/12");
    expect(init.method).toBe("PATCH");
    expect(sentBody()).toEqual({
      start: "2026-10-05T21:45:00.000Z",
      end: "2026-10-05T22:30:00.000Z",
      lectureHallId: 0,
    });
  });

  it("sends false rather than dropping it", async () => {
    respondWith({});
    await updateLecture(1, 12, { chatEnabled: false, private: false });
    expect(sentBody()).toEqual({ chatEnabled: false, private: false });
  });

  it("changes a series through its own route", async () => {
    respondWith({});
    await updateLectureSeries(1, 12, { description: "Neu", chatEnabled: true });
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/12/series");
    expect(init.method).toBe("PATCH");
    expect(sentBody()).toEqual({ description: "Neu", chatEnabled: true });
  });

  it("moves a series' time with PUT", async () => {
    respondWith({});
    await updateLectureSeriesTime(1, 12, new Date("2026-10-05T08:00:00Z"), new Date("2026-10-05T10:00:00Z"));
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/12/series/time");
    expect(init.method).toBe("PUT");
    expect(sentBody()).toEqual({ start: "2026-10-05T08:00:00.000Z", end: "2026-10-05T10:00:00.000Z" });
  });
});

describe("updateLecturesLectureHall", () => {
  it("moves several lectures with one PUT, a hall of 0 included", async () => {
    respondWith({});
    await updateLecturesLectureHall(1, [4, 5], 0);
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/lecture-hall");
    expect(init.method).toBe("PUT");
    expect(sentBody()).toEqual({ streamIds: [4, 5], lectureHallId: 0 });
  });
});

describe("deleting and copying", () => {
  it("deletes several lectures in one request", async () => {
    respondWith({});
    await deleteLectures(1, [4, 5]);
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/delete");
    expect(init.method).toBe("POST");
    expect(sentBody()).toEqual({ streamIds: [4, 5] });
  });

  it("deletes a series by one of its lectures", async () => {
    respondWith({});
    await deleteLectureSeries(1, 4);
    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/4/series");
    expect(init.method).toBe("DELETE");
  });

  it("copies and answers the copy's ID", async () => {
    respondWith({ streamId: 99 });
    expect(await copyLecture(1, 4, 3, true)).toBe(99);
    const [url] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/4/copy");
    expect(sentBody()).toEqual({ targetCourseId: 3, move: true });
  });
});

function lecture(overrides: Partial<CourseLecture> = {}): CourseLecture {
  return {
    id: 1,
    courseId: 1,
    name: "VL",
    description: "",
    start: new Date("2026-10-05T08:00:00Z"),
    end: new Date("2026-10-05T10:00:00Z"),
    lectureHallId: 2,
    lectureHallName: "HS1",
    seriesIdentifier: "",
    streamKey: "k",
    chatEnabled: false,
    private: false,
    liveNow: false,
    recording: false,
    past: false,
    converting: false,
    premiere: false,
    vodVersions: [],
    durationSeconds: 0,
    files: [],
    transcodingProgresses: [],
    videoSectionCount: 0,
    ...overrides,
  };
}

const labels = (l: CourseLecture) => lectureBadges(l).map((b) => b.label);

describe("lectureState and lectureBadges", () => {
  it("calls a lecture neither live, recorded nor past scheduled", () => {
    expect(lectureState(lecture())).toBe("planned");
    expect(labels(lecture())).toEqual(["Scheduled", "Chat off"]);
  });

  it("puts live before anything else", () => {
    expect(lectureState(lecture({ liveNow: true, recording: true }))).toBe("live");
  });

  it("shows a recording as VoD, and as converting while a version transcodes", () => {
    expect(lectureState(lecture({ recording: true }))).toBe("recording");
    expect(labels(lecture({ recording: true }))[0]).toBe("VoD");
    expect(lectureState(lecture({ recording: true, converting: true }))).toBe("converting");
  });

  it("marks a past lecture without a recording", () => {
    expect(lectureState(lecture({ past: true }))).toBe("past");
    expect(lectureBadges(lecture({ past: true }))[0]).toMatchObject({ label: "Past", tone: "warn" });
  });

  it("adds private, premiere and chat beside the state", () => {
    expect(labels(lecture({ private: true, premiere: true, chatEnabled: true }))).toEqual([
      "Scheduled",
      "Private",
      "Premiere",
      "Chat on",
    ]);
  });

  it("marks self-streaming only where it is still to happen", () => {
    expect(labels(lecture({ lectureHallId: 0 }))).toContain("Self-stream");
    expect(labels(lecture({ lectureHallId: 0, liveNow: true }))).toContain("Self-stream");
    expect(labels(lecture({ lectureHallId: 0, recording: true }))).not.toContain("Self-stream");
    expect(labels(lecture({ lectureHallId: 0, past: true }))).not.toContain("Self-stream");
  });
});

describe("list helpers", () => {
  const a = lecture({ id: 1, start: new Date("2026-10-01T08:00:00Z"), seriesIdentifier: "s" });
  const b = lecture({ id: 2, start: new Date("2026-10-08T08:00:00Z"), seriesIdentifier: "s" });
  const c = lecture({ id: 3, start: new Date("2026-10-08T08:00:00Z") });

  it("sorts newest or oldest first, ties by ID", () => {
    expect(sortLectures([a, b, c], false).map((l) => l.id)).toEqual([3, 2, 1]);
    expect(sortLectures([c, b, a], true).map((l) => l.id)).toEqual([1, 2, 3]);
  });

  it("counts a series, and nothing for a lone lecture", () => {
    expect(seriesSize([a, b, c], a)).toBe(2);
    expect(seriesSize([a, b, c], c)).toBe(0);
  });

  it("formats durations", () => {
    expect(formatDuration(0)).toBe("");
    expect(formatDuration(45 * 60)).toBe("45:00");
    expect(formatDuration(3909)).toBe("1:05:09");
  });

  it("reads the lecture from the fragment, and only that shape", () => {
    expect(lectureFromHash("#lecture-12")).toBe(12);
    expect(lectureFromHash("#lecture-li-12")).toBeNull();
    expect(lectureFromHash("#lectures:12")).toBeNull();
    expect(lectureFromHash("")).toBeNull();
  });

  it("keeps the sort order where the old page's $persist did", () => {
    expect(loadSortAscending()).toBe(false);
    localStorage.setItem("_x_courseStreamsSortOrder", "true");
    expect(loadSortAscending()).toBe(true);
    saveSortAscending(false);
    expect(localStorage.getItem("_x_courseStreamsSortOrder")).toBe("false");
    localStorage.setItem("_x_courseStreamsSortOrder", "{broken");
    expect(loadSortAscending()).toBe(false);
  });
});

describe("lectureErrorMessage", () => {
  it("explains a 404 as gone or not administered", () => {
    expect(lectureErrorMessage(new ApiError(404, "not found"))).toMatch(/no longer exists or you do not administer it/);
  });

  it("explains a 403 as the hall rule, the only thing these refuse a course admin", () => {
    expect(lectureErrorMessage(new ApiError(403, "forbidden"))).toMatch(/server administrators/);
  });

  it("passes the server's message through otherwise", () => {
    expect(lectureErrorMessage(new ApiError(400, "end must be after start"))).toBe("end must be after start");
  });
});
