import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { clearToken } from "./api";
import { adminLinks, clock, endStream, reactionPercentages, switchCameraPreset, timeLeft } from "./watch-admin";

let fetchMock: ReturnType<typeof vi.fn>;
beforeEach(() => {
  clearToken();
  fetchMock = vi.fn((url: string) =>
    Promise.resolve(
      String(url).endsWith("/auth/token")
        ? new Response(JSON.stringify({ access_token: "t", expires_in: 900 }))
        : new Response("{}"),
    ),
  );
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const calls = () => fetchMock.mock.calls.filter(([u]) => !String(u).endsWith("/auth/token"));

describe("actions", () => {
  it("switches a preset on the course's stream", async () => {
    await switchCameraPreset(3, 7, 2);
    const [url, init] = calls()[0]!;
    expect(url).toBe("/api/v2/courses/3/streams/7/presets/2/switch");
    expect(init.method).toBe("POST");
  });

  it("ends a stream, keeping or discarding", async () => {
    await endStream(3, 7, true);
    const [url, init] = calls()[0]!;
    expect(url).toBe("/api/v2/courses/3/streams/7/end");
    expect(JSON.parse(init.body)).toMatchObject({ discard: true });
  });
});

describe("adminLinks", () => {
  it("points at the SPA pages", () => {
    expect(adminLinks(3, 7, "ab c")).toEqual({
      edit: "/admin/courses/3/lectures",
      stats: "/admin/courses/3/lectures/7/stats",
      live: "/admin/courses/3/lectures/7/live",
      watch: "/w/ab%20c/7",
    });
  });
});

describe("reactionPercentages", () => {
  const stats = { shares: [{ reaction: "👍", count: 1, share: 0.125 }, { reaction: "👏", count: 7, share: 0.875 }] };
  it("renders percentages in the allowed order, 0% for unused", () => {
    const r = reactionPercentages(stats as never, ["👏", "👍", "😮"]);
    expect(r.map((x) => [x.reaction, x.label])).toEqual([["👏", "87.5%"], ["👍", "12.5%"], ["😮", "0%"]]);
  });
  it("falls back to the event's emoji and copes with no event", () => {
    expect(reactionPercentages(stats as never)).toHaveLength(2);
    expect(reactionPercentages(undefined)).toEqual([]);
  });
});

describe("time", () => {
  it("formats the time left", () => {
    const now = new Date("2026-01-01T10:00:00");
    expect(timeLeft(new Date("2026-01-01T11:05:09"), now)).toBe("1:05:09");
    expect(timeLeft(new Date("2026-01-01T10:12:03"), now)).toBe("12:03");
    expect(timeLeft(new Date("2026-01-01T09:00:00"), now)).toBe("ended");
    expect(timeLeft(null, now)).toBe("");
  });
  it("formats the clock", () => expect(clock(new Date("2026-01-01T04:05:06"))).toBe("04:05:06"));
});
