import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError, clearToken } from "./api";
import {
  checkUnit,
  createLectureUnit,
  deleteLectureUnit,
  fetchLectureUnits,
  formatTimecode,
  parseTimecode,
  renderDescription,
  sortUnits,
  unitsErrorMessage,
} from "./lecture-units";

let fetchMock: ReturnType<typeof vi.fn>;

/** Answers the token request, then the given body for everything else. */
function respondWith(body: unknown, status = 200): void {
  fetchMock.mockImplementation((url: string) => {
    if (String(url).endsWith("/auth/token")) {
      return Promise.resolve(new Response(JSON.stringify({ access_token: "t", expires_in: 900 }), { status: 200 }));
    }
    return Promise.resolve(new Response(status === 204 ? null : JSON.stringify(body), { status }));
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

describe("timecodes", () => {
  it("formats whole seconds as HH:MM:SS", () => {
    expect(formatTimecode(0)).toBe("00:00:00");
    expect(formatTimecode(61_999)).toBe("00:01:01");
    expect(formatTimecode(3_723_000)).toBe("01:02:03");
    expect(formatTimecode(-5)).toBe("00:00:00");
  });

  it("parses hours, minutes and seconds, or bare seconds", () => {
    expect(parseTimecode("01:02:03")).toBe(3_723_000);
    expect(parseTimecode(" 2:03 ")).toBe(123_000);
    expect(parseTimecode("90")).toBe(90_000);
    expect(parseTimecode("0")).toBe(0);
  });

  it("refuses what is not a time", () => {
    for (const bad of ["", "abc", "1:2:3:4", "1:60", "0:00:75", "-1", "1.5", "1::2"]) {
      expect(parseTimecode(bad), bad).toBeNull();
    }
  });

  it("round-trips what it formats", () => {
    expect(parseTimecode(formatTimecode(5_430_000))).toBe(5_430_000);
  });
});

describe("checkUnit", () => {
  const draft = { title: " Intro ", description: "*hi*", from: "0:00:10", to: "0:01:00" };

  it("trims the title and converts the range to milliseconds", () => {
    expect(checkUnit(draft)).toEqual({
      ok: true,
      unit: { title: "Intro", description: "*hi*", fromMs: 10_000, toMs: 60_000 },
    });
  });

  it("wants a title", () => {
    expect(checkUnit({ ...draft, title: "  " })).toMatchObject({ ok: false, problem: "Give the unit a title." });
  });

  it("wants readable times", () => {
    expect(checkUnit({ ...draft, to: "soon" })).toMatchObject({ ok: false });
  });

  it("wants the unit to end after it starts", () => {
    expect(checkUnit({ ...draft, to: draft.from })).toMatchObject({ ok: false, problem: expect.stringContaining("after") });
    expect(checkUnit({ ...draft, from: "0:02:00" })).toMatchObject({ ok: false });
  });

  it("holds the range to a known length only", () => {
    expect(checkUnit(draft, 30)).toMatchObject({ ok: false, problem: expect.stringContaining("00:00:30") });
    expect(checkUnit(draft, 60)).toMatchObject({ ok: true });
    expect(checkUnit(draft, 0)).toMatchObject({ ok: true });
  });
});

describe("renderDescription", () => {
  // Sanitizing is DOMPurify's and needs a real DOM; happy-dom's is too thin for it to
  // run, so the e2e spec checks that a script in a description does not survive.
  it("renders Markdown", () => {
    expect(renderDescription("**bold**")).toContain("<strong>bold</strong>");
  });

  it("renders nothing for a blank description", () => {
    expect(renderDescription("  \n")).toBe("");
  });
});

describe("sortUnits", () => {
  it("orders by start, then by ID, without touching the input", () => {
    const a = { id: 2, title: "a", description: "", fromMs: 5, toMs: 9 };
    const b = { id: 1, title: "b", description: "", fromMs: 5, toMs: 9 };
    const c = { id: 3, title: "c", description: "", fromMs: 1, toMs: 2 };
    const input = [a, b, c];
    expect(sortUnits(input).map((u) => u.id)).toEqual([3, 1, 2]);
    expect(input.map((u) => u.id)).toEqual([2, 1, 3]);
  });
});

describe("the RPCs", () => {
  it("fetches the units with the recording", async () => {
    respondWith({
      lectureName: "Hopfen",
      playlistUrl: "https://vod/x.m3u8",
      durationSeconds: 5400,
      units: [{ id: 3, title: "Intro", description: "d", fromMs: 0, toMs: 60000 }],
    });

    const got = await fetchLectureUnits(5, 7);

    expect(lastCall()[0]).toBe("/api/v2/courses/5/streams/7/units");
    expect(got).toEqual({
      lectureName: "Hopfen",
      playlistUrl: "https://vod/x.m3u8",
      durationSeconds: 5400,
      units: [{ id: 3, title: "Intro", description: "d", fromMs: 0, toMs: 60000 }],
    });
  });

  it("reads a lecture with nothing in it", async () => {
    respondWith({});

    expect(await fetchLectureUnits(5, 7)).toEqual({ lectureName: "", playlistUrl: "", durationSeconds: 0, units: [] });
  });

  it("creates a unit and answers it with its ID", async () => {
    respondWith({ id: 9, title: "Intro", description: "d", fromMs: 1000, toMs: 2000 });

    const got = await createLectureUnit(5, 7, { title: "Intro", description: "d", fromMs: 1000, toMs: 2000 });

    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/5/streams/7/units");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toMatchObject({ title: "Intro", fromMs: 1000, toMs: 2000 });
    expect(got.id).toBe(9);
  });

  it("deletes by unit ID", async () => {
    respondWith(null, 204);

    await deleteLectureUnit(5, 7, 9);

    const [url, init] = lastCall();
    expect(url).toBe("/api/v2/courses/5/streams/7/units/9");
    expect(init.method).toBe("DELETE");
  });
});

describe("unitsErrorMessage", () => {
  it("names a missing lecture, an expired session and the server's own words", () => {
    expect(unitsErrorMessage(new ApiError(404, "no such stream"))).toMatch(/does not exist/);
    expect(unitsErrorMessage(new ApiError(401, "x"))).toMatch(/session expired/);
    expect(unitsErrorMessage(new ApiError(400, "a unit needs a title"))).toBe("a unit needs a title");
    expect(unitsErrorMessage(new Error("boom"))).toMatch(/Something went wrong/);
  });
});
