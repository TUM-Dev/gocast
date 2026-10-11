import { afterEach, describe, expect, it, vi } from "vitest";

import { routeFetch } from "@/components/watch/panels/testing";
import {
  MIN_SEARCH_LENGTH,
  SearchUnavailableError,
  currentIndex,
  fetchPlaylist,
  fetchSections,
  formatTime,
  neighbours,
  parseTime,
  searchSubtitles,
  splitTime,
  useBookmarks,
} from "./watch-panels";

afterEach(() => vi.unstubAllGlobals());

describe("time", () => {
  it("formats minutes and hours", () => {
    expect(formatTime(0)).toBe("0:00");
    expect(formatTime(75)).toBe("1:15");
    expect(formatTime(3725.9)).toBe("1:02:05");
    expect(formatTime(-3)).toBe("0:00");
    expect(formatTime(NaN)).toBe("0:00");
  });

  it("parses what it formats, and nothing else", () => {
    expect(parseTime("75")).toBe(75);
    expect(parseTime("1:15")).toBe(75);
    expect(parseTime(" 1:02:05 ")).toBe(3725);
    for (const bad of ["", "a", "1:", "1:2:3:4", "-1", "1.5"]) expect(parseTime(bad)).toBeNull();
  });

  it("splits seconds for the bookmark RPCs", () => {
    expect(splitTime(3725.7)).toEqual({ hours: 1, minutes: 2, seconds: 5 });
  });
});

describe("useBookmarks", () => {
  const stored = [
    { bookmarkId: 2, description: "later", hours: 0, minutes: 10, seconds: 0 },
    { bookmarkId: 1, description: "early", hours: 0, minutes: 0, seconds: 30 },
  ];

  it("loads the stream's bookmarks sorted by time", async () => {
    const { calls } = routeFetch([{ path: "/bookmarks", body: { bookmarks: stored } }]);
    const b = useBookmarks(7);
    await b.load();
    expect(calls[0].url).toBe("/api/v2/bookmarks?stream_id=7");
    expect(b.bookmarks.value.map((x) => [x.id, x.time])).toEqual([
      [1, 30],
      [2, 600],
    ]);
  });

  it("sends add and update as query parameters, which is all the gateway reads", async () => {
    const { calls } = routeFetch([
      { path: "/bookmarks", body: { bookmarks: stored } },
      {
        method: "POST",
        path: "/bookmarks",
        body: { bookmark: { bookmarkId: 3, description: "new", hours: 1, minutes: 0, seconds: 5 } },
      },
      {
        method: "PUT",
        path: "/bookmarks/1",
        body: { bookmark: { bookmarkId: 1, description: "renamed", hours: 0, minutes: 20, seconds: 0 } },
      },
    ]);
    const b = useBookmarks(7);
    await b.load();

    expect(await b.add("new", 3605)).toBe(true);
    const post = calls.find((c) => c.method === "POST")!;
    expect(new URL(post.url, "http://x").searchParams.toString()).toBe(
      "stream_id=7&description=new&hours=1&minutes=0&seconds=5",
    );
    expect(b.bookmarks.value.map((x) => x.id)).toEqual([1, 2, 3]);

    expect(await b.update(1, "renamed", 1200)).toBe(true);
    expect(calls.find((c) => c.method === "PUT")!.url).toBe(
      "/api/v2/bookmarks/1?description=renamed&hours=0&minutes=20&seconds=0",
    );
    expect(b.bookmarks.value.map((x) => x.id)).toEqual([2, 1, 3]);
  });

  it("removes a bookmark once the server has", async () => {
    routeFetch([
      { path: "/bookmarks", body: { bookmarks: stored } },
      { method: "DELETE", path: "/bookmarks/2", status: 204 },
    ]);
    const b = useBookmarks(7);
    await b.load();
    expect(await b.remove(2)).toBe(true);
    expect(b.bookmarks.value.map((x) => x.id)).toEqual([1]);
  });

  it("keeps the list and reports the error when a request fails", async () => {
    routeFetch([
      { path: "/bookmarks", body: { bookmarks: stored } },
      { method: "DELETE", path: "/bookmarks/2", status: 500, body: { message: "boom" } },
    ]);
    const b = useBookmarks(7);
    await b.load();
    expect(await b.remove(2)).toBe(false);
    expect(b.bookmarks.value).toHaveLength(2);
    expect(b.error.value).toBe("boom");
  });
});

