import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { SELF_STREAMED, fetchSchedule, updateLecture } from "./schedule";

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

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

const from = new Date("2026-10-05T00:00:00Z");
const to = new Date("2026-10-12T00:00:00Z");

describe("fetchSchedule", () => {
  it("sends the window and each hall, self-streaming as 0", async () => {
    respondWith({});

    await fetchSchedule(from, to, [SELF_STREAMED, 3]);

    const url = new URL(lastCall()[0], "http://x");
    expect(url.pathname).toBe("/api/v2/schedule");
    expect(url.searchParams.get("from")).toBe("2026-10-05T00:00:00.000Z");
    expect(url.searchParams.get("to")).toBe("2026-10-12T00:00:00.000Z");
    expect(url.searchParams.getAll("lectureHallIds")).toEqual(["0", "3"]);
    expect(url.searchParams.has("allLectureHalls")).toBe(false);
  });

  it("asks for every hall without listing them", async () => {
    respondWith({});

    await fetchSchedule(from, to, "all");

    const url = new URL(lastCall()[0], "http://x");
    expect(url.searchParams.get("allLectureHalls")).toBe("true");
    expect(url.searchParams.getAll("lectureHallIds")).toEqual([]);
  });

  it("reads the lectures, with a self-streamed one's hall left out", async () => {
    respondWith({
      lectures: [
        {
          streamId: 12,
          courseId: 1,
          courseName: "Einführung Brauereiwesen",
          name: "VL 5",
          start: "2026-10-05T21:45:00Z",
          end: "2026-10-05T21:59:00Z",
        },
      ],
    });

    const [lecture] = await fetchSchedule(from, to, "all");

    expect(lecture.streamId).toBe(12);
    expect(lecture.start.toISOString()).toBe("2026-10-05T21:45:00.000Z");
    expect(lecture.lectureHallId).toBe(0);
    expect(lecture.lectureHallName).toBe("");
    expect(lecture.description).toBe("");
  });
});

describe("updateLecture", () => {
  it("sends only what changed", async () => {
    respondWith({});

    await updateLecture(1, 12, { name: "Hopfen" });

    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/1/streams/12");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(String(init.body))).toEqual({ name: "Hopfen" });
  });
});
