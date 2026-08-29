import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { fetchProgress, formatDuration, minutesUntilStart, thumbnailUrl } from "./streams";

let fetchMock: ReturnType<typeof vi.fn>;

function urls(): string[] {
  return fetchMock.mock.calls.map((call) => String(call[0]));
}

beforeEach(() => {
  clearToken();
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => vi.unstubAllGlobals());

function respondWith(body: unknown): void {
  fetchMock
    .mockResolvedValueOnce(new Response(JSON.stringify({ access_token: "t" }), { status: 200 }))
    .mockResolvedValueOnce(new Response(JSON.stringify(body), { status: 200 }));
}

describe("fetchProgress", () => {
  /**
   * The endpoint returns the rows the user actually has — not one per id asked for,
   * and in database order. The old page zipped request and response by index, which
   * silently attached one lecture's progress bar to another as soon as any lecture in
   * the list had never been opened.
   */
  it("keys progress by stream id rather than by position", async () => {
    respondWith({
      progressBatch: [
        { streamId: 30, progress: 0.75, watched: false },
        { streamId: 10, progress: 1, watched: true },
      ],
    });

    const progress = await fetchProgress([10, 20, 30]);

    expect(progress.get(10)).toEqual({ streamId: 10, progress: 1, watched: true });
    expect(progress.get(30)).toEqual({ streamId: 30, progress: 0.75, watched: false });
    // Asked for, never watched, so absent rather than defaulted to something.
    expect(progress.has(20)).toBe(false);
  });

  it("sends every id as its own parameter", async () => {
    respondWith({ progressBatch: [] });

    await fetchProgress([10, 20]);

    expect(urls()[1]).toBe("/api/v2/progress?streamIds=10&streamIds=20");
  });

  it("answers an empty request without asking the server, which rejects it", async () => {
    const progress = await fetchProgress([]);

    expect(progress.size).toBe(0);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("thumbnailUrl", () => {
  it("escapes a slug so it cannot climb out of the path", () => {
    expect(thumbnailUrl("../../etc", 1)).toBe("/api/v2/streams/..%2F..%2Fetc/1/thumbs");
  });
});

describe("formatDuration", () => {
  it.each([
    [0, "00:00:00"],
    [90, "00:01:30"],
    [5445, "01:30:45"],
  ])("renders %d seconds as %s", (seconds, expected) => {
    expect(formatDuration(seconds)).toBe(expected);
  });
});

describe("minutesUntilStart", () => {
  const now = new Date("2026-08-21T12:00:00Z");
  const at = (iso: string) => ({ start: new Date(iso) }) as Parameters<typeof minutesUntilStart>[0];

  it("counts up to the start and goes negative after it", () => {
    expect(minutesUntilStart(at("2026-08-21T12:30:00Z"), now)).toBe(30);
    expect(minutesUntilStart(at("2026-08-21T11:45:00Z"), now)).toBe(-15);
  });
});