describe("sections", () => {
  it("reads them sorted, as seconds", async () => {
    const { calls } = routeFetch([
      {
        path: /\/sections$/,
        body: {
          sections: [
            { description: "B", startMinutes: 5, fileId: 9 },
            { description: "A", fileId: 8 },
          ],
        },
      },
    ]);
    const s = await fetchSections("ws 24", 3);
    expect(calls[0].url).toBe("/api/v2/streams/ws%2024/3/sections");
    expect(s).toEqual([
      { description: "A", time: 0, fileId: 8 },
      { description: "B", time: 300, fileId: 9 },
    ]);
  });

  it("finds the section playing", () => {
    const at = [{ time: 0 }, { time: 60 }, { time: 120 }];
    expect(currentIndex(at, 0)).toBe(0);
    expect(currentIndex(at, 59.9)).toBe(0);
    expect(currentIndex(at, 60)).toBe(1);
    expect(currentIndex(at, 9999)).toBe(2);
    expect(currentIndex([{ time: 10 }], 5)).toBe(-1);
    expect(currentIndex([], 5)).toBe(-1);
  });
});

describe("playlist", () => {
  const now = new Date("2026-10-11T12:00:00Z");
  const entry = (id: number, extra: Record<string, unknown> = {}) => ({
    streamId: id,
    courseSlug: "ws24",
    streamName: `L${id}`,
    start: "2026-10-01T10:00:00Z",
    ...extra,
  });

  it("leaves out live and future lectures, but never the current one", async () => {
    routeFetch([
      {
        path: /\/playlist$/,
        body: {
          entries: [
            entry(1),
            entry(2, { liveNow: true }),
            entry(3, { start: "2026-12-01T10:00:00Z" }),
            entry(4, { liveNow: true }),
            entry(5, { streamProgress: { progress: 1.5, watched: true } }),
          ],
        },
      },
    ]);
    const list = await fetchPlaylist("ws24", 4, now);
    expect(list.map((e) => e.streamId)).toEqual([1, 4, 5]);
    expect(list[2]).toMatchObject({ progress: 1, watched: true, slug: "ws24" });
  });

  it("finds the neighbours", () => {
    const e = [1, 2, 3].map((id) => ({ streamId: id, slug: "s", name: "", progress: 0, watched: false }));
    expect(neighbours(e, 2)).toMatchObject({ prev: { streamId: 1 }, next: { streamId: 3 } });
    expect(neighbours(e, 1).prev).toBeNull();
    expect(neighbours(e, 3).next).toBeNull();
    expect(neighbours(e, 99)).toEqual({ prev: null, next: null });
  });
});

describe("searchSubtitles", () => {
  it("asks with the query and reads milliseconds as seconds", async () => {
    const { calls } = routeFetch([
      {
        path: "/streams/5/subtitles/search",
        body: { hits: [{ timestamp: "65500", textPrev: "a", text: "b", textNext: "c" }] },
      },
    ]);
    const hits = await searchSubtitles(5, "hop fen", 10);
    expect(calls[0].url).toBe("/api/v2/streams/5/subtitles/search?query=hop+fen&limit=10");
    expect(hits).toEqual([{ time: 65.5, before: "a", text: "b", after: "c" }]);
  });

  it("reports an unavailable search as its own error", async () => {
    routeFetch([{ path: "/streams/5/subtitles/search", status: 503, body: { message: "off" } }]);
    await expect(searchSubtitles(5, "abc")).rejects.toBeInstanceOf(SearchUnavailableError);
  });

  it("passes other failures through", async () => {
    routeFetch([{ path: "/streams/5/subtitles/search", status: 400, body: { message: "too short" } }]);
    await expect(searchSubtitles(5, "ab")).rejects.toThrow("too short");
    expect(MIN_SEARCH_LENGTH).toBe(3);
  });
});
